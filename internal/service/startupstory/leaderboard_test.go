package startupstory

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func (f *fakeStore) boardName(ownerID primitive.ObjectID) string {
	if name, ok := f.boardNames[ownerID.Hex()]; ok && name != "" {
		return name
	}
	return "?"
}

func (f *fakeStore) LeaderboardWeekly(_ context.Context, cohort int, weekKey string, limit int) ([]domain.StartupLeaderboardEntry, error) {
	best := map[string]*domain.StartupLeaderboardEntry{}
	for _, r := range f.boardRuns {
		if r.Cohort != cohort || r.Mode != domain.StartupModeRanked || r.WeekKey != weekKey || r.Status != domain.StartupStatusEnded {
			continue
		}
		if f.boardRoles[r.OwnerID.Hex()] == "admin" {
			continue
		}
		key := r.OwnerID.Hex()
		if cur, ok := best[key]; !ok || r.Score > cur.Score {
			best[key] = &domain.StartupLeaderboardEntry{OwnerID: r.OwnerID, Name: f.boardName(r.OwnerID), Score: r.Score, BestRunID: r.ID, Outcome: r.Outcome}
		}
	}
	out := []domain.StartupLeaderboardEntry{}
	for _, e := range best {
		out = append(out, *e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (f *fakeStore) LeaderboardDeepest(_ context.Context, cohort int, weekKey string, limit int) ([]domain.StartupLeaderboardEntry, error) {
	best := map[string]*domain.StartupLeaderboardEntry{}
	deeper := func(a, b domain.StartupLeaderboardEntry) bool {
		return a.MaxAct > b.MaxAct || (a.MaxAct == b.MaxAct && a.Score > b.Score)
	}
	for _, r := range f.boardRuns {
		if r.Cohort != cohort || r.Mode != domain.StartupModeRanked || r.WeekKey != weekKey || r.Status != domain.StartupStatusEnded || f.boardRoles[r.OwnerID.Hex()] == "admin" {
			continue
		}
		e := domain.StartupLeaderboardEntry{OwnerID: r.OwnerID, Name: f.boardName(r.OwnerID), MaxAct: r.MaxAct, Score: r.Score, BestRunID: r.ID, Outcome: r.Outcome}
		if cur, ok := best[r.OwnerID.Hex()]; !ok || deeper(e, *cur) {
			best[r.OwnerID.Hex()] = &e
		}
	}
	out := []domain.StartupLeaderboardEntry{}
	for _, e := range best {
		out = append(out, *e)
	}
	sort.Slice(out, func(i, j int) bool { return deeper(out[i], out[j]) })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (f *fakeStore) LeaderboardFame(_ context.Context, cohort int, limit int) ([]domain.StartupLeaderboardEntry, error) {
	out := []domain.StartupLeaderboardEntry{}
	for _, s := range f.boardStudios {
		if s.Cohort != cohort || f.boardRoles[s.OwnerID.Hex()] == "admin" {
			continue
		}
		out = append(out, domain.StartupLeaderboardEntry{OwnerID: s.OwnerID, Name: f.boardName(s.OwnerID), Fame: s.Fame})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Fame > out[j].Fame })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func boardFixture() *fakeStore {
	f := &fakeStore{boardNames: map[string]string{}, boardRoles: map[string]string{}}
	learnerA := primitive.NewObjectID()
	learnerB := primitive.NewObjectID()
	admin := primitive.NewObjectID()
	otherCohort := primitive.NewObjectID()
	f.boardNames[learnerA.Hex()] = "Ploy"
	f.boardNames[learnerB.Hex()] = ""
	f.boardNames[admin.Hex()] = "Boss"
	f.boardNames[otherCohort.Hex()] = "Away"
	f.boardRoles[learnerA.Hex()] = "learner"
	f.boardRoles[learnerB.Hex()] = "learner"
	f.boardRoles[admin.Hex()] = "admin"
	f.boardRoles[otherCohort.Hex()] = "learner"
	ended := func(owner primitive.ObjectID, cohort int, score int, week string) *domain.StartupRun {
		return &domain.StartupRun{ID: primitive.NewObjectID(), OwnerID: owner, Cohort: cohort, Role: "learner", Mode: domain.StartupModeRanked, WeekKey: week, Status: domain.StartupStatusEnded, Outcome: domain.StartupOutcomeIPO, Score: score}
	}
	f.boardRuns = []*domain.StartupRun{
		ended(learnerA, 12, 100, "2026-W40"),
		ended(learnerA, 12, 300, "2026-W40"),
		ended(learnerB, 12, 200, "2026-W40"),
		ended(admin, 12, 999, "2026-W40"),
		ended(otherCohort, 13, 500, "2026-W40"),
		ended(learnerA, 12, 400, "2026-W39"),
		{ID: primitive.NewObjectID(), OwnerID: learnerB, Cohort: 12, Role: "learner", Mode: domain.StartupModeRanked, WeekKey: "2026-W40", Status: domain.StartupStatusActive, Score: 1000},
	}
	f.boardStudios = []*domain.StartupStudio{
		{OwnerID: learnerA, Cohort: 12, Fame: 10},
		{OwnerID: learnerB, Cohort: 12, Fame: 30},
		{OwnerID: admin, Cohort: 12, Fame: 999},
		{OwnerID: otherCohort, Cohort: 13, Fame: 50},
	}
	return f
}

func TestWeeklyRankingOneRowPerPerson(t *testing.T) {
	ctx := context.Background()
	svc := &Service{store: boardFixture(), now: func() time.Time { return t0 }}
	player := Player{ID: primitive.NewObjectID().Hex(), Cohort: 12, Role: "learner"}

	entries, err := svc.Leaderboard(ctx, player, "weekly")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 learners, got %+v", entries)
	}
	if entries[0].Score != 300 || entries[1].Score != 200 {
		t.Fatalf("expected best-per-person desc: %+v", entries)
	}
	if entries[0].BestRunID.IsZero() || entries[0].Outcome == "" {
		t.Fatalf("best run should be identified: %+v", entries[0])
	}
	for _, e := range entries {
		if e.Name == "Boss" || e.Name == "Away" {
			t.Fatalf("admin and other cohorts excluded: %+v", entries)
		}
	}
}

func TestWeeklyNameFallsBackToGenID(t *testing.T) {
	f := boardFixture()
	f.boardNames[f.boardRuns[2].OwnerID.Hex()] = ""
	ctx := context.Background()
	svc := &Service{store: f, now: func() time.Time { return t0 }}
	player := Player{ID: primitive.NewObjectID().Hex(), Cohort: 12, Role: "learner"}
	entries, err := svc.Leaderboard(ctx, player, "weekly")
	if err != nil {
		t.Fatal(err)
	}
	if entries[1].Name != "?" {
		t.Fatalf("empty first name should fall back, got %q", entries[1].Name)
	}
}

func TestFameRankingExcludesAdmins(t *testing.T) {
	ctx := context.Background()
	svc := &Service{store: boardFixture(), now: func() time.Time { return t0 }}
	player := Player{ID: primitive.NewObjectID().Hex(), Cohort: 12, Role: "learner"}

	entries, err := svc.Leaderboard(ctx, player, "fame")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].Fame != 30 || entries[1].Fame != 10 {
		t.Fatalf("expected fame desc without admins: %+v", entries)
	}
	if entries[0].Name != "?" {
		t.Fatalf("expected gen-id fallback for unnamed learner, got %q", entries[0].Name)
	}
}

func TestLeaderboardRejectsBadTab(t *testing.T) {
	ctx := context.Background()
	svc := &Service{store: boardFixture(), now: func() time.Time { return t0 }}
	player := Player{ID: primitive.NewObjectID().Hex(), Cohort: 12, Role: "learner"}
	if _, err := svc.Leaderboard(ctx, player, "alltime"); !errors.Is(err, domain.ErrStartupInvalidChoice) {
		t.Fatalf("expected invalid choice, got %v", err)
	}
	if _, err := svc.Leaderboard(ctx, player, ""); err != nil {
		t.Fatalf("empty tab should default to weekly, got %v", err)
	}
}
