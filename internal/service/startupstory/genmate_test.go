package startupstory

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func genmateFixture() (*fakeStore, []domain.StartupGenmate) {
	pool := []domain.StartupGenmate{}
	for _, name := range []string{"Ploy", "Nat", "Kan", "Mint", "Tee", "Bank", "Fah", "Aom"} {
		pool = append(pool, domain.StartupGenmate{UserID: primitive.NewObjectID(), Name: name})
	}
	return &fakeStore{pool: pool}, pool
}

func startHubWithPool(t *testing.T, svc *Service, ctx context.Context, player Player) *domain.StartupRun {
	t.Helper()
	if _, err := svc.StartRun(ctx, player, domain.StartupModeFree); err != nil {
		t.Fatal(err)
	}
	run, err := svc.PickFounder(ctx, player, 0)
	if err != nil {
		t.Fatal(err)
	}
	return run
}

func TestPoolSnapshottedAtCreation(t *testing.T) {
	ctx := context.Background()
	store, pool := genmateFixture()
	svc := &Service{store: store, now: func() time.Time { return t0 }}
	player := Player{ID: primitive.NewObjectID().Hex(), Cohort: 12, Role: "learner"}

	run, err := svc.StartRun(ctx, player, domain.StartupModeFree)
	if err != nil {
		t.Fatal(err)
	}
	if len(run.GenmatePool) != len(pool) {
		t.Fatalf("pool should snapshot %d genmates, got %d", len(pool), len(run.GenmatePool))
	}
	for _, g := range run.GenmatePool {
		if g.UserID.Hex() == player.ID {
			t.Fatal("self should never be in the pool")
		}
	}
}

func TestOptOutExcludesFromNextPool(t *testing.T) {
	ctx := context.Background()
	store, pool := genmateFixture()
	svc := &Service{store: store, now: func() time.Time { return t0 }}
	player := Player{ID: primitive.NewObjectID().Hex(), Cohort: 12, Role: "learner"}

	out, err := svc.OptOut(ctx, player, true)
	if err != nil || !out {
		t.Fatalf("opt-out should stick: %v %v", out, err)
	}
	run, err := svc.StartRun(ctx, player, domain.StartupModeFree)
	if err != nil {
		t.Fatal(err)
	}
	if len(run.GenmatePool) != len(pool) {
		t.Fatal("other users opting out should not shrink this pool")
	}
	genie := pool[0]
	if _, err := svc.OptOut(ctx, Player{ID: genie.UserID.Hex(), Cohort: 12, Role: "learner"}, true); err != nil {
		t.Fatal(err)
	}
	run2, err := svc.StartRun(ctx, player, domain.StartupModeFree)
	if err != nil {
		if _, err := svc.Abandon(ctx, player); err != nil {
			t.Fatal(err)
		}
		if run2, err = svc.StartRun(ctx, player, domain.StartupModeFree); err != nil {
			t.Fatal(err)
		}
	}
	for _, g := range run2.GenmatePool {
		if g.UserID == genie.UserID {
			t.Fatal("opted-out genmate must never appear")
		}
	}
	out, err = svc.OptOut(ctx, player, false)
	if err != nil || out {
		t.Fatalf("opt-in should stick: %v %v", out, err)
	}
}

func hubWithGenmate(t *testing.T, poolSize int) *domain.StartupRun {
	t.Helper()
	for seed := uint64(1); ; seed++ {
		run := NewRun(primitive.NewObjectID(), 12, "learner", domain.StartupModeFree, seed, t0)
		for i := 0; i < poolSize; i++ {
			run.GenmatePool = append(run.GenmatePool, domain.StartupGenmate{UserID: primitive.NewObjectID(), Name: "Genie"})
		}
		if err := PickFounder(run, 0); err != nil {
			t.Fatal(err)
		}
		for _, c := range run.Candidates {
			if c.GenmateID != "" {
				return run
			}
		}
		if seed > 1000 {
			t.Fatal("no seed dealt a genmate")
		}
	}
}

