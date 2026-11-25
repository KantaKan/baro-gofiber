package attendance

import (
	"context"
	"errors"
	"testing"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// trackedRepo is an in-memory AttendanceRepository for the decorator tests.
type trackedRepo struct {
	domain.AttendanceRepository
	byID    map[primitive.ObjectID]*domain.AttendanceRecord
	insertOK bool
}

func newTrackedRepo() *trackedRepo {
	return &trackedRepo{byID: map[primitive.ObjectID]*domain.AttendanceRecord{}, insertOK: true}
}
func (r *trackedRepo) live() []domain.AttendanceRecord {
	var out []domain.AttendanceRecord
	for _, x := range r.byID {
		if !x.Deleted {
			out = append(out, *x)
		}
	}
	return out
}
func (r *trackedRepo) FindRecords(_ interface{}, f domain.AttendanceRecordFilter, _ interface{}) ([]domain.AttendanceRecord, error) {
	var out []domain.AttendanceRecord
	for _, x := range r.live() {
		if (f.UserID.IsZero() || x.UserID == f.UserID) && (f.Cohort == 0 || x.CohortNumber == f.Cohort) && (f.Date == "" || x.Date == f.Date) {
			out = append(out, x)
		}
	}
	return out, nil
}
func (r *trackedRepo) ActiveCohorts(interface{}, string) ([]int, error) { return []int{13}, nil }
func (r *trackedRepo) FindByID(_ interface{}, id primitive.ObjectID) (*domain.AttendanceRecord, error) {
	return r.byID[id], nil
}
func (r *trackedRepo) InsertRecord(_ interface{}, x *domain.AttendanceRecord) error {
	x.ID = primitive.NewObjectID()
	c := *x
	r.byID[x.ID] = &c
	return nil
}
func (r *trackedRepo) InsertIfAbsent(ctx interface{}, x *domain.AttendanceRecord) (bool, error) {
	if !r.insertOK {
		return false, nil
	}
	return true, r.InsertRecord(ctx, x)
}
func (r *trackedRepo) UpdateRecord(_ interface{}, id primitive.ObjectID, update interface{}) error {
	if st, ok := update.(bson.M)["status"]; ok {
		r.byID[id].Status = st.(domain.AttendanceStatus)
	}
	return nil
}
func (r *trackedRepo) UpdateRecords(interface{}, domain.AttendanceRecordFilter, interface{}) error { return nil }
func (r *trackedRepo) DeleteRecord(_ interface{}, id primitive.ObjectID, _ string) error {
	r.byID[id].Deleted = true
	return nil
}

type failingStats struct{ memStats }

func (failingStats) Upsert(context.Context, domain.LearnerStats) error { return errors.New("timeout") }

func TestTrackStatsFollowsEveryWrite(t *testing.T) {
	inner := newTrackedRepo()
	store := &memStats{docs: map[primitive.ObjectID]domain.LearnerStats{}}
	repo := TrackStats(inner, NewStatsKeeper(inner, store))
	ctx := context.Background()
	u := primitive.NewObjectID()

	x := rec(u, "2026-09-08", domain.SessionMorning, domain.StatusLate)
	if err := repo.InsertRecord(ctx, &x); err != nil {
		t.Fatal(err)
	}
	if store.docs[u].LeaveUsed != 1 {
		t.Fatalf("after insert: %+v", store.docs[u])
	}
	if err := repo.UpdateRecord(ctx, x.ID, bson.M{"status": domain.StatusPresent}); err != nil {
		t.Fatal(err)
	}
	if store.docs[u].LeaveUsed != 0 || store.docs[u].Streak != 1 {
		t.Fatalf("after admin mark present: %+v", store.docs[u])
	}
	if err := repo.DeleteRecord(ctx, x.ID, "admin"); err != nil {
		t.Fatal(err)
	}
	if s := store.docs[u]; s.Streak != 0 || s.LeaveUsed != 0 || len(s.Last10) != 0 {
		t.Fatalf("after deleting the only record: %+v", s)
	}
}

func TestTrackStatsSkipsNoOps(t *testing.T) {
	inner := newTrackedRepo()
	inner.insertOK = false
	store := &memStats{docs: map[primitive.ObjectID]domain.LearnerStats{}}
	repo := TrackStats(inner, NewStatsKeeper(inner, store))
	x := rec(primitive.NewObjectID(), "2026-09-08", domain.SessionMorning, domain.StatusLate)
	if ok, _ := repo.InsertIfAbsent(context.Background(), &x); ok || len(store.docs) != 0 {
		t.Fatalf("InsertIfAbsent that did not insert recomputed: %+v", store.docs)
	}
	if err := repo.UpdateRecords(context.Background(), domain.AttendanceRecordFilter{Cohort: 13}, bson.M{"locked": true}); err != nil || len(store.docs) != 0 {
		t.Fatalf("lock-only UpdateRecords recomputed: %v %+v", err, store.docs)
	}
}

func TestTrackStatsWriteSucceedsWhenRecomputeFails(t *testing.T) {
	inner := newTrackedRepo()
	repo := TrackStats(inner, NewStatsKeeper(inner, &failingStats{}))
	x := rec(primitive.NewObjectID(), "2026-09-08", domain.SessionMorning, domain.StatusPresent)
	if err := repo.InsertRecord(context.Background(), &x); err != nil {
		t.Fatalf("check-in failed because stats failed: %v", err)
	}
	if len(inner.live()) != 1 {
		t.Fatal("record not written")
	}
}
