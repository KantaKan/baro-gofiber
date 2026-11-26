package attendance

import (
	"context"
	"errors"
	"fmt"
	"log"
	"slices"
	"sync"
	"time"

	"gofiber-baro/internal/domain"
	"gofiber-baro/pkg/utils"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type StatsRecords interface {
	FindRecords(ctx interface{}, filter domain.AttendanceRecordFilter, opts interface{}) ([]domain.AttendanceRecord, error)
	ActiveCohorts(ctx interface{}, since string) ([]int, error)
}

// statsRecordFields is every record field ComputeStats reads.
func statsRecordFields() *options.FindOptions {
	return options.Find().SetProjection(bson.M{"_id": 0, "user_id": 1, "cohort_number": 1, "date": 1, "session": 1, "status": 1, "marked_by": 1, "submitted_at": 1})
}

// StatsKeeper keeps learner_stats in step with attendance records.
type StatsKeeper struct {
	records StatsRecords
	store   domain.LearnerStatsRepository

	mu          sync.Mutex
	lastNightly string
	touched     map[primitive.ObjectID]time.Time
}

func NewStatsKeeper(records StatsRecords, store domain.LearnerStatsRepository) *StatsKeeper {
	return &StatsKeeper{records: records, store: store, touched: map[primitive.ObjectID]time.Time{}}
}

// Recompute rewrites one learner's stats from their live records.
func (k *StatsKeeper) Recompute(ctx context.Context, userID primitive.ObjectID, cohort int) error {
	recs, err := k.records.FindRecords(ctx, domain.AttendanceRecordFilter{UserID: userID, Cohort: cohort, NotDeleted: true}, statsRecordFields())
	if err != nil {
		return err
	}
	s := ComputeStats(recs)
	s.UserID, s.CohortNumber, s.UpdatedAt = userID, cohort, time.Now()
	if err := k.store.Upsert(ctx, s); err != nil {
		return err
	}
	k.mu.Lock()
	k.touched[userID] = time.Now()
	k.mu.Unlock()
	return nil
}

// RebuildCohort rewrites stats for every learner with records in the cohort, skipping
// learners recomputed after the rebuild began so their newer stats are not overwritten.
func (k *StatsKeeper) RebuildCohort(ctx context.Context, cohort int) (int, error) {
	start := time.Now()
	recs, err := k.records.FindRecords(ctx, domain.AttendanceRecordFilter{Cohort: cohort, NotDeleted: true}, statsRecordFields())
	if err != nil {
		return 0, err
	}
	now := time.Now()
	all := make([]domain.LearnerStats, 0)
	cohortLearners := StatsByUser(recs)
	k.mu.Lock()
	for _, s := range cohortLearners {
		if t, ok := k.touched[s.UserID]; ok && !t.Before(start) {
			continue
		}
		s.CohortNumber, s.UpdatedAt = cohort, now
		all = append(all, s)
	}
	k.mu.Unlock()
	err = k.store.UpsertMany(ctx, all)
	k.mu.Lock()
	for id := range cohortLearners {
		if t, ok := k.touched[id]; ok && t.Before(start) {
			delete(k.touched, id)
		}
	}
	k.mu.Unlock()
	return len(all), err
}

// RebuildActive rebuilds every cohort with attendance in the last 7 days.
func (k *StatsKeeper) RebuildActive(ctx context.Context, now time.Time) error {
	return k.rebuildSince(ctx, utils.FormatDate(now.AddDate(0, 0, -7)))
}

// RebuildAll rebuilds every cohort with live attendance records.
func (k *StatsKeeper) RebuildAll(ctx context.Context) error {
	return k.rebuildSince(ctx, "")
}

func (k *StatsKeeper) rebuildSince(ctx context.Context, since string) error {
	cohorts, err := k.records.ActiveCohorts(ctx, since)
	if err != nil {
		return err
	}
	// Ascending, so a learner with records in two cohorts ends on the newer one.
	slices.Sort(cohorts)
	var errs error
	for _, c := range cohorts {
		n, err := k.RebuildCohort(ctx, c)
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("cohort %d: %w", c, err))
			continue
		}
		log.Printf("learner stats: rebuilt cohort %d (%d learners)", c, n)
	}
	return errs
}

// DueForNightlyRebuild is true once per Bangkok day, at or after 02:00. The first call
// seeds the guard, so calling it at a daytime boot marks that day's rebuild as done.
func (k *StatsKeeper) DueForNightlyRebuild(now time.Time) bool {
	now = now.In(bangkokLoc())
	if now.Hour() < 2 {
		return false
	}
	day := utils.FormatDate(now)
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.lastNightly == day {
		return false
	}
	k.lastNightly = day
	return true
}
