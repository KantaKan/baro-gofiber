package attendance

import (
	"context"
	"log"
	"time"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// TrackStats wraps an attendance repository so every write that can change a learner's
// statuses recomputes that learner's stats. Reads pass straight through.
func TrackStats(inner domain.AttendanceRepository, keeper *StatsKeeper) domain.AttendanceRepository {
	return &statsTrackingRepo{AttendanceRepository: inner, keeper: keeper}
}

type statsTrackingRepo struct {
	domain.AttendanceRepository
	keeper *StatsKeeper
}

// refresh never fails the write; a missed recompute is fixed by the next rebuild.
func (r *statsTrackingRepo) refresh(userID primitive.ObjectID, cohort int) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := r.keeper.Recompute(ctx, userID, cohort); err != nil {
		log.Printf("learner stats: recompute %s: %v", userID.Hex(), err)
	}
}

func (r *statsTrackingRepo) refreshByID(ctx interface{}, id primitive.ObjectID) {
	if x, err := r.AttendanceRepository.FindByID(ctx, id); err == nil && x != nil {
		r.refresh(x.UserID, x.CohortNumber)
	}
}

func (r *statsTrackingRepo) InsertRecord(ctx interface{}, record *domain.AttendanceRecord) error {
	err := r.AttendanceRepository.InsertRecord(ctx, record)
	if err == nil {
		r.refresh(record.UserID, record.CohortNumber)
	}
	return err
}

func (r *statsTrackingRepo) InsertIfAbsent(ctx interface{}, record *domain.AttendanceRecord) (bool, error) {
	ok, err := r.AttendanceRepository.InsertIfAbsent(ctx, record)
	if err == nil && ok {
		r.refresh(record.UserID, record.CohortNumber)
	}
	return ok, err
}

func (r *statsTrackingRepo) UpsertRecord(ctx interface{}, filter domain.AttendanceRecordFilter, update interface{}) (*domain.AttendanceRecord, error) {
	x, err := r.AttendanceRepository.UpsertRecord(ctx, filter, update)
	if err == nil && x != nil {
		r.refresh(x.UserID, x.CohortNumber)
	}
	return x, err
}

func (r *statsTrackingRepo) UpdateRecord(ctx interface{}, id primitive.ObjectID, update interface{}) error {
	err := r.AttendanceRepository.UpdateRecord(ctx, id, update)
	if err == nil {
		r.refreshByID(ctx, id)
	}
	return err
}

func (r *statsTrackingRepo) DeleteRecord(ctx interface{}, id primitive.ObjectID, deletedBy string) error {
	err := r.AttendanceRepository.DeleteRecord(ctx, id, deletedBy)
	if err == nil {
		r.refreshByID(ctx, id)
	}
	return err
}

func (r *statsTrackingRepo) UpdateRecords(ctx interface{}, filter domain.AttendanceRecordFilter, update interface{}) error {
	var learners []domain.AttendanceRecord
	if touchesStats(update) {
		// Find affected learners BEFORE the write so bulk deletes capture them before they're hidden.
		recs, ferr := r.AttendanceRepository.FindRecords(ctx, filter, options.Find().SetProjection(bson.M{"user_id": 1, "cohort_number": 1}))
		if ferr != nil {
			log.Printf("learner stats: bulk update lookup: %v", ferr)
			// Still perform the write despite lookup failure
		} else {
			learners = recs
		}
	}
	err := r.AttendanceRepository.UpdateRecords(ctx, filter, update)
	if err != nil {
		return err
	}
	// Recompute stats for affected learners (deduplicated by UserID).
	seen := map[primitive.ObjectID]bool{}
	for _, x := range learners {
		if !seen[x.UserID] {
			seen[x.UserID] = true
			r.refresh(x.UserID, x.CohortNumber)
		}
	}
	return nil
}

// touchesStats reports whether a $set document can change statuses; unknown shapes count as yes.
func touchesStats(update interface{}) bool {
	m, ok := update.(bson.M)
	if !ok {
		return true
	}
	_, status := m["status"]
	_, deleted := m["deleted"]
	return status || deleted
}
