package attendance

import (
	"context"
	"testing"
	"time"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type memStats struct {
	docs map[primitive.ObjectID]domain.LearnerStats
}

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
func (m *memStats) FindByCohort(_ context.Context, cohort int, _ bool) ([]domain.LearnerStats, error) {
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

// ActiveCohorts returns the fixed cohorts if set, else every cohort with a record on or after since.
func (k keeperRecords) ActiveCohorts(_ interface{}, since string) ([]int, error) {
	if k.cohorts != nil {
		return k.cohorts, nil
	}
	seen := map[int]bool{}
	var out []int
	for _, r := range k.list {
		if r.Date >= since && !seen[r.CohortNumber] {
			seen[r.CohortNumber] = true
			out = append(out, r.CohortNumber)
		}
	}
	return out, nil
}

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

func TestRecomputeCountsOnlyTheGivenCohort(t *testing.T) {
	u := primitive.NewObjectID()
	old := rec(u, "2026-01-05", domain.SessionMorning, domain.StatusLate)
	old.CohortNumber = 12
	recs := keeperRecords{list: []domain.AttendanceRecord{old, rec(u, "2026-09-01", domain.SessionMorning, domain.StatusLate)}}
	store := &memStats{docs: map[primitive.ObjectID]domain.LearnerStats{}}
	if err := NewStatsKeeper(recs, store).Recompute(context.Background(), u, 13); err != nil {
		t.Fatal(err)
	}
	if got := store.docs[u].LeaveUsed; got != 1 {
		t.Fatalf("LeaveUsed = %d, want 1 (cohort 12 record counted)", got)
	}
}

func TestDueForNightlyRebuildAfterDaytimeSeed(t *testing.T) {
	k := NewStatsKeeper(keeperRecords{}, &memStats{docs: map[primitive.ObjectID]domain.LearnerStats{}})
	if !k.DueForNightlyRebuild(bkkAt("2026-09-08", 10, 0)) {
		t.Error("first call at 10:00 did not seed")
	}
	if k.DueForNightlyRebuild(bkkAt("2026-09-08", 10, 1)) {
		t.Error("due again after a daytime seed")
	}
	if !k.DueForNightlyRebuild(bkkAt("2026-09-09", 2, 30)) {
		t.Error("not due the night after a daytime seed")
	}
}

// hookRecords runs onCohort once, after a whole-cohort read has taken its snapshot.
type hookRecords struct {
	*keeperRecords
	onCohort func()
}

func (h *hookRecords) FindRecords(ctx interface{}, f domain.AttendanceRecordFilter, o interface{}) ([]domain.AttendanceRecord, error) {
	out, err := h.keeperRecords.FindRecords(ctx, f, o)
	if f.UserID.IsZero() && h.onCohort != nil {
		run := h.onCohort
		h.onCohort = nil
		run()
	}
	return out, err
}

func TestRebuildCohortKeepsNewerRecompute(t *testing.T) {
	a, b := primitive.NewObjectID(), primitive.NewObjectID()
	base := &keeperRecords{list: []domain.AttendanceRecord{rec(a, "2026-09-01", domain.SessionMorning, domain.StatusPresent), rec(b, "2026-09-01", domain.SessionMorning, domain.StatusPresent)}}
	recs := &hookRecords{keeperRecords: base}
	store := &memStats{docs: map[primitive.ObjectID]domain.LearnerStats{}}
	k := NewStatsKeeper(recs, store)
	recs.onCohort = func() {
		base.list = append(base.list, rec(a, "2026-09-02", domain.SessionMorning, domain.StatusLate))
		if err := k.Recompute(context.Background(), a, 13); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := k.RebuildCohort(context.Background(), 13); err != nil {
		t.Fatal(err)
	}
	if got := store.docs[a].LeaveUsed; got != 1 {
		t.Fatalf("A LeaveUsed = %d, want 1: rebuild overwrote the newer recompute", got)
	}
	if _, ok := store.docs[b]; !ok {
		t.Fatal("rebuild skipped B")
	}
}

func TestRebuildCleanupKeepsOtherCohortsTouched(t *testing.T) {
	a := primitive.NewObjectID()
	x := primitive.NewObjectID()
	aRec := rec(a, "2026-09-01", domain.SessionMorning, domain.StatusPresent)
	xRec := rec(x, "2026-09-01", domain.SessionMorning, domain.StatusPresent)
	xRec.CohortNumber = 14

	store := &memStats{docs: map[primitive.ObjectID]domain.LearnerStats{}}
	recs := keeperRecords{list: []domain.AttendanceRecord{aRec, xRec}}
	k := NewStatsKeeper(recs, store)
	if err := k.Recompute(context.Background(), x, 14); err != nil {
		t.Fatal(err)
	}
	time.Sleep(1 * time.Millisecond)
	if _, err := k.RebuildCohort(context.Background(), 13); err != nil {
		t.Fatal(err)
	}
	k.mu.Lock()
	_, exists := k.touched[x]
	k.mu.Unlock()

	if !exists {
		t.Fatal("rebuild cleanup deleted touched[X] from another cohort")
	}
}

func TestRebuildAllCoversCohortsWithOnlyOldRecords(t *testing.T) {
	cur, alum := primitive.NewObjectID(), primitive.NewObjectID()
	now := time.Date(2026, 9, 8, 3, 0, 0, 0, time.UTC)
	old := rec(alum, "2026-01-05", domain.SessionMorning, domain.StatusPresent)
	old.CohortNumber = 12
	recs := keeperRecords{list: []domain.AttendanceRecord{rec(cur, "2026-09-07", domain.SessionMorning, domain.StatusPresent), old}}
	active := &memStats{docs: map[primitive.ObjectID]domain.LearnerStats{}}
	if err := NewStatsKeeper(recs, active).RebuildActive(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	if _, ok := active.docs[alum]; ok || len(active.docs) != 1 {
		t.Fatalf("RebuildActive should skip cohort 12, got %+v", active.docs)
	}
	all := &memStats{docs: map[primitive.ObjectID]domain.LearnerStats{}}
	if err := NewStatsKeeper(recs, all).RebuildAll(context.Background()); err != nil {
		t.Fatal(err)
	}
	if all.docs[alum].CohortNumber != 12 || len(all.docs) != 2 {
		t.Fatalf("RebuildAll should cover cohort 12, got %+v", all.docs)
	}
}

func TestRebuildEndsOnNewestCohort(t *testing.T) {
	u := primitive.NewObjectID()
	old := rec(u, "2026-01-05", domain.SessionMorning, domain.StatusPresent)
	old.CohortNumber = 12
	store := &memStats{docs: map[primitive.ObjectID]domain.LearnerStats{}}
	recs := keeperRecords{list: []domain.AttendanceRecord{rec(u, "2026-09-07", domain.SessionMorning, domain.StatusPresent), old}, cohorts: []int{13, 12}}
	if err := NewStatsKeeper(recs, store).RebuildActive(context.Background(), time.Now()); err != nil {
		t.Fatal(err)
	}
	if got := store.docs[u].CohortNumber; got != 13 {
		t.Fatalf("learner in cohorts 12 and 13 ended on %d, want 13", got)
	}
}
