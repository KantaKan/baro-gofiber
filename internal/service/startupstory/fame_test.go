package startupstory

import (
	"context"
	"testing"
	"time"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestFameMath(t *testing.T) {
	if fameGainFor(34823) != 348 {
		t.Fatalf("got %d", fameGainFor(34823))
	}
	if fameGainFor(50) != 1 || fameGainFor(0) != 1 || fameGainFor(-500) != 1 {
		t.Fatal("tiny scores should still earn 1 fame")
	}
}

func TestUnlockThresholds(t *testing.T) {
	f, items, skin := unlocksFor(0)
	if len(f) != 0 || len(items) != 0 || skin != "" {
		t.Fatalf("no unlocks at 0 fame: %v %v %q", f, items, skin)
	}
	f, _, _ = unlocksFor(20)
	if len(f) != 1 || f[0] != "Ex-FAANG Refugee" {
		t.Fatalf("20 fame should unlock FAANG: %v", f)
	}
	_, items, _ = unlocksFor(50)
	if len(items) != 2 {
		t.Fatalf("50 fame should unlock 2 items: %v", items)
	}
	f, _, _ = unlocksFor(100)
	if len(f) != 2 {
		t.Fatalf("100 fame should unlock 2 founders: %v", f)
	}
	_, _, skin = unlocksFor(200)
	if skin != "rooftop-bangkok" {
		t.Fatalf("200 fame should unlock the skin, got %q", skin)
	}
}

func TestLockedEntriesExcludedFromPools(t *testing.T) {
	base := founderPool(nil)
	for _, f := range base {
		if f.Title == "Ex-FAANG Refugee" || f.Title == "Genmate Legend" {
			t.Fatalf("locked founder in base pool: %v", f.Title)
		}
	}
	withUnlock := founderPool([]string{"Ex-FAANG Refugee"})
	found := false
	for _, f := range withUnlock {
		if f.Title == "Ex-FAANG Refugee" {
			found = true
		}
	}
	if !found {
		t.Fatal("unlocked founder should join the pool")
	}
	pool := itemPool(nil)
	for _, it := range pool {
		if it.ID == "copilot-subscription" || it.ID == "standing-desk" {
			t.Fatalf("locked item in base pool: %v", it.ID)
		}
	}
	pool = itemPool([]string{"copilot-subscription", "standing-desk"})
	if len(pool) != len(itemCatalog)+2 {
		t.Fatalf("unlocked items should join the pool: %d", len(pool))
	}
	if _, ok := findItem("copilot-subscription"); !ok {
		t.Fatal("unlocked items should resolve for effects")
	}
}

func TestUnlockedFounderCanAppearInOffer(t *testing.T) {
	seen := false
	for seed := uint64(1); seed <= 30 && !seen; seed++ {
		run := newRunWithPool(primitive.NewObjectID(), 12, "learner", domain.StartupModeFree, seed, t0, founderPool([]string{"Genmate Legend"}), nil)
		for _, f := range run.FounderOffer {
			if f.Title == "Genmate Legend" {
				seen = true
			}
		}
	}
	if !seen {
		t.Fatal("unlocked founder never appeared in 30 offers")
	}
}

func TestPickFounderRemembersTitle(t *testing.T) {
	run := NewRun(primitive.NewObjectID(), 12, "learner", domain.StartupModeFree, 70, t0)
	if err := PickFounder(run, 1); err != nil {
		t.Fatal(err)
	}
	if run.Founder == "" {
		t.Fatal("run should remember the picked founder")
	}
}

func settleEndedRun(t *testing.T, svc *Service, ctx context.Context, store *fakeStore, player Player) *domain.StartupRun {
	t.Helper()
	if _, err := svc.StartRun(ctx, player, domain.StartupModeFree); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.PickFounder(ctx, player, 0); err != nil {
		t.Fatal(err)
	}
	run, err := svc.Abandon(ctx, player)
	if err != nil {
		t.Fatal(err)
	}
	return run
}

func TestAbandonSettlesFameOnce(t *testing.T) {
	ctx := context.Background()
	store := &fakeStore{}
	svc := &Service{store: store, now: func() time.Time { return t0 }}
	player := Player{ID: primitive.NewObjectID().Hex(), Cohort: 12, Role: "learner"}

	run := settleEndedRun(t, svc, ctx, store, player)
	if store.studio.Fame != fameGainFor(run.Score) {
		t.Fatalf("fame %d want %d", store.studio.Fame, fameGainFor(run.Score))
	}
	if len(store.studio.HallOfFame) != 1 || store.studio.HallOfFame[0].RunID != run.ID {
		t.Fatalf("hall should record the run: %+v", store.studio.HallOfFame)
	}
	entry := hallEntryFor(run)
	f, items, skin := unlocksFor(store.studio.Fame)
	before := store.studio.Fame
	if err := store.SettleRun(ctx, store.studio.OwnerID, entry, fameGainFor(run.Score), f, items, skin); err != nil {
		t.Fatal(err)
	}
	if store.studio.Fame != before || len(store.studio.HallOfFame) != 1 {
		t.Fatal("settling the same run twice must be a no-op")
	}
}

func TestHallKeepsLast20(t *testing.T) {
	ctx := context.Background()
	store := &fakeStore{studio: &domain.StartupStudio{}}
	for i := 0; i < 25; i++ {
		entry := domain.StartupHallEntry{RunID: primitive.NewObjectID(), Score: i, Outcome: domain.StartupOutcomePivot}
		if err := store.SettleRun(ctx, primitive.NewObjectID(), entry, 1, nil, nil, ""); err != nil {
			t.Fatal(err)
		}
	}
	if len(store.studio.HallOfFame) != 20 {
		t.Fatalf("hall should keep 20, got %d", len(store.studio.HallOfFame))
	}
	if store.studio.HallOfFame[19].Score != 24 {
		t.Fatal("hall should keep the latest runs")
	}
	if store.studio.Fame != 25 {
		t.Fatalf("fame should accumulate: %d", store.studio.Fame)
	}
}

func TestBigWinUnlocksFounderAndSkin(t *testing.T) {
	ctx := context.Background()
	store := &fakeStore{}
	svc := &Service{store: store, now: func() time.Time { return t0 }}
	player := Player{ID: primitive.NewObjectID().Hex(), Cohort: 12, Role: "learner"}

	if _, err := svc.StartRun(ctx, player, domain.StartupModeFree); err != nil {
		t.Fatal(err)
	}
	active, _ := store.FindActiveRun(ctx, primitive.NilObjectID)
	active.Score = 25000
	active.Status = domain.StartupStatusEnded
	active.Outcome = domain.StartupOutcomeIPO
	active.Stage = domain.StartupStageEnded
	active.Founder = "Bootcamp Grad"
	now := t0
	active.EndedAt = &now
	store.run = active

	gain := fameGainFor(25000)
	f, items, skin := unlocksFor(store.studio.Fame + gain)
	if err := store.SettleRun(ctx, active.OwnerID, hallEntryFor(active), gain, f, items, skin); err != nil {
		t.Fatal(err)
	}
	if store.studio.Fame != gain {
		t.Fatalf("fame %d want %d", store.studio.Fame, gain)
	}
	if len(store.studio.UnlockedFounders) != 2 || len(store.studio.UnlockedItems) != 2 {
		t.Fatalf("250 fame should unlock all founders+items: %+v %+v", store.studio.UnlockedFounders, store.studio.UnlockedItems)
	}
	if store.studio.OfficeSkin != "rooftop-bangkok" {
		t.Fatalf("250 fame should unlock the skin: %q", store.studio.OfficeSkin)
	}
}
