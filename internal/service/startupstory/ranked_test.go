package startupstory

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestWeekKeyFormat(t *testing.T) {
	bangkok := bangkokLocation()
	noon := time.Date(2026, 10, 1, 12, 0, 0, 0, bangkok)
	if got := weekKeyFor(noon); got != "2026-W40" {
		t.Fatalf("got %q", got)
	}
	edge := time.Date(2026, 9, 28, 0, 30, 0, 0, time.UTC)
	if got := weekKeyFor(edge); got != "2026-W40" {
		t.Fatalf("UTC 00:30 Monday is 07:30 Bangkok Monday, got %q", got)
	}
	sundayNight := time.Date(2026, 9, 27, 16, 0, 0, 0, time.UTC)
	if got := weekKeyFor(sundayNight); got != "2026-W39" {
		t.Fatalf("UTC Sunday evening is still Sunday in Bangkok, got %q", got)
	}
}

func TestWeekSeedDeterministic(t *testing.T) {
	if WeekSeed("2026-W40") != WeekSeed("2026-W40") {
		t.Fatal("same week should seed identically")
	}
	if WeekSeed("2026-W40") == WeekSeed("2026-W41") {
		t.Fatal("different weeks should seed differently")
	}
}

func playRankedWeek(t *testing.T, weekKey string) *domain.StartupRun {
	t.Helper()
	run := newRunWithPool(primitive.NewObjectID(), 12, "learner", domain.StartupModeRanked, WeekSeed(weekKey), t0, founderPool(nil), nil)
	run.WeekKey = weekKey
	if err := PickFounder(run, 0); err != nil {
		t.Fatal(err)
	}
	if err := StartProject(run, "Game", "Thai Culture", nil, t0); err != nil {
		t.Fatal(err)
	}
	if err := Ship(run, run.Project.EndsAt); err != nil {
		t.Fatal(err)
	}
	return run
}

func TestRankedSameWeekSameChoices(t *testing.T) {
	a := playRankedWeek(t, "2026-W40")
	b := playRankedWeek(t, "2026-W40")
	if !reflect.DeepEqual(a.Market, b.Market) || !reflect.DeepEqual(a.BossOrder, b.BossOrder) ||
		!reflect.DeepEqual(a.FounderOffer, b.FounderOffer) || !reflect.DeepEqual(a.LastResult, b.LastResult) {
		t.Fatal("same week + same choices should play identically")
	}
	other := playRankedWeek(t, "2026-W41")
	if reflect.DeepEqual(a.Market, other.Market) && reflect.DeepEqual(a.BossOrder, other.BossOrder) {
		t.Fatal("different weeks should roll differently")
	}
}

func rankedPlayer() Player {
	return Player{ID: primitive.NewObjectID().Hex(), Cohort: 12, Role: "learner"}
}

func TestRankedAttemptLimit(t *testing.T) {
	ctx := context.Background()
	store := &fakeStore{}
	svc := &Service{store: store, now: func() time.Time { return t0 }}
	player := rankedPlayer()

	for i := 0; i < MaxRankedAttempts; i++ {
		if _, err := svc.StartRun(ctx, player, domain.StartupModeRanked); err != nil {
			t.Fatalf("attempt %d: %v", i+1, err)
		}
		if _, err := svc.PickFounder(ctx, player, 0); err != nil {
			t.Fatal(err)
		}
		if _, err := svc.Abandon(ctx, player); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := svc.StartRun(ctx, player, domain.StartupModeRanked); !errors.Is(err, domain.ErrStartupNoAttempts) {
		t.Fatalf("4th ranked run should be rejected, got %v", err)
	}
	if _, err := svc.StartRun(ctx, player, domain.StartupModeFree); err != nil {
		t.Fatalf("free play should stay unlimited, got %v", err)
	}
}

func TestRankedSharesSeedAcrossAccounts(t *testing.T) {
	ctx := context.Background()
	mkSvc := func() (*Service, *fakeStore) {
		store := &fakeStore{}
		return &Service{store: store, now: func() time.Time { return t0 }}, store
	}
	svcA, _ := mkSvc()
	svcB, _ := mkSvc()
	runA, err := svcA.StartRun(ctx, rankedPlayer(), domain.StartupModeRanked)
	if err != nil {
		t.Fatal(err)
	}
	runB, err := svcB.StartRun(ctx, rankedPlayer(), domain.StartupModeRanked)
	if err != nil {
		t.Fatal(err)
	}
	if runA.Seed != runB.Seed || runA.WeekKey == "" || runA.WeekKey != runB.WeekKey {
		t.Fatalf("ranked runs should share the weekly seed: %+v vs %+v", runA, runB)
	}
	if !reflect.DeepEqual(runA.FounderOffer, runB.FounderOffer) {
		t.Fatal("same seed should deal the same founders")
	}
}

func TestOverviewAttemptsLeft(t *testing.T) {
	ctx := context.Background()
	store := &fakeStore{}
	svc := &Service{store: store, now: func() time.Time { return t0 }}
	player := rankedPlayer()

	over, err := svc.Overview(ctx, player)
	if err != nil {
		t.Fatal(err)
	}
	if over.RankedAttemptsLeft != MaxRankedAttempts || over.WeekKey == "" {
		t.Fatalf("fresh overview: %+v", over)
	}
	if _, err := svc.StartRun(ctx, player, domain.StartupModeRanked); err != nil {
		t.Fatal(err)
	}
	over, err = svc.Overview(ctx, player)
	if err != nil {
		t.Fatal(err)
	}
	if over.RankedAttemptsLeft != MaxRankedAttempts-1 {
		t.Fatalf("expected %d left, got %+v", MaxRankedAttempts-1, over)
	}
}
