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

func shipToDraft(t *testing.T, seed uint64) *domain.StartupRun {
	t.Helper()
	run := NewRun(primitive.NewObjectID(), 12, "learner", domain.StartupModeFree, seed, t0)
	run.Market = domain.StartupMarket{}
	if err := PickFounder(run, 0); err != nil {
		t.Fatal(err)
	}
	if err := StartProject(run, "Web App", "Education", nil, t0); err != nil {
		t.Fatal(err)
	}
	if err := Ship(run, run.Project.EndsAt); err != nil {
		t.Fatal(err)
	}
	return run
}

func TestShipDealsStoredItemOffer(t *testing.T) {
	run := shipToDraft(t, 31)
	if run.Stage != domain.StartupStageItem {
		t.Fatalf("expected item stage, got %s", run.Stage)
	}
	if len(run.ItemOffer) != CandidateOffers {
		t.Fatalf("expected %d offers, got %+v", CandidateOffers, run.ItemOffer)
	}
	seen := map[string]bool{}
	for _, id := range run.ItemOffer {
		if _, ok := findItem(id); !ok {
			t.Fatalf("unknown item %q", id)
		}
		if seen[id] {
			t.Fatalf("duplicate offer %q", id)
		}
		seen[id] = true
	}
	again := shipToDraft(t, 31)
	if !reflect.DeepEqual(run.ItemOffer, again.ItemOffer) {
		t.Fatal("same seed should deal the same offer")
	}
}

func TestPickItemAdvancesToHub(t *testing.T) {
	run := shipToDraft(t, 32)
	want := run.ItemOffer[1]
	if err := PickItem(run, 1); err != nil {
		t.Fatal(err)
	}
	if len(run.Items) != 1 || run.Items[0] != want {
		t.Fatalf("expected drafted %q, got %+v", want, run.Items)
	}
	if run.ItemOffer != nil {
		t.Fatalf("offer should clear after draft, got %+v", run.ItemOffer)
	}
	if run.Stage != domain.StartupStageHub {
		t.Fatalf("expected hub after draft, got %s", run.Stage)
	}
}

func TestPickItemGuards(t *testing.T) {
	run := shipToDraft(t, 33)
	if err := PickItem(run, -2); !errors.Is(err, domain.ErrStartupInvalidChoice) {
		t.Fatalf("expected invalid choice, got %v", err)
	}
	if err := PickItem(run, len(run.ItemOffer)); !errors.Is(err, domain.ErrStartupInvalidChoice) {
		t.Fatalf("expected invalid choice, got %v", err)
	}
	if err := PickItem(run, 0); err != nil {
		t.Fatal(err)
	}
	if err := PickItem(run, 0); !errors.Is(err, domain.ErrStartupWrongStage) {
		t.Fatalf("expected wrong stage in hub, got %v", err)
	}
}

func TestStartProjectBlockedDuringDraft(t *testing.T) {
	run := shipToDraft(t, 34)
	if err := StartProject(run, "Web App", "Education", nil, t0); !errors.Is(err, domain.ErrStartupWrongStage) {
		t.Fatalf("expected wrong stage during draft, got %v", err)
	}
	if err := Hire(run, "cand-0-0"); !errors.Is(err, domain.ErrStartupWrongStage) {
		t.Fatalf("expected wrong stage for hire during draft, got %v", err)
	}
	if err := PickItem(run, 0); err != nil {
		t.Fatal(err)
	}
	if err := StartProject(run, "Web App", "Education", nil, t0); err != nil {
		t.Fatal(err)
	}
}

