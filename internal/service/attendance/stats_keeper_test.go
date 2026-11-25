package attendance

import (
	"context"
	"testing"
	"time"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type memStats struct{ docs map[primitive.ObjectID]domain.LearnerStats }

func (m *memStats) Upsert(_ context.Context, s domain.LearnerStats) error {
	m.docs[s.UserID] = s
	return nil
}
func (m *memStats) UpsertMany(_ context.Context, all []domain.LearnerStats) error {
	for _, s := range all {
		m.docs[s.UserID] = s
	}
	return nil
}
func (m *memStats) FindByCohort(_ context.Context, cohort int) ([]domain.LearnerStats, error) {
	var out []domain.LearnerStats
	for _, s := range m.docs {
		if s.CohortNumber == cohort {
			out = append(out, s)
		}
	}
	return out, nil
}

type keeperRecords struct {
	list    []domain.AttendanceRecord
	cohorts []int
}

func (k keeperRecords) FindRecords(_ interface{}, f domain.AttendanceRecordFilter, _ interface{}) ([]domain.AttendanceRecord, error) {
	var out []domain.AttendanceRecord
	for _, r := range k.list {
		if (f.UserID.IsZero() || r.UserID == f.UserID) && (f.Cohort == 0 || r.CohortNumber == f.Cohort) {
			out = append(out, r)
		}
	}
	return out, nil
}
func (k keeperRecords) ActiveCohorts(interface{}, string) ([]int, error) { return k.cohorts, nil }

func TestRecomputeStoresOneLearner(t *testing.T) {
	u, other := primitive.NewObjectID(), primitive.NewObjectID()
	recs := keeperRecords{list: []domain.AttendanceRecord{rec(u, "2026-09-01", domain.SessionMorning, domain.StatusLate), rec(other, "2026-09-01", domain.SessionMorning, domain.StatusLate)}}
	store := &memStats{docs: map[primitive.ObjectID]domain.LearnerStats{}}
	if err := NewStatsKeeper(recs, store).Recompute(context.Background(), u, 13); err != nil {
		t.Fatal(err)
	}
	if len(store.docs) != 1 || store.docs[u].LeaveUsed != 1 || store.docs[u].CohortNumber != 13 || store.docs[u].UpdatedAt.IsZero() {
		t.Fatalf("store = %+v", store.docs)
	}
}

func TestRebuildActiveCoversEveryActiveCohort(t *testing.T) {
	a, b, alum := primitive.NewObjectID(), primitive.NewObjectID(), primitive.NewObjectID()
	list := []domain.AttendanceRecord{rec(a, "2026-09-01", domain.SessionMorning, domain.StatusPresent), rec(b, "2026-09-01", domain.SessionMorning, domain.StatusPresent), rec(alum, "2026-01-01", domain.SessionMorning, domain.StatusPresent)}
	list[2].CohortNumber = 12
	store := &memStats{docs: map[primitive.ObjectID]domain.LearnerStats{}}
	if err := NewStatsKeeper(keeperRecords{list: list, cohorts: []int{13}}, store).RebuildActive(context.Background(), time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, ok := store.docs[alum]; ok || len(store.docs) != 2 {
		t.Fatalf("want only cohort 13 learners, got %+v", store.docs)
	}
}

func TestDueForNightlyRebuild(t *testing.T) {
	k := NewStatsKeeper(keeperRecords{}, &memStats{docs: map[primitive.ObjectID]domain.LearnerStats{}})
	if k.DueForNightlyRebuild(bkkAt("2026-09-08", 1, 59)) {
		t.Error("due before 02:00")
	}
	if !k.DueForNightlyRebuild(bkkAt("2026-09-08", 2, 0)) {
		t.Error("not due at 02:00")
	}
	if k.DueForNightlyRebuild(bkkAt("2026-09-08", 2, 1)) {
		t.Error("due twice on the same day")
	}
	if !k.DueForNightlyRebuild(bkkAt("2026-09-09", 2, 30)) {
		t.Error("not due the next night")
	}
}