func TestGenmateDrawsAndStatBounds(t *testing.T) {
	run := hubWithGenmate(t, 8)
	seenGenmate := false
	for _, c := range run.Candidates {
		if c.GenmateID != "" {
			seenGenmate = true
			matched := false
			for _, g := range run.GenmatePool {
				if g.UserID.Hex() == c.GenmateID && g.Name == c.Name {
					matched = true
				}
			}
			if !matched {
				t.Fatalf("genmate candidate must come from the snapshot: %+v", c)
			}
		}
		for _, s := range [4]int{c.Frontend, c.Backend, c.Design, c.Debug} {
			if s < 1 || s > statMax(run.Act)+2 {
				t.Fatalf("genmate stats rolled like everyone else: %+v", c)
			}
		}
	}
	if !seenGenmate {
		t.Fatal("expected a genmate in this offer")
	}
}

func TestGenmateDrawRate(t *testing.T) {
	genmates := 0
	rolls := 0
	for seed := uint64(100); seed < 300; seed++ {
		run := NewRun(primitive.NewObjectID(), 12, "learner", domain.StartupModeFree, seed, t0)
		pool := []domain.StartupGenmate{}
		for i := 0; i < 8; i++ {
			pool = append(pool, domain.StartupGenmate{UserID: primitive.NewObjectID(), Name: "G"})
		}
		run.GenmatePool = pool
		for _, c := range rollCandidates(run, CandidateOffers) {
			rolls++
			if c.GenmateID != "" {
				genmates++
			}
		}
	}
	rate := float64(genmates) / float64(rolls)
	if rate < 0.15 || rate > 0.35 {
		t.Fatalf("genmate draw rate %0.2f outside 25%% tolerance", rate)
	}
}

func TestHiredGenmateNotRedrawn(t *testing.T) {
	run := hubWithGenmate(t, 1)
	genieID := ""
	hired := ""
	for _, c := range run.Candidates {
		if c.GenmateID != "" {
			genieID = c.GenmateID
			hired = c.ID
			break
		}
	}
	if err := Hire(run, hired); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		for _, c := range rollCandidates(run, CandidateOffers) {
			if c.GenmateID == genieID {
				t.Fatal("hired genmate should not be redrawn")
			}
		}
	}
}

func TestOverviewReportsOptOutState(t *testing.T) {
	ctx := context.Background()
	store, _ := genmateFixture()
	svc := &Service{store: store, now: func() time.Time { return t0 }}
	player := Player{ID: primitive.NewObjectID().Hex(), Cohort: 12, Role: "learner"}

	ov, err := svc.Overview(ctx, player)
	if err != nil {
		t.Fatal(err)
	}
	if ov.OptOut {
		t.Fatal("a new player starts opted in")
	}
	if _, err := svc.OptOut(ctx, player, true); err != nil {
		t.Fatal(err)
	}
	ov, err = svc.Overview(ctx, player)
	if err != nil {
		t.Fatal(err)
	}
	if !ov.OptOut {
		t.Fatal("overview must reflect the saved preference so the Lobby toggle can render it")
	}
}

func TestCandidateExposesNoSurnameOrEmail(t *testing.T) {
	run := NewRun(primitive.NewObjectID(), 12, "learner", domain.StartupModeFree, 81, t0)
	run.GenmatePool = []domain.StartupGenmate{{UserID: primitive.NewObjectID(), Name: "Ploy"}}
	if err := PickFounder(run, 0); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(run.Candidates)
	if err != nil {
		t.Fatal(err)
	}
	lowered := strings.ToLower(string(raw))
	for _, leak := range []string{"email", "surname", "last_name", "lastname"} {
		if strings.Contains(lowered, leak) {
			t.Fatalf("candidate response must not contain %q: %s", leak, raw)
		}
	}
}
