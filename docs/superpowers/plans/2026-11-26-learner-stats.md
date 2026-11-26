# Precomputed Learner Stats Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Admin pages read precomputed per-learner attendance totals and slimmed reflections instead of whole-cohort history, so each page moves tens of KB instead of hundreds.

**Architecture:** A new `learner_stats` collection holds one document per learner, recomputed by a decorator around the attendance repository after every write, plus a startup and nightly rebuild. Admin views read stats and only the records for the dates on screen. Reflections come through a Mongo aggregation that keeps text only for one night. `GET /users` drops heavy fields; the classmate panel fetches one learner on open.

**Tech Stack:** Go 1.x, Fiber v2, mongo-driver v1.17, errgroup; React 18, TypeScript, React Query v3, Vitest.

**Spec:** `docs/superpowers/specs/2026-11-26-learner-stats-design.md` (this repo)

## Global Constraints

- Two separate git repos: `D:\code\baro\baro-gofiber` (backend) and `D:\code\baro\react-genbaro` (frontend). Run git commands from inside the repo, one repo per command, and guard with `[ "$(basename "$(git rev-parse --show-toplevel)")" = <repo> ]`. Never run both repos' git in parallel tool calls.
- Branch from `main`; never commit to `main` directly. Ship each PR with `gh pr create --base main` then `gh pr merge --merge --delete-branch`, only after the user says to merge.
- Commit messages end with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`; PR bodies end with `🤖 Generated with [Claude Code](https://claude.com/claude-code)`.
- Do not run the backend locally against the production `MONGO_URI` (startup mutates data). Verify DB behavior through tests with fakes and through production latency logs after deploy.
- Backend checks before any PR: `go build ./... && go vet ./internal/... ./cmd/... && go test ./internal/...`.
- Frontend checks before any PR: `npm run typecheck && npx vitest run && npm run build` (build catches CSS errors the others miss).
- JSON response shapes of `/admin/today`, `/admin/attendance/register`, `/admin/learners`, `/admin/support`, `/admin/reflections/board` must not change.
- Match surrounding style: short doc comments on exported identifiers, no other comments.
- Rollout order is fixed: PR 1 (backend write-only) → PR 2 (frontend) → PR 3 (backend switch reads).

## Review Focus

1. A learner with no attendance records has no `learner_stats` document: views must show zeros and `last10: []`, never `null` (pinned in Task 8).
2. A reflection stored without `day` (date only) must still appear in mood strips and still be matched as the text night (pinned in Task 9).
3. Admin undo/delete (`UpdateRecord`, `DeleteRecord`) must move the stats back, including when the learner ends up with no live records (pinned in Task 4).
4. Register's previous-week KPI deltas need the previous week's records, not just the selected week (pinned in Task 10).
5. A failing stats recompute (DB timeout) must not fail the learner's check-in or the admin's mark (pinned in Task 4).

---

## PR 1 — Backend: write `learner_stats`, reads unchanged

Branch: `feat/learner-stats-write` in `baro-gofiber`.

```bash
cd /d/code/baro/baro-gofiber && [ "$(basename "$(git rev-parse --show-toplevel)")" = baro-gofiber ] && git checkout main && git pull && git checkout -b feat/learner-stats-write
```

### Task 1: Stats types and `ComputeStats`

**Files:**
- Modify: `internal/domain/attendance.go` (add types at end)
- Modify: `internal/service/attendance/progress.go:43-46` (DayResult becomes an alias)
- Create: `internal/service/attendance/learner_stats.go`
- Modify: `internal/service/wellbeing/signals.go:161-175` (use `attendance.LateRun`)
- Test: `internal/service/attendance/learner_stats_test.go`

**Interfaces:**
- Produces: `domain.DayResult{Date string; Status AttendanceStatus}` (bson `date`,`status`); `attendance.DayResult = domain.DayResult`; `domain.LearnerStats{UserID primitive.ObjectID (_id); CohortNumber int; AttendanceRate float64; LeaveUsed int; Streak int; LateRun int; Last10 []DayResult; LastSeen *time.Time; UpdatedAt time.Time}`; `domain.LearnerStatsRepository{Upsert(ctx context.Context, s LearnerStats) error; UpsertMany(ctx context.Context, all []LearnerStats) error; FindByCohort(ctx context.Context, cohort int) ([]LearnerStats, error)}`; `attendance.ComputeStats(records []domain.AttendanceRecord) domain.LearnerStats` (UserID/Cohort left zero); `attendance.LateRun(days []DayResult) int`; `attendance.StatsByUser(records []domain.AttendanceRecord) map[primitive.ObjectID]domain.LearnerStats` (UserID and CohortNumber filled from the records).

- [ ] **Step 1: Write the failing test**

Create `internal/service/attendance/learner_stats_test.go`:

```go
package attendance

import (
	"testing"
	"time"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func rec(u primitive.ObjectID, date string, s domain.AttendanceSession, st domain.AttendanceStatus) domain.AttendanceRecord {
	return domain.AttendanceRecord{UserID: u, CohortNumber: 13, Date: date, Session: s, Status: st}
}

func TestComputeStatsMatchesViewFunctions(t *testing.T) {
	u := primitive.NewObjectID()
	m, a := domain.SessionMorning, domain.SessionAfternoon
	recs := []domain.AttendanceRecord{
		rec(u, "2026-09-01", m, domain.StatusPresent), rec(u, "2026-09-01", a, domain.StatusPresent),
		rec(u, "2026-09-02", m, domain.StatusAbsentExcused), rec(u, "2026-09-02", a, domain.StatusAbsentExcused),
		rec(u, "2026-09-03", m, domain.StatusHoliday), rec(u, "2026-09-03", a, domain.StatusHoliday),
		rec(u, "2026-09-04", m, domain.StatusLate), rec(u, "2026-09-04", a, domain.StatusPresent),
		rec(u, "2026-09-07", m, domain.StatusLate), rec(u, "2026-09-07", a, domain.StatusLate),
		rec(u, "2026-09-08", m, domain.StatusAbsent),
	}
	seen := time.Date(2026, 9, 7, 1, 50, 0, 0, time.UTC)
	recs[9].MarkedBy, recs[9].SubmittedAt = domain.MarkedBySelf, seen
	days := BuildDays(recs)
	got := ComputeStats(recs)
	if got.AttendanceRate != AttendanceRate(days) || got.LeaveUsed != LeaveDaysUsed(days) || got.Streak != OnTimeStreak(days) || got.LateRun != LateRun(days) {
		t.Fatalf("stats %+v disagree with view functions", got)
	}
	if len(got.Last10) != len(days) || got.Last10[len(got.Last10)-1].Date != "2026-09-08" {
		t.Errorf("last10 = %+v", got.Last10)
	}
	if got.LastSeen == nil || !got.LastSeen.Equal(seen) {
		t.Errorf("last seen = %v, want %v", got.LastSeen, seen)
	}
}

func TestComputeStatsEmpty(t *testing.T) {
	got := ComputeStats(nil)
	if got.AttendanceRate != 0 || got.LeaveUsed != 0 || got.Streak != 0 || got.LastSeen != nil || got.Last10 == nil || len(got.Last10) != 0 {
		t.Fatalf("empty history → %+v, want zeros and an empty (non-nil) last10", got)
	}
}

func TestLateRun(t *testing.T) {
	d := []DayResult{{Date: "1", Status: domain.StatusLate}, {Date: "2", Status: domain.StatusHoliday}, {Date: "3", Status: domain.StatusLate}, {Date: "4", Status: domain.StatusLate}}
	if n := LateRun(d); n != 3 {
		t.Errorf("late run = %d, want 3 (holidays are skipped)", n)
	}
	d = append(d, DayResult{Date: "5", Status: domain.StatusPresent})
	if n := LateRun(d); n != 0 {
		t.Errorf("late run after a present day = %d, want 0", n)
	}
}

func TestStatsByUser(t *testing.T) {
	a, b := primitive.NewObjectID(), primitive.NewObjectID()
	got := StatsByUser([]domain.AttendanceRecord{rec(a, "2026-09-01", domain.SessionMorning, domain.StatusLate), rec(b, "2026-09-01", domain.SessionMorning, domain.StatusPresent)})
	if len(got) != 2 || got[a].UserID != a || got[a].CohortNumber != 13 || got[a].LeaveUsed != 1 || got[b].LeaveUsed != 0 {
		t.Fatalf("by user = %+v", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/service/attendance/ -run 'ComputeStats|LateRun|StatsByUser'`
Expected: FAIL, `undefined: ComputeStats`.

- [ ] **Step 3: Add the domain types**

Append to `internal/domain/attendance.go` (add `"context"` to its imports):

```go
// DayResult is one day's combined attendance status.
type DayResult struct {
	Date   string           `bson:"date" json:"date"`
	Status AttendanceStatus `bson:"status" json:"status"`
}

// LearnerStats is one learner's attendance totals, kept current on every attendance write.
type LearnerStats struct {
	UserID         primitive.ObjectID `bson:"_id" json:"user_id"`
	CohortNumber   int                `bson:"cohort_number" json:"cohort_number"`
	AttendanceRate float64            `bson:"attendance_rate" json:"attendance_rate"`
	LeaveUsed      int                `bson:"leave_used" json:"leave_used"`
	Streak         int                `bson:"streak" json:"streak"`
	LateRun        int                `bson:"late_run" json:"late_run"`
	Last10         []DayResult        `bson:"last10" json:"last10"`
	LastSeen       *time.Time         `bson:"last_seen,omitempty" json:"last_seen,omitempty"`
	UpdatedAt      time.Time          `bson:"updated_at" json:"updated_at"`
}

type LearnerStatsRepository interface {
	Upsert(ctx context.Context, s LearnerStats) error
	UpsertMany(ctx context.Context, all []LearnerStats) error
	FindByCohort(ctx context.Context, cohort int) ([]LearnerStats, error)
}
```

In `internal/service/attendance/progress.go` replace the `DayResult` struct (lines 43-46) with:

```go
type DayResult = domain.DayResult
```

- [ ] **Step 4: Write `learner_stats.go`**

Create `internal/service/attendance/learner_stats.go`:

```go
package attendance

import (
	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// LateRun counts consecutive late days from the newest, skipping holidays and no-class days.
func LateRun(days []DayResult) int {
	run := 0
	for i := len(days) - 1; i >= 0; i-- {
		s := days[i].Status
		if s == domain.StatusHoliday || s == domain.StatusNoClass {
			continue
		}
		if s != domain.StatusLate {
			break
		}
		run++
	}
	return run
}

// ComputeStats derives a learner's totals from their live attendance records.
func ComputeStats(records []domain.AttendanceRecord) domain.LearnerStats {
	days := BuildDays(records)
	s := domain.LearnerStats{
		AttendanceRate: AttendanceRate(days),
		LeaveUsed:      LeaveDaysUsed(days),
		Streak:         OnTimeStreak(days),
		LateRun:        LateRun(days),
		Last10:         last(days, 10),
	}
	for _, r := range records {
		if r.MarkedBy == domain.MarkedBySelf && !r.SubmittedAt.IsZero() && (s.LastSeen == nil || r.SubmittedAt.After(*s.LastSeen)) {
			t := r.SubmittedAt
			s.LastSeen = &t
		}
	}
	return s
}

// StatsByUser computes stats for every learner that appears in records.
func StatsByUser(records []domain.AttendanceRecord) map[primitive.ObjectID]domain.LearnerStats {
	byUser := map[primitive.ObjectID][]domain.AttendanceRecord{}
	cohort := map[primitive.ObjectID]int{}
	for _, r := range records {
		byUser[r.UserID] = append(byUser[r.UserID], r)
		cohort[r.UserID] = r.CohortNumber
	}
	out := make(map[primitive.ObjectID]domain.LearnerStats, len(byUser))
	for id, rs := range byUser {
		s := ComputeStats(rs)
		s.UserID, s.CohortNumber = id, cohort[id]
		out[id] = s
	}
	return out
}
```

`last(days, n)` already exists in `admin_views.go` and returns a non-nil copy.

In `internal/service/wellbeing/signals.go`, inside `SupportReasons`, replace the late-run loop (`run := 0` through the closing `}` of the `for`) with:

```go
	run := attendance.LateRun(days)
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go build ./... && go test ./internal/service/attendance/ ./internal/service/wellbeing/`
Expected: PASS (existing `TestSupportReasons` still passes).

- [ ] **Step 6: Commit**

```bash
git add internal/domain/attendance.go internal/service/attendance/progress.go internal/service/attendance/learner_stats.go internal/service/attendance/learner_stats_test.go internal/service/wellbeing/signals.go
git commit -m "feat: compute per-learner attendance stats from records

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task 2: `learner_stats` repository

**Files:**
- Create: `internal/repository/learner_stats_repo.go`

**Interfaces:**
- Consumes: `domain.LearnerStats`, `domain.LearnerStatsRepository` (Task 1)
- Produces: `repository.NewLearnerStatsRepository(db *mongo.Database) domain.LearnerStatsRepository`

- [ ] **Step 1: Write the repository**

```go
package repository

import (
	"context"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type learnerStatsRepository struct{ col *mongo.Collection }

func NewLearnerStatsRepository(db *mongo.Database) domain.LearnerStatsRepository {
	col := db.Collection("learner_stats")
	_, _ = col.Indexes().CreateOne(context.Background(), mongo.IndexModel{Keys: bson.D{{Key: "cohort_number", Value: 1}}})
	return &learnerStatsRepository{col: col}
}

func (r *learnerStatsRepository) Upsert(ctx context.Context, s domain.LearnerStats) error {
	_, err := r.col.ReplaceOne(ctx, bson.M{"_id": s.UserID}, s, options.Replace().SetUpsert(true))
	return err
}

func (r *learnerStatsRepository) UpsertMany(ctx context.Context, all []domain.LearnerStats) error {
	if len(all) == 0 {
		return nil
	}
	models := make([]mongo.WriteModel, len(all))
	for i, s := range all {
		models[i] = mongo.NewReplaceOneModel().SetFilter(bson.M{"_id": s.UserID}).SetReplacement(s).SetUpsert(true)
	}
	_, err := r.col.BulkWrite(ctx, models, options.BulkWrite().SetOrdered(false))
	return err
}

func (r *learnerStatsRepository) FindByCohort(ctx context.Context, cohort int) ([]domain.LearnerStats, error) {
	cur, err := r.col.Find(ctx, bson.M{"cohort_number": cohort})
	if err != nil {
		return nil, err
	}
	var out []domain.LearnerStats
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}
```

- [ ] **Step 2: Build and vet**

Run: `go build ./... && go vet ./internal/repository/`
Expected: no output. (No unit test: there is no Mongo test harness; behavior is exercised through fakes in Tasks 3–5 and verified in production logs in Task 6.)

- [ ] **Step 3: Commit**

```bash
git add internal/repository/learner_stats_repo.go
git commit -m "feat: learner_stats repository

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task 3: `StatsKeeper` (recompute and rebuild)

**Files:**
- Create: `internal/service/attendance/stats_keeper.go`
- Test: `internal/service/attendance/stats_keeper_test.go`

**Interfaces:**
- Consumes: `ComputeStats`, `StatsByUser` (Task 1); `domain.LearnerStatsRepository` (Task 1)
- Produces: `attendance.NewStatsKeeper(records StatsRecords, store domain.LearnerStatsRepository) *StatsKeeper`; `type StatsRecords interface { FindRecords(ctx interface{}, filter domain.AttendanceRecordFilter, opts interface{}) ([]domain.AttendanceRecord, error); ActiveCohorts(ctx interface{}, since string) ([]int, error) }`; methods `Recompute(ctx context.Context, userID primitive.ObjectID, cohort int) error`, `RebuildCohort(ctx context.Context, cohort int) (int, error)`, `RebuildActive(ctx context.Context, now time.Time) error`, `DueForNightlyRebuild(now time.Time) bool`.

- [ ] **Step 1: Write the failing test**

```go
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
```

(`bkkAt` and `rec` come from `admin_views_test.go` and Task 1's test file in the same package.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/service/attendance/ -run 'Recompute|RebuildActive|Nightly'`
Expected: FAIL, `undefined: NewStatsKeeper`.

- [ ] **Step 3: Implement**

Create `internal/service/attendance/stats_keeper.go`:

```go
package attendance

import (
	"context"
	"errors"
	"log"
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
}

func NewStatsKeeper(records StatsRecords, store domain.LearnerStatsRepository) *StatsKeeper {
	return &StatsKeeper{records: records, store: store}
}

// Recompute rewrites one learner's stats from their live records.
func (k *StatsKeeper) Recompute(ctx context.Context, userID primitive.ObjectID, cohort int) error {
	recs, err := k.records.FindRecords(ctx, domain.AttendanceRecordFilter{UserID: userID, NotDeleted: true}, statsRecordFields())
	if err != nil {
		return err
	}
	s := ComputeStats(recs)
	s.UserID, s.CohortNumber, s.UpdatedAt = userID, cohort, time.Now()
	return k.store.Upsert(ctx, s)
}

// RebuildCohort rewrites stats for every learner with records in the cohort.
func (k *StatsKeeper) RebuildCohort(ctx context.Context, cohort int) (int, error) {
	recs, err := k.records.FindRecords(ctx, domain.AttendanceRecordFilter{Cohort: cohort, NotDeleted: true}, statsRecordFields())
	if err != nil {
		return 0, err
	}
	now := time.Now()
	all := make([]domain.LearnerStats, 0)
	for _, s := range StatsByUser(recs) {
		s.CohortNumber, s.UpdatedAt = cohort, now
		all = append(all, s)
	}
	return len(all), k.store.UpsertMany(ctx, all)
}

// RebuildActive rebuilds every cohort with attendance in the last 7 days.
func (k *StatsKeeper) RebuildActive(ctx context.Context, now time.Time) error {
	cohorts, err := k.records.ActiveCohorts(ctx, utils.FormatDate(now.AddDate(0, 0, -7)))
	if err != nil {
		return err
	}
	var errs error
	for _, c := range cohorts {
		n, err := k.RebuildCohort(ctx, c)
		if err != nil {
			errs = errors.Join(errs, err)
			continue
		}
		log.Printf("learner stats: rebuilt cohort %d (%d learners)", c, n)
	}
	return errs
}

// DueForNightlyRebuild is true once per Bangkok day, at or after 02:00.
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/service/attendance/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/service/attendance/stats_keeper.go internal/service/attendance/stats_keeper_test.go
git commit -m "feat: StatsKeeper recomputes and rebuilds learner stats

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task 4: Repository decorator that keeps stats current

**Files:**
- Create: `internal/service/attendance/stats_tracking_repo.go`
- Test: `internal/service/attendance/stats_tracking_repo_test.go`

**Interfaces:**
- Consumes: `*StatsKeeper` (Task 3)
- Produces: `attendance.TrackStats(inner domain.AttendanceRepository, keeper *StatsKeeper) domain.AttendanceRepository`

- [ ] **Step 1: Write the failing test**

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/service/attendance/ -run TrackStats`
Expected: FAIL, `undefined: TrackStats`.

- [ ] **Step 3: Implement**

Create `internal/service/attendance/stats_tracking_repo.go`:

```go
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
	err := r.AttendanceRepository.UpdateRecords(ctx, filter, update)
	if err != nil || !touchesStats(update) {
		return err
	}
	recs, ferr := r.AttendanceRepository.FindRecords(ctx, filter, options.Find().SetProjection(bson.M{"user_id": 1, "cohort_number": 1}))
	if ferr != nil {
		log.Printf("learner stats: bulk update lookup: %v", ferr)
		return nil
	}
	seen := map[primitive.ObjectID]bool{}
	for _, x := range recs {
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/service/attendance/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/service/attendance/stats_tracking_repo.go internal/service/attendance/stats_tracking_repo_test.go
git commit -m "feat: recompute learner stats after every attendance write

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task 5: Wire it up, rebuild jobs, drift check on Today

**Files:**
- Modify: `cmd/container.go:41-47` (fields), `:143` (repo wiring)
- Modify: `cmd/main.go:136-150` (startup + nightly rebuild)
- Modify: `internal/handler/admin_views_handler.go` (drift check)
- Test: `internal/handler/admin_views_handler_test.go`

**Interfaces:**
- Consumes: `NewLearnerStatsRepository` (Task 2), `NewStatsKeeper`, `TrackStats` (Tasks 3–4), `attendance.TodayView` (existing)
- Produces: container fields `LearnerStatsRepo domain.LearnerStatsRepository`, `StatsKeeper *attendance.StatsKeeper`; `(*AdminViewsHandler).SetStatsDriftCheck(store statsReader)`; handler interface `statsReader { FindByCohort(ctx context.Context, cohort int) ([]domain.LearnerStats, error) }` (reused in Task 10).

- [ ] **Step 1: Write the failing test**

Append to `internal/handler/admin_views_handler_test.go` (add `"bytes"`, `"context"`, `"log"` imports):

```go
type avStats struct{ list []domain.LearnerStats }

func (s avStats) FindByCohort(context.Context, int) ([]domain.LearnerStats, error) { return s.list, nil }

func TestTodayLogsStatsDrift(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)
	u := domain.User{ID: primitive.NewObjectID(), FirstName: "A", Role: "learner"}
	h := NewAdminViewsHandler(avUsers{[]domain.User{u}}, avRecords{}, avSettings{}, avHolidays{})
	h.SetStatsDriftCheck(avStats{[]domain.LearnerStats{{UserID: u.ID, Streak: 9}}})
	h.driftSync = true
	app := fiber.New()
	app.Get("/admin/today", h.Today)
	if res, _ := app.Test(httptest.NewRequest("GET", "/admin/today?cohort=13", nil)); res.StatusCode != 200 {
		t.Fatalf("status %d", res.StatusCode)
	}
	if !strings.Contains(buf.String(), "stats drift: cohort 13, 1 learners differ") {
		t.Errorf("log = %q", buf.String())
	}
}
```

(also add `"os"` to the imports.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/handler/ -run StatsDrift`
Expected: FAIL, `h.SetStatsDriftCheck undefined`.

- [ ] **Step 3: Add the drift check**

In `internal/handler/admin_views_handler.go` add `"math"` to imports, then:

```go
type statsReader interface {
	FindByCohort(ctx context.Context, cohort int) ([]domain.LearnerStats, error)
}

// SetStatsDriftCheck compares Today's computed numbers with learner_stats and logs mismatches.
// It proves learner_stats on real data before the views read it.
func (h *AdminViewsHandler) SetStatsDriftCheck(store statsReader) { h.drift = store }

func (h *AdminViewsHandler) logStatsDrift(cohort int, v attendance.TodayView) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	stored, err := h.drift.FindByCohort(ctx, cohort)
	if err != nil {
		log.Printf("stats drift: cohort %d: %v", cohort, err)
		return
	}
	by := map[string]domain.LearnerStats{}
	for _, s := range stored {
		by[s.UserID.Hex()] = s
	}
	n := 0
	for _, l := range v.Learners {
		s := by[l.ID]
		if s.Streak != l.Streak || s.LeaveUsed != l.LeaveUsed || math.Abs(s.AttendanceRate-l.AttendanceRate) > 1e-9 {
			n++
		}
	}
	if n > 0 {
		log.Printf("stats drift: cohort %d, %d learners differ", cohort, n)
	}
}
```

Add fields to `AdminViewsHandler`: `drift statsReader` and `driftSync bool` (tests run the check inline). In `Today`, replace the final `return` with:

```go
	view := attendance.BuildTodayRoster(users, recs, now, st.Rules, classDay, st.FlagsOn())
	if h.drift != nil {
		if h.driftSync {
			h.logStatsDrift(cohort, view)
		} else {
			go h.logStatsDrift(cohort, view)
		}
	}
	return utils.SendResponse(c, fiber.StatusOK, "Today retrieved", view)
```

- [ ] **Step 4: Wire the container and jobs**

`cmd/container.go`: add fields next to `AttendanceRepo`:

```go
	LearnerStatsRepo   domain.LearnerStatsRepository
	StatsKeeper        *attendance.StatsKeeper
```

Replace line 143 (`c.AttendanceRepo = repository.NewAttendanceRepository(c.DB)`) with:

```go
	rawAttendance := repository.NewAttendanceRepository(c.DB)
	c.LearnerStatsRepo = repository.NewLearnerStatsRepository(c.DB)
	c.StatsKeeper = attendance.NewStatsKeeper(rawAttendance, c.LearnerStatsRepo)
	c.AttendanceRepo = attendance.TrackStats(rawAttendance, c.StatsKeeper)
```

After `c.AdminViewsHandler = handler.NewAdminViewsHandler(...)` (line 226) add:

```go
	c.AdminViewsHandler.SetStatsDriftCheck(c.LearnerStatsRepo)
```

`cmd/main.go`: before `go func() { for range time.Tick(time.Minute) {` add:

```go
	go func() {
		if err := container.StatsKeeper.RebuildActive(context.Background(), utils.GetThailandTime()); err != nil {
			log.Printf("learner stats: startup rebuild: %v", err)
		}
	}()
```

Inside the minute loop, after the cup runner block, add:

```go
			if container.StatsKeeper.DueForNightlyRebuild(now) {
				go func() {
					if err := container.StatsKeeper.RebuildActive(context.Background(), now); err != nil {
						log.Printf("learner stats: nightly rebuild: %v", err)
					}
				}()
			}
```

- [ ] **Step 5: Run the full backend checks**

Run: `go build ./... && go vet ./internal/... ./cmd/... && go test ./internal/...`
Expected: all `ok`.

- [ ] **Step 6: Commit**

```bash
git add cmd/container.go cmd/main.go internal/handler/admin_views_handler.go internal/handler/admin_views_handler_test.go
git commit -m "feat: keep learner_stats current and log drift against Today

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task 6: Ship PR 1 and prove parity

- [ ] **Step 1: Open the PR (after the user says merge)**

```bash
cd /d/code/baro/baro-gofiber && [ "$(basename "$(git rev-parse --show-toplevel)")" = baro-gofiber ] && git push -u origin feat/learner-stats-write && gh pr create --base main --title "feat: precompute learner stats (write path only)" --body "Adds learner_stats, recomputed after every attendance write (repository decorator) and rebuilt at startup and nightly 02:00 Bangkok. Views still compute from history; /admin/today logs \`stats drift\` if stored and computed numbers differ. Spec: docs/superpowers/specs/2026-11-26-learner-stats-design.md

🤖 Generated with [Claude Code](https://claude.com/claude-code)" && gh pr merge --merge --delete-branch && git checkout main && git pull
```

- [ ] **Step 2: After the user redeploys, check the logs**

Expected in Dokploy logs: `learner stats: rebuilt cohort 13 (N learners)` at startup. Over the next day: no `stats drift:` lines while admins use Today. Any `stats drift` line blocks PR 3: investigate which write path the decorator misses before continuing.

---

## PR 2 — Frontend: classmate panel fetches the learner on open

Branch: `feat/classmate-panel-fetch` in `react-genbaro`.

```bash
cd /d/code/baro/react-genbaro && [ "$(basename "$(git rev-parse --show-toplevel)")" = react-genbaro ] && git checkout main && git pull && git checkout -b feat/classmate-panel-fetch
```

### Task 7: `ClassmatePanel` loads recommendations itself

**Files:**
- Modify: `src/learner/classmates/ClassmatePanel.tsx`
- Test: `src/learner/classmates/ClassmatesPage.test.tsx`

**Interfaces:**
- Consumes: `userService.getUserById(id): Promise<UserData>` (existing), `Bone` from `@/components/LoadingSkeleton` (existing)
- Produces: React Query key `["person", id]`

- [ ] **Step 1: Update the test to the new data flow**

In `ClassmatesPage.test.tsx`: remove `profile_comments` from the `u3` entry in `people`, and change the `userService` mock to:

```ts
vi.mock("@/application/services/userService", () => ({
  userService: {
    getAllUsers: vi.fn(),
    addProfileComment: vi.fn().mockResolvedValue(undefined),
    getUserById: vi.fn().mockImplementation(async (id: string) =>
      id === "u3"
        ? { _id: "u3", profile_comments: [{ id: "c1", userId: "u2", zoomName: "22_Mint(Mint/มิ้นท์)", content: "Great at CSS", createdAt: "2026-09-01" }] }
        : { _id: id, profile_comments: [] }
    ),
  },
}));
```

In the test `"never links to non-http social URLs and shows recommenders' avatars"`, change the recommendation lookup to wait for the fetch:

```ts
    const rec = (await within(panel).findByText("Great at CSS")).closest("div")!.parentElement!;
```

Add a test:

```ts
  it("fetches the classmate's recommendations when the panel opens", async () => {
    renderPage();
    fireEvent.click(await screen.findByRole("button", { name: /Open เถียม/ }));
    const panel = await screen.findByRole("dialog");
    expect(await within(panel).findByText("Great at CSS")).toBeInTheDocument();
    expect(within(panel).getByText(/Recommendations · 1/)).toBeInTheDocument();
    expect(userService.getUserById).toHaveBeenCalledWith("u3");
  });
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `npx vitest run src/learner/classmates/ClassmatesPage.test.tsx`
Expected: FAIL, "Great at CSS" not found.

- [ ] **Step 3: Fetch in the panel**

In `ClassmatePanel.tsx`, add imports:

```ts
import { useQuery } from "react-query";
import { Bone } from "@/components/LoadingSkeleton";
```

At the top of the component body, before `if (!person) return null;`:

```ts
  const full = useQuery(["person", person?._id], () => userService.getUserById(person!._id), { enabled: !!person, staleTime: 60_000 });
```

Replace `const recs = (person.profile_comments ?? [])...` with:

```ts
  const comments = full.data?.profile_comments ?? [];
  const recs = comments.filter((c) => !c.parentId).slice(0, 3);
```

Replace the Recommendations heading and empty line:

```tsx
          <h4 className="mb-1 mt-[22px] text-small font-semibold uppercase tracking-[.08em] text-l-ink3">Recommendations{full.data ? ` · ${comments.length}` : ""}</h4>
          {full.isLoading &&
            [0, 1].map((i) => (
              <div key={i} aria-hidden className="flex gap-2.5 border-t border-l-hair py-2.5 first:border-t-0">
                <Bone className="h-[30px] w-[30px] shrink-0 rounded-full" />
                <div className="grid flex-1 gap-1.5">
                  <Bone className="h-4 w-3/4" />
                  <Bone className="h-3 w-20" />
                </div>
              </div>
            ))}
          {full.data && recs.length === 0 && <p className="text-body text-l-ink3">No recommendations yet.</p>}
```

- [ ] **Step 4: Run the frontend checks**

Run: `npm run typecheck && npx vitest run && npm run build`
Expected: typecheck clean, all tests pass, build succeeds.

- [ ] **Step 5: Commit and ship (after the user says merge)**

```bash
cd /d/code/baro/react-genbaro && [ "$(basename "$(git rev-parse --show-toplevel)")" = react-genbaro ] && git add src/learner/classmates/ClassmatePanel.tsx src/learner/classmates/ClassmatesPage.test.tsx && git commit -m "feat: classmate panel fetches recommendations on open

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>" && git push -u origin feat/classmate-panel-fetch && gh pr create --base main --title "feat: classmate panel fetches recommendations on open" --body "Prepares for /users dropping profile_comments from the list: the panel loads GET /users/:id on open and shows a skeleton for recommendations meanwhile.

🤖 Generated with [Claude Code](https://claude.com/claude-code)" && gh pr merge --merge --delete-branch && git checkout main && git pull
```

---

## PR 3 — Backend: views read stats; slim reflections and `/users`

Starts only after PR 1 shows no drift for a day and PR 2 is deployed. Branch: `perf/read-learner-stats` in `baro-gofiber`.

```bash
cd /d/code/baro/baro-gofiber && [ "$(basename "$(git rev-parse --show-toplevel)")" = baro-gofiber ] && git checkout main && git pull && git checkout -b perf/read-learner-stats
```

### Task 8: View builders take stats

**Files:**
- Modify: `internal/service/attendance/admin_views.go` (`BuildTodayRoster`, `BuildRegister`)
- Modify: `internal/service/wellbeing/signals.go` (`SupportReasons`), `internal/service/wellbeing/views.go` (`ReasonsFor`, `BuildSupportList`)
- Modify: `internal/service/report/build.go:225`
- Test: `internal/service/attendance/admin_views_test.go`, `internal/service/wellbeing/signals_test.go`, `internal/service/wellbeing/views_test.go`

**Interfaces:**
- Consumes: `domain.LearnerStats`, `attendance.StatsByUser`, `attendance.ComputeStats` (Task 1)
- Produces:
  - `BuildTodayRoster(learners []domain.User, today []domain.AttendanceRecord, stats map[primitive.ObjectID]domain.LearnerStats, now time.Time, rules domain.CheckinRules, classToday, flagUnusual bool) TodayView`
  - `BuildRegister(learners []domain.User, records []domain.AttendanceRecord, stats map[primitive.ObjectID]domain.LearnerStats, week string, holidays map[string]bool) RegisterView`
  - `SupportReasons(leaveUsed, lateRun int, patterns []Pattern, openItems int) []Reason`
  - `ReasonsFor(u domain.User, st domain.LearnerStats, items []domain.SupportItem, acks []domain.SupportAck, recentClassDays []string, words []string) (reasons []Reason, open, done int)`
  - `BuildSupportList(users []domain.User, stats map[primitive.ObjectID]domain.LearnerStats, items []domain.SupportItem, acks []domain.SupportAck, recentClassDays []string, words []string) []SupportRow`

- [ ] **Step 1: Update tests to the new signatures and add the zero-stats test**

In every existing call in `admin_views_test.go`: `BuildTodayRoster(users, recs, now, ...)` → `BuildTodayRoster(users, recs, StatsByUser(recs), now, ...)`; `BuildRegister(users, recs, week, hol)` → `BuildRegister(users, recs, StatsByUser(recs), week, hol)`. In `signals_test.go`: `SupportReasons(6, nil, nil, 0)` → `SupportReasons(6, 0, nil, 0)`; `SupportReasons(0, late, nil, 0)` → `SupportReasons(0, LateRun(late), nil, 0)` (import `attendance` and use `attendance.LateRun`); `SupportReasons(0, nil, []Pattern{...}, 2)` → `SupportReasons(0, 0, []Pattern{...}, 2)`. In `views_test.go`: `ReasonsFor(u, recs, ...)` → `ReasonsFor(u, attendance.ComputeStats(recs), ...)`; `BuildSupportList(users, recs, ...)` → `BuildSupportList(users, attendance.StatsByUser(recs), ...)`.

Add to `admin_views_test.go`:

```go
func TestBuildTodayRosterLearnerWithoutStats(t *testing.T) {
	a := learner("A", "F", "01")
	v := BuildTodayRoster([]domain.User{a}, nil, map[primitive.ObjectID]domain.LearnerStats{}, bkkAt("2026-09-08", 9, 0), domain.DefaultRules(), true, false)
	l := v.Learners[0]
	if l.Streak != 0 || l.LeaveUsed != 0 || l.AttendanceRate != 0 || l.Last10 == nil {
		t.Fatalf("learner with no stats doc → %+v, want zeros and empty last10", l)
	}
	b, _ := json.Marshal(l)
	if !strings.Contains(string(b), `"last10":[]`) {
		t.Errorf("last10 must serialize as [] not null: %s", b)
	}
}
```

(add `"encoding/json"` and `"strings"` imports.)

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/service/...`
Expected: FAIL (build errors: wrong argument count).

- [ ] **Step 3: Change `BuildTodayRoster`**

Replace its signature and the per-learner stats block:

```go
// BuildTodayRoster assembles the admin Today board for one cohort from today's records and stored stats.
func BuildTodayRoster(learners []domain.User, records []domain.AttendanceRecord, stats map[primitive.ObjectID]domain.LearnerStats, now time.Time, rules domain.CheckinRules, classToday, flagUnusual bool) TodayView {
```

Replace

```go
		days := BuildDays(recs)
		l.Streak = OnTimeStreak(days)
		l.LeaveUsed = LeaveDaysUsed(days)
		l.WarningLevel = WarningLevel(l.LeaveUsed)
		l.AttendanceRate = AttendanceRate(days)
		l.Last10 = last(days, 10)
```

with

```go
		st := stats[u.ID]
		l.Streak = st.Streak
		l.LeaveUsed = st.LeaveUsed
		l.WarningLevel = WarningLevel(st.LeaveUsed)
		l.AttendanceRate = st.AttendanceRate
		l.Last10 = append([]DayResult{}, st.Last10...)
```

- [ ] **Step 4: Change `BuildRegister`**

Signature:

```go
func BuildRegister(learners []domain.User, records []domain.AttendanceRecord, stats map[primitive.ObjectID]domain.LearnerStats, week string, holidays map[string]bool) RegisterView {
```

Replace `l.LeaveUsed = LeaveDaysUsed(BuildDays(byUser[u.ID]))` with `l.LeaveUsed = stats[u.ID].LeaveUsed`.

In `internal/service/report/build.go`, at the top of `Build` add `stats := attendance.StatsByUser(in.Records)` and change line 225 to `v := attendance.BuildRegister(in.Users, in.Records, stats, m, in.Holidays)`.

- [ ] **Step 5: Change the wellbeing functions**

`signals.go`:

```go
func SupportReasons(leaveUsed, lateRun int, patterns []Pattern, openItems int) []Reason {
	var out []Reason
	if leaveUsed >= 6 {
		out = append(out, Reason{"leave", fmt.Sprintf("Leave %d/8", leaveUsed)})
	}
	if lateRun >= 3 {
		out = append(out, Reason{"late_run", fmt.Sprintf("Late %d days running", lateRun)})
	}
```

(keep the rest of the function as is).

`views.go`: `ReasonsFor(u domain.User, st domain.LearnerStats, items ..., acks ..., recentClassDays []string, words []string)` and replace

```go
	days := attendance.BuildDays(recs)
	all := SupportReasons(attendance.LeaveDaysUsed(days), days, WorthALook(Nights(u.Reflections), recentClassDays, words), open)
```

with

```go
	all := SupportReasons(st.LeaveUsed, st.LateRun, WorthALook(Nights(u.Reflections), recentClassDays, words), open)
```

`BuildSupportList(users []domain.User, stats map[primitive.ObjectID]domain.LearnerStats, items ..., acks ..., recentClassDays []string, words []string)`: delete the `byUser` grouping and call `ReasonsFor(u, stats[u.ID], items, acks, recentClassDays, words)`.

- [ ] **Step 6: Run the service tests**

Run: `go test ./internal/service/...`
Expected: PASS. (`./internal/handler/` still fails to build until Tasks 10–11; that is expected inside this task.)

- [ ] **Step 7: Commit**

```bash
git add internal/service/attendance/admin_views.go internal/service/attendance/admin_views_test.go internal/service/wellbeing internal/service/report/build.go
git commit -m "refactor: view builders take precomputed learner stats

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task 9: Reflections aggregation and projected user-by-id

**Files:**
- Modify: `internal/domain/user.go:304-` (`UserRepository` gains two methods)
- Modify: `internal/repository/user_repo.go` (implement)
- Modify: `internal/service/user/user_service.go` (wrappers)
- Test: `internal/repository/user_reflections_pipeline_test.go`

**Interfaces:**
- Produces:
  - `domain.UserRepository.FindCohortLearnersWithReflections(ctx interface{}, cohort int, fields bson.M, n int, textDay string) ([]User, error)`
  - `domain.UserRepository.FindByIDWithFields(ctx interface{}, id primitive.ObjectID, projection bson.M) (*User, error)`
  - `repository.reflectionsView(n int, textDay string) bson.M` (unexported, tested)
  - `(*user.Service).ListCohortLearnersWithReflections(cohort int, fields bson.M, n int, textDay string) ([]domain.User, error)`
  - `(*user.Service).GetUserByIDWithFields(id string, projection bson.M) (*domain.User, error)`

Test fakes in `user_handler_care_energy_test.go`, `care_energy_test.go`, `plant_override_test.go` embed `domain.UserRepository`, so they compile unchanged.

- [ ] **Step 1: Write the failing test**

```go
package repository

import (
	"testing"

	"go.mongodb.org/mongo-driver/bson"
)

func TestReflectionsViewKeepsTextForOneNight(t *testing.T) {
	v := reflectionsView(20, "2026-09-08")
	m := v["$map"].(bson.M)
	in := m["input"].(bson.M)["$slice"].(bson.A)
	if in[1] != -20 {
		t.Errorf("slice = %v, want last 20", in[1])
	}
	cond := m["in"].(bson.M)["$cond"].(bson.A)
	eq := cond[0].(bson.M)["$eq"].(bson.A)
	if eq[1] != "2026-09-08" || cond[1] != "$$r" {
		t.Errorf("text night match = %v → %v", eq, cond[1])
	}
	slim := cond[2].(bson.M)
	if _, ok := slim["reflection"].(bson.M)["barometer"]; !ok || len(slim) != 4 {
		t.Errorf("other nights keep %v, want _id/day/date/barometer only", slim)
	}
	day := eq[0].(bson.M)["$cond"].(bson.A)
	if fmt := day[2].(bson.M)["$dateToString"].(bson.M)["format"]; fmt != "%Y-%m-%d" {
		t.Errorf("reflections without day must match on their date, got %v", day[2])
	}
}

func TestReflectionsViewWithoutTextNight(t *testing.T) {
	cond := reflectionsView(5, "")["$map"].(bson.M)["in"].(bson.M)["$cond"].(bson.A)
	if cond[0].(bson.M)["$eq"].(bson.A)[1] == "" {
		t.Error("empty textDay would keep text for undated reflections")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/repository/ -run ReflectionsView`
Expected: FAIL, `undefined: reflectionsView`.

- [ ] **Step 3: Implement**

`internal/domain/user.go`, add to `UserRepository` (domain already imports bson):

```go
	FindCohortLearnersWithReflections(ctx interface{}, cohort int, fields bson.M, n int, textDay string) ([]User, error)
	FindByIDWithFields(ctx interface{}, id primitive.ObjectID, projection bson.M) (*User, error)
```

`internal/repository/user_repo.go`:

```go
// reflectionsView keeps each learner's last n reflections; only those written for textDay keep
// their text, the rest keep id, day, date and barometer.
func reflectionsView(n int, textDay string) bson.M {
	if textDay == "" {
		textDay = "-"
	}
	day := bson.M{"$cond": bson.A{
		bson.M{"$gt": bson.A{bson.M{"$ifNull": bson.A{"$$r.day", ""}}, ""}},
		"$$r.day",
		bson.M{"$dateToString": bson.M{"format": "%Y-%m-%d", "date": "$$r.date", "onNull": ""}},
	}}
	return bson.M{"$map": bson.M{
		"input": bson.M{"$slice": bson.A{bson.M{"$ifNull": bson.A{"$reflections", bson.A{}}}, -n}},
		"as":    "r",
		"in": bson.M{"$cond": bson.A{
			bson.M{"$eq": bson.A{day, textDay}},
			"$$r",
			bson.M{"_id": "$$r._id", "day": "$$r.day", "date": "$$r.date", "reflection": bson.M{"barometer": "$$r.reflection.barometer"}},
		}},
	}}
}

func (r *userRepository) FindCohortLearnersWithReflections(ctx interface{}, cohort int, fields bson.M, n int, textDay string) ([]domain.User, error) {
	c := ctx.(context.Context)
	project := bson.M{"reflections": reflectionsView(n, textDay)}
	for k, v := range fields {
		project[k] = v
	}
	cur, err := r.collection.Aggregate(c, mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"cohort_number": cohort, "role": "learner"}}},
		{{Key: "$sort", Value: bson.D{{Key: "jsd_number", Value: 1}}}},
		{{Key: "$limit", Value: 1000}},
		{{Key: "$project", Value: project}},
	})
	if err != nil {
		return nil, err
	}
	var out []domain.User
	if err := cur.All(c, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *userRepository) FindByIDWithFields(ctx interface{}, id primitive.ObjectID, projection bson.M) (*domain.User, error) {
	c := ctx.(context.Context)
	var user domain.User
	err := r.collection.FindOne(c, bson.M{"_id": id}, options.FindOne().SetProjection(projection)).Decode(&user)
	if err == mongo.ErrNoDocuments {
		return nil, domain.ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}
```

`internal/service/user/user_service.go`:

```go
// ListCohortLearnersWithReflections is ListCohortLearners with each learner's last n reflections,
// text kept only for textDay.
func (s *Service) ListCohortLearnersWithReflections(cohort int, fields bson.M, n int, textDay string) ([]domain.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return s.repo.FindCohortLearnersWithReflections(ctx, cohort, fields, n, textDay)
}

// GetUserByIDWithFields loads one user with only the projected fields.
func (s *Service) GetUserByIDWithFields(id string, projection bson.M) (*domain.User, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, domain.ErrUserNotFound
	}
	return s.repo.FindByIDWithFields(context.Background(), oid, projection)
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/repository/ ./internal/service/user/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/domain/user.go internal/repository/user_repo.go internal/repository/user_reflections_pipeline_test.go internal/service/user/user_service.go
git commit -m "feat: load learners with slim recent reflections via aggregation

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task 10: Admin Today and Register read stats

**Files:**
- Modify: `internal/handler/admin_views_handler.go`
- Modify: `cmd/container.go:226-227`
- Test: `internal/handler/admin_views_handler_test.go`

**Interfaces:**
- Consumes: `statsReader` (Task 5), `ListCohortLearnersWithReflections` (Task 9), new builder signatures (Task 8)
- Produces: `NewAdminViewsHandler(users adminViewUsers, records adminViewRecords, stats statsReader, settings attendance.CohortSettingsGetter, holidays attendance.HolidayChecker) *AdminViewsHandler`; `adminViewUsers` gains `ListCohortLearnersWithReflections(cohort int, fields bson.M, n int, textDay string) ([]domain.User, error)`; `statsMap(list []domain.LearnerStats) map[primitive.ObjectID]domain.LearnerStats` (used in Task 11)

- [ ] **Step 1: Update tests**

In `admin_views_handler_test.go`:

```go
func (u avUsers) ListCohortLearnersWithReflections(int, bson.M, int, string) ([]domain.User, error) {
	return u.list, nil
}

// avRangeRecords records the filters a handler asked for.
type avRangeRecords struct{ filters *[]domain.AttendanceRecordFilter }

func (r avRangeRecords) FindRecords(_ interface{}, f domain.AttendanceRecordFilter, _ interface{}) ([]domain.AttendanceRecord, error) {
	*r.filters = append(*r.filters, f)
	return nil, nil
}
```

Change `avApp()` to `NewAdminViewsHandler(avUsers{...}, avRecords{}, avStats{}, avSettings{}, avHolidays{})`. Delete `TestTodayLogsStatsDrift` (the drift check is removed). Keep `TestLoadCohortRecordsFullOnlyInRange`: the weekly report still uses `loadCohortRecords`. Add:

```go
func TestRegisterLoadsSelectedAndPreviousWeekOnly(t *testing.T) {
	var got []domain.AttendanceRecordFilter
	h := NewAdminViewsHandler(avUsers{}, avRangeRecords{&got}, avStats{}, avSettings{}, avHolidays{})
	app := fiber.New()
	app.Get("/admin/attendance/register", h.Register)
	if res, _ := app.Test(httptest.NewRequest("GET", "/admin/attendance/register?cohort=13&week=2026-09-07", nil)); res.StatusCode != 200 {
		t.Fatalf("status %d", res.StatusCode)
	}
	if len(got) != 1 || got[0].DateFrom != "2026-08-31" || got[0].DateTo != "2026-09-11" {
		t.Fatalf("filters = %+v, want 2026-08-31..2026-09-11 (previous week for KPI deltas)", got)
	}
}

func TestTodayLoadsOnlyToday(t *testing.T) {
	var got []domain.AttendanceRecordFilter
	h := NewAdminViewsHandler(avUsers{}, avRangeRecords{&got}, avStats{}, avSettings{}, avHolidays{})
	loc, _ := time.LoadLocation("Asia/Bangkok")
	h.now = func() time.Time { return time.Date(2026, 9, 8, 9, 0, 0, 0, loc) }
	app := fiber.New()
	app.Get("/admin/today", h.Today)
	app.Test(httptest.NewRequest("GET", "/admin/today?cohort=13", nil))
	if len(got) != 1 || got[0].Date != "2026-09-08" || got[0].Cohort != 13 {
		t.Fatalf("filters = %+v, want today's records only", got)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/handler/ -run 'Admin|Register|Today'`
Expected: FAIL (build error: `NewAdminViewsHandler` argument count).

- [ ] **Step 3: Rewrite the handler data loading**

In `admin_views_handler.go`:

- Add `ListCohortLearnersWithReflections(cohort int, fields bson.M, n int, textDay string) ([]domain.User, error)` to `adminViewUsers`.
- Add `stats statsReader` field; delete the `drift`, `driftSync` fields and `SetStatsDriftCheck`/`logStatsDrift` (and the `math` import).
- Constructor: `func NewAdminViewsHandler(users adminViewUsers, records adminViewRecords, stats statsReader, settings attendance.CohortSettingsGetter, holidays attendance.HolidayChecker) *AdminViewsHandler`.
- Replace `cohortData` with:

```go
func statsMap(list []domain.LearnerStats) map[primitive.ObjectID]domain.LearnerStats {
	m := make(map[primitive.ObjectID]domain.LearnerStats, len(list))
	for _, s := range list {
		m[s.UserID] = s
	}
	return m
}

// viewData loads learners, stored stats and the records for the dates on screen, in parallel.
func (h *AdminViewsHandler) viewData(cohort int, records domain.AttendanceRecordFilter, loadUsers func() ([]domain.User, error)) ([]domain.User, []domain.AttendanceRecord, map[primitive.ObjectID]domain.LearnerStats, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var (
		users []domain.User
		recs  []domain.AttendanceRecord
		stats []domain.LearnerStats
		g     errgroup.Group
	)
	g.Go(func() (err error) {
		if users, err = loadUsers(); err != nil {
			log.Printf("admin views cohort %d: users: %v", cohort, err)
		}
		return err
	})
	g.Go(func() (err error) {
		if recs, err = h.records.FindRecords(ctx, records, viewRecordFields()); err != nil {
			log.Printf("admin views cohort %d: attendance records: %v", cohort, err)
		}
		return err
	})
	g.Go(func() (err error) {
		if stats, err = h.stats.FindByCohort(ctx, cohort); err != nil {
			log.Printf("admin views cohort %d: learner stats: %v", cohort, err)
		}
		return err
	})
	if err := g.Wait(); err != nil {
		return nil, nil, nil, err
	}
	return users, recs, statsMap(stats), nil
}
```

- `Today` body after the cohort check:

```go
	now := h.now()
	today := utils.FormatDate(now)
	users, recs, stats, err := h.viewData(cohort, domain.AttendanceRecordFilter{Cohort: cohort, NotDeleted: true, Date: today}, func() ([]domain.User, error) {
		return h.users.ListCohortLearnersWithReflections(cohort, learnerFields(), 5, "")
	})
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error loading today")
	}
	st, err := h.settings.Get(cohort)
	if err != nil {
		log.Printf("admin views cohort %d: settings: %v", cohort, err)
		return utils.SendError(c, fiber.StatusInternalServerError, "Error loading today")
	}
	classDay, err := attendance.IsClassDay(now, h.holidays)
	if err != nil {
		log.Printf("admin today cohort %d: holidays: %v", cohort, err)
		return utils.SendError(c, fiber.StatusInternalServerError, "Error loading today")
	}
	classDay = classDay && st.InSession(today)
	return utils.SendResponse(c, fiber.StatusOK, "Today retrieved", attendance.BuildTodayRoster(users, recs, stats, now, st.Rules, classDay, st.FlagsOn()))
```

- `Register`: replace the `cohortData` call with

```go
	t, _ := time.Parse("2006-01-02", week)
	users, recs, stats, err := h.viewData(cohort, domain.AttendanceRecordFilter{Cohort: cohort, NotDeleted: true, DateFrom: t.AddDate(0, 0, -7).Format("2006-01-02"), DateTo: t.AddDate(0, 0, 4).Format("2006-01-02")}, func() ([]domain.User, error) {
		return h.users.ListCohortLearners(cohort, learnerFields())
	})
```

and the final call with `attendance.BuildRegister(users, recs, stats, week, hol)`.

- `loadCohortRecords`, `historyFields` and `recentReflections` stay for now (Task 11 removes `recentReflections`; `loadCohortRecords` remains for the weekly report).

`cmd/container.go`: `handler.NewAdminViewsHandler(c.UserService, c.AttendanceRepo, c.LearnerStatsRepo, c.CohortService, c.HolidayService)` and delete the `SetStatsDriftCheck` line.

- [ ] **Step 4: Run tests** (handler package builds after Task 11 too; run what builds)

Run: `go build ./internal/handler/ 2>&1 | head; go test ./internal/handler/ -run 'Admin|Register|Today'`
Expected: if only wellbeing errors remain, continue to Task 11 before running; otherwise PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/handler/admin_views_handler.go internal/handler/admin_views_handler_test.go cmd/container.go
git commit -m "perf: Today and Register read learner stats and on-screen records only

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task 11: Wellbeing views and weekly report read stats

**Files:**
- Modify: `internal/handler/wellbeing_views_handler.go`, `internal/handler/report_source.go`, `cmd/container.go:233`
- Test: `internal/handler/wellbeing_views_handler_test.go`

**Interfaces:**
- Consumes: `statsReader`, `statsMap` (Task 10), new wellbeing signatures (Task 8), `ListCohortLearnersWithReflections` (Task 9)
- Produces: `NewWellbeingViewsHandler(users wbUsers, records adminViewRecords, stats statsReader, holidays attendance.HolidayChecker, care careReader) *WellbeingViewsHandler`; `(*WellbeingViewsHandler).SupportRows(users []domain.User, stats map[primitive.ObjectID]domain.LearnerStats) []wellbeing.SupportRow`

- [ ] **Step 1: Update tests**

In `wellbeing_views_handler_test.go` add:

```go
func (u wbUsersFake) ListCohortLearnersWithReflections(int, bson.M, int, string) ([]domain.User, error) {
	return u.list, nil
}
```

and change `wbApp` to `NewWellbeingViewsHandler(wbUsersFake{users}, avRecords{}, avStats{}, avHolidays{}, careReadFake{})`. Add:

```go
func TestLearnersReadStoredStats(t *testing.T) {
	u := domain.User{ID: primitive.NewObjectID(), FirstName: "Mint", Role: "learner"}
	seen := time.Date(2026, 9, 9, 1, 55, 0, 0, time.UTC)
	h := NewWellbeingViewsHandler(wbUsersFake{[]domain.User{u}}, avRecords{}, avStats{[]domain.LearnerStats{{UserID: u.ID, AttendanceRate: 0.5, LeaveUsed: 6, LateRun: 3, LastSeen: &seen}}}, avHolidays{}, careReadFake{})
	loc, _ := time.LoadLocation("Asia/Bangkok")
	h.now = func() time.Time { return time.Date(2026, 9, 9, 18, 0, 0, 0, loc) }
	app := fiber.New()
	app.Get("/admin/learners", h.Learners)
	app.Get("/admin/support", h.Support)
	_, body := send(app, "GET", "/admin/learners?cohort=13", "")
	for _, want := range []string{`"attendance_rate":0.5`, `"leave_used":6`, `"support":true`, `"last_seen":"2026-09-09T01:55:00Z"`} {
		if !strings.Contains(body, want) {
			t.Errorf("learners body missing %s: %s", want, body)
		}
	}
	if _, body := send(app, "GET", "/admin/support?cohort=13", ""); !strings.Contains(body, `"late_run"`) || !strings.Contains(body, `"Leave 6/8"`) {
		t.Errorf("support body: %s", body)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/handler/`
Expected: FAIL (build errors in wellbeing handler).

- [ ] **Step 3: Rewrite the wellbeing data loading**

In `wellbeing_views_handler.go`:

- Add `stats statsReader` field; constructor `NewWellbeingViewsHandler(users wbUsers, records adminViewRecords, stats statsReader, holidays attendance.HolidayChecker, care careReader)`.
- Add:

```go
// textNight is the latest class night: the only night whose reflection text any view reads.
func (h *WellbeingViewsHandler) textNight() string {
	if n := h.classNights(1); len(n) > 0 {
		return n[0]
	}
	return ""
}

// learnersAndStats loads a cohort's learners (last n reflections, text for the latest night) and stored stats in parallel.
func (h *WellbeingViewsHandler) learnersAndStats(c *fiber.Ctx, n int) ([]domain.User, map[primitive.ObjectID]domain.LearnerStats, bool, error) {
	cohort := c.QueryInt("cohort")
	if cohort <= 0 {
		return nil, nil, false, utils.SendError(c, fiber.StatusBadRequest, "Invalid cohort")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var (
		users []domain.User
		stats []domain.LearnerStats
		g     errgroup.Group
	)
	g.Go(func() (err error) {
		users, err = h.users.ListCohortLearnersWithReflections(cohort, learnerFields(), n, h.textNight())
		return err
	})
	g.Go(func() (err error) {
		stats, err = h.stats.FindByCohort(ctx, cohort)
		return err
	})
	if err := g.Wait(); err != nil {
		log.Printf("wellbeing cohort %d: %v", cohort, err)
		return nil, nil, false, utils.SendError(c, fiber.StatusInternalServerError, "Error loading learners")
	}
	return users, statsMap(stats), true, nil
}
```

- Delete `usersAndRecords`. Keep `cohortRecords` (weekly report).
- `Board`: replace the whole function body with:

```go
	cohort := c.QueryInt("cohort")
	if cohort <= 0 {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid cohort")
	}
	rng := c.Query("range", "night")
	if rng != "night" && rng != "week" && rng != "4w" {
		return utils.SendError(c, fiber.StatusBadRequest, "range must be night, week or 4w")
	}
	nights := h.classNights(20)
	textDay := ""
	if len(nights) > 0 {
		textDay = nights[len(nights)-1]
	}
	users, err := h.users.ListCohortLearnersWithReflections(cohort, learnerFields(), 30, textDay)
	if err != nil {
		log.Printf("wellbeing cohort %d: users: %v", cohort, err)
		return utils.SendError(c, fiber.StatusInternalServerError, "Error loading learners")
	}
	return utils.SendResponse(c, fiber.StatusOK, "Reflections", wellbeing.BuildBoard(users, nights, rng, wellbeing.DefaultWords))
```
- `Learners`: `users, stats, ok, err := h.learnersAndStats(c, 20)`; delete the `byUser` grouping; in the loop:

```go
		st := stats[u.ID]
		row := learnerRow{LearnerRef: wellbeing.Ref(u), Email: u.Email, AttendanceStatus: u.AttendanceStatus, AttendanceRate: st.AttendanceRate, LeaveUsed: st.LeaveUsed, LastSeen: st.LastSeen, Mood: make([]string, len(nights))}
```

  delete the `LastSeen` loop over records; `ReasonsFor(u, st, items, acks, recent, wellbeing.DefaultWords)`.
- `Support`: `users, stats, ok, err := h.learnersAndStats(c, 20)`; `h.SupportRows(users, stats)`.
- `SupportRows(users []domain.User, stats map[primitive.ObjectID]domain.LearnerStats)` → `wellbeing.BuildSupportList(users, stats, items, acks, h.classNights(2), wellbeing.DefaultWords)`.
- `Overview`: `reasons, _, _ := wellbeing.ReasonsFor(*u, attendance.ComputeStats(recs), items, acks, recent, wellbeing.DefaultWords)`.

In `admin_views_handler.go` delete `recentReflections` (now unused).

`report_source.go`: `Support: s.WB.SupportRows(users, attendance.StatsByUser(recs))` (import `gofiber-baro/internal/service/attendance`).

`cmd/container.go:233`: `handler.NewWellbeingViewsHandler(c.UserService, c.AttendanceRepo, c.LearnerStatsRepo, c.HolidayService, c.CareRepo)`.

- [ ] **Step 4: Run the full backend checks**

Run: `go build ./... && go vet ./internal/... ./cmd/... && go test ./internal/...`
Expected: all `ok` (including the existing `TestWellbeingViews` cases).

- [ ] **Step 5: Commit**

```bash
git add internal/handler/wellbeing_views_handler.go internal/handler/wellbeing_views_handler_test.go internal/handler/admin_views_handler.go internal/handler/report_source.go cmd/container.go
git commit -m "perf: wellbeing views read learner stats and slim reflections

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task 12: Slim `GET /users` and `GET /users/:id`

**Files:**
- Modify: `internal/handler/user_handler.go` (`GetAllUsers` projection, `GetUserByID`)
- Test: `internal/handler/user_list_fields_test.go`

**Interfaces:**
- Consumes: `GetUsersWithFields` (existing, #98), `GetUserByIDWithFields` (Task 9)
- Produces: `userListFields bson.M` (package var)

- [ ] **Step 1: Write the failing test**

```go
package handler

import "testing"

func TestUserListFieldsDropHeavyFields(t *testing.T) {
	for _, f := range []string{"reflections", "password", "profile_comments", "profile_reactions", "plant_reactions", "care_energy_log", "gift_boxes", "owned_cosmetic_ids", "new_cosmetic_ids"} {
		if v, ok := userListFields[f]; !ok || v != 0 {
			t.Errorf("%s not excluded from GET /users", f)
		}
	}
	for _, keep := range []string{"badges", "social_links", "bio", "skills", "avatar"} {
		if _, excluded := userListFields[keep]; excluded {
			t.Errorf("%s must stay in GET /users", keep)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/handler/ -run UserListFields`
Expected: FAIL, `undefined: userListFields`.

- [ ] **Step 3: Implement**

In `user_handler.go` add:

```go
// userListFields drops what no list screen reads; the classmate panel loads a learner's
// recommendations from GET /users/:id when opened.
var userListFields = bson.M{
	"reflections": 0, "password": 0, "profile_comments": 0, "profile_reactions": 0, "plant_reactions": 0,
	"care_energy_log": 0, "gift_boxes": 0, "owned_cosmetic_ids": 0, "new_cosmetic_ids": 0,
}
```

In `GetAllUsers` replace `bson.M{"reflections": 0, "password": 0}` with `userListFields`.

In `GetUserByID`, replace `user, err := h.userService.GetUserByID(id)` with:

```go
	var user *domain.User
	var err error
	if claims.Role == "admin" || claims.UserID == id {
		user, err = h.userService.GetUserByID(id)
	} else {
		user, err = h.userService.GetUserByIDWithFields(id, bson.M{"reflections": 0, "password": 0})
	}
```

(`isAdmin` below stays as is.)

- [ ] **Step 4: Run the full backend checks**

Run: `go build ./... && go vet ./internal/... ./cmd/... && go test ./internal/...`
Expected: all `ok`.

- [ ] **Step 5: Commit**

```bash
git add internal/handler/user_handler.go internal/handler/user_list_fields_test.go
git commit -m "perf: GET /users drops comments, reactions and logs; classmates skip reflections

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task 13: Ship PR 3 and verify in production

- [ ] **Step 1: Open and merge the PR (after the user says merge)**

```bash
cd /d/code/baro/baro-gofiber && [ "$(basename "$(git rev-parse --show-toplevel)")" = baro-gofiber ] && git push -u origin perf/read-learner-stats && gh pr create --base main --title "perf: admin views read learner stats; slim reflections and /users" --body "Today/Register/Learners/Support read learner_stats and only the records on screen (Register also loads the previous week for KPI deltas). Reflections come through an aggregation that keeps text only for the latest class night. GET /users drops comments/reactions/logs; GET /users/:id skips reflections for classmates. Drift check removed. Spec: docs/superpowers/specs/2026-11-26-learner-stats-design.md

🤖 Generated with [Claude Code](https://claude.com/claude-code)" && gh pr merge --merge --delete-branch && git checkout main && git pull
```

- [ ] **Step 2: After redeploy, check the latency logs**

Expected over ~10 minutes of normal use: no `slow request` lines for `/admin/today`, `/admin/attendance/register`, `/admin/learners`, `/admin/support`, `/users`; `/admin/reflections/board` at most ~1 s. No `find users` / `getMore` above 200 KB from these pages. If a page is still slow, the paired `slow mongo` line names the query to look at next.