func TestServiceDraftSurvivesRefresh(t *testing.T) {
	ctx := context.Background()
	store := &fakeStore{}
	svc := &Service{store: store, now: func() time.Time { return t0 }}
	player := Player{ID: primitive.NewObjectID().Hex(), Cohort: 12, Role: "learner"}
	if _, err := svc.StartRun(ctx, player, domain.StartupModeFree); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.PickFounder(ctx, player, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.StartProject(ctx, player, "Web App", "Education", nil); err != nil {
		t.Fatal(err)
	}
	store.run.Project.EndsAt = t0.Add(-time.Second)
	shipped, err := svc.Ship(ctx, player)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := store.FindActiveRun(ctx, primitive.NilObjectID)
	b, _ := store.FindActiveRun(ctx, primitive.NilObjectID)
	if !reflect.DeepEqual(a.ItemOffer, b.ItemOffer) || !reflect.DeepEqual(shipped.ItemOffer, a.ItemOffer) {
		t.Fatal("refresh rerolled the item offer")
	}
	if a.Stage == domain.StartupStageEvent {
		if _, err := svc.PickEvent(ctx, player, 0); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := svc.PickItem(ctx, player, 0); err != nil {
		t.Fatal(err)
	}
}

func evalWithItems(t *testing.T, seed uint64, items []string) *domain.StartupResult {
	t.Helper()
	run := NewRun(primitive.NewObjectID(), 12, "learner", domain.StartupModeFree, seed, t0)
	run.Market = domain.StartupMarket{}
	run.Staff = []domain.StartupDev{{ID: "d", Frontend: 3, Backend: 3, Design: 3, Debug: 3}}
	run.Stage = domain.StartupStageHub
	run.Items = items
	if err := StartProject(run, "Web App", "Education", []string{"d"}, t0); err != nil {
		t.Fatal(err)
	}
	run.Project.EndsAt = t0
	res, err := evaluate(run)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestItemsStack(t *testing.T) {
	plain := evalWithItems(t, 40, nil)
	one := evalWithItems(t, 40, []string{"keyboard"})
	two := evalWithItems(t, 40, []string{"keyboard", "keyboard"})
	if !(two.Total > one.Total && one.Total > plain.Total) {
		t.Fatalf("keyboards should stack: plain %d one %d two %d", plain.Total, one.Total, two.Total)
	}
}

func TestLegacyCodebaseUpsideAndDownside(t *testing.T) {
	fx := effectsOf([]string{"legacy-codebase"})
	if fx.powerMult != 1.3 || fx.bugs != 6 {
		t.Fatalf("legacy should be x1.3 power +6 bugs, got %+v", fx)
	}
	for seed := uint64(41); seed <= 45; seed++ {
		plain := evalWithItems(t, seed, nil)
		cursed := evalWithItems(t, seed, []string{"legacy-codebase"})
		if cursed.Bugs != plain.Bugs+6 {
			t.Fatalf("seed %d: legacy should add 6 bugs: plain %d cursed %d", seed, plain.Bugs, cursed.Bugs)
		}
	}
}

func TestCrunchCultureUpsideAndDownside(t *testing.T) {
	run := NewRun(primitive.NewObjectID(), 12, "learner", domain.StartupModeFree, 42, t0)
	run.Market = domain.StartupMarket{}
	run.Staff = []domain.StartupDev{{ID: "d", Frontend: 3, Backend: 3, Design: 3, Debug: 3}}
	run.Stage = domain.StartupStageHub
	run.Items = []string{"crunch-culture"}
	if err := StartProject(run, "Web App", "Education", []string{"d"}, t0); err != nil {
		t.Fatal(err)
	}
	got := int(run.Project.EndsAt.Sub(run.Project.StartedAt).Seconds())
	if got != 18 {
		t.Fatalf("expected 18s crunch project, got %d", got)
	}
	plain := evalWithItems(t, 42, nil)
	crunched := evalWithItems(t, 42, []string{"crunch-culture"})
	if !(crunched.Total < plain.Total) {
		t.Fatalf("crunch should cost stats: plain %d crunched %d", plain.Total, crunched.Total)
	}
}

func TestRubberDuckAndPitchDeck(t *testing.T) {
	plain := evalWithItems(t, 43, nil)
	duck := evalWithItems(t, 43, []string{"rubber-duck"})
	if want := max(0, plain.Bugs-2); duck.Bugs != want {
		t.Fatalf("duck should remove 2 bugs: plain %d duck %d want %d", plain.Bugs, duck.Bugs, want)
	}
	deck := evalWithItems(t, 43, []string{"pitch-deck"})
	if deck.Total <= plain.Total {
		t.Fatalf("pitch deck should help investor: plain %d deck %d", plain.Total, deck.Total)
	}
}
