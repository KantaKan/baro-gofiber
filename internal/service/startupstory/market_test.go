package startupstory

import (
	"context"
	"reflect"
	"testing"
	"time"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func validThemeName(name string) bool {
	return validTheme(name)
}

func TestMarketRolledAtCreation(t *testing.T) {
	run := NewRun(primitive.NewObjectID(), 12, "learner", domain.StartupModeFree, 5, t0)
	if len(run.Market.Hot) != MarketHotCount || len(run.Market.Cold) != 1 {
		t.Fatalf("expected 2 hot + 1 cold, got %+v", run.Market)
	}
	seen := map[string]bool{}
	for _, th := range append(append([]string{}, run.Market.Hot...), run.Market.Cold...) {
		if !validThemeName(th) || seen[th] {
			t.Fatalf("bad market themes: %+v", run.Market)
		}
		seen[th] = true
	}
}

func TestMarketDeterministicPerSeed(t *testing.T) {
	a := NewRun(primitive.NewObjectID(), 12, "learner", domain.StartupModeFree, 77, t0)
	b := NewRun(primitive.NewObjectID(), 12, "learner", domain.StartupModeFree, 77, t0)
	if !reflect.DeepEqual(a.Market, b.Market) {
		t.Fatalf("same seed gave different markets: %+v vs %+v", a.Market, b.Market)
	}
}

func TestMarketMult(t *testing.T) {
	m := domain.StartupMarket{Hot: []string{"Education"}, Cold: []string{"Travel"}}
	if marketMult(m, "Education") != MarketHotMult {
		t.Fatal("hot should be x1.5")
	}
	if marketMult(m, "Travel") != MarketColdMult {
		t.Fatal("cold should be x0.6")
	}
	if marketMult(m, "Health") != 1.0 {
		t.Fatal("neutral should be x1.0")
	}
	if marketMult(domain.StartupMarket{}, "Health") != 1.0 {
		t.Fatal("empty market should be neutral")
	}
}

func shipWithMarket(t *testing.T, seed uint64, market domain.StartupMarket, theme string) *domain.StartupRun {
	t.Helper()
	run := NewRun(primitive.NewObjectID(), 12, "learner", domain.StartupModeFree, seed, t0)
	run.Market = market
	if err := PickFounder(run, 0); err != nil {
		t.Fatal(err)
	}
	if err := StartProject(run, "Game", theme, nil, t0); err != nil {
		t.Fatal(err)
	}
	if err := Ship(run, run.Project.EndsAt); err != nil {
		t.Fatal(err)
	}
	return run
}

func TestHotBeatsCold(t *testing.T) {
	market := domain.StartupMarket{Hot: []string{"Education"}, Cold: []string{"Travel"}}
	for seed := uint64(1); seed <= 20; seed++ {
		hot := shipWithMarket(t, seed, market, "Education")
		cold := shipWithMarket(t, seed, market, "Travel")
		if hot.LastResult.Combo != "good" || cold.LastResult.Combo != "good" {
			t.Fatal("test themes should both be good combos for Game")
		}
		if hot.LastResult.Total <= cold.LastResult.Total {
			t.Fatalf("seed %d: hot %d should beat cold %d", seed, hot.LastResult.Total, cold.LastResult.Total)
		}
	}
}

func TestComboOrderingNeutralMarket(t *testing.T) {
	for seed := uint64(1); seed <= 20; seed++ {
		neutral := domain.StartupMarket{}
		great := shipWithMarket(t, seed, neutral, "Thai Culture")
		good := shipWithMarket(t, seed, neutral, "Education")
		meh := shipWithMarket(t, seed, neutral, "Fintech")
		if great.LastResult.Combo != "great" || good.LastResult.Combo != "good" || meh.LastResult.Combo != "meh" {
			t.Fatal("combo table changed")
		}
		if !(great.LastResult.Total > good.LastResult.Total && good.LastResult.Total > meh.LastResult.Total) {
			t.Fatalf("seed %d: expected great %d > good %d > meh %d",
				seed, great.LastResult.Total, good.LastResult.Total, meh.LastResult.Total)
		}
	}
}

func TestShipRecordsDiscoveredCombo(t *testing.T) {
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
	theme := "Education"
	if _, err := svc.StartProject(ctx, player, "Web App", theme, nil); err != nil {
		t.Fatal(err)
	}
	store.run.Project.EndsAt = t0.Add(-time.Second)
	shipped, err := svc.Ship(ctx, player)
	if err != nil {
		t.Fatal(err)
	}
	want := ComboKey("Web App", theme)
	if shipped.LastResult == nil {
		t.Fatal("expected last_result after ship")
	}
	found := false
	for _, k := range store.studio.DiscoveredCombos {
		if k == want {
			found = true
		}
	}
	if !found {
		t.Fatalf("studio missing combo %q: %+v", want, store.studio.DiscoveredCombos)
	}
	before := len(store.studio.DiscoveredCombos)
	if _, err := svc.PickItem(ctx, player, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.StartProject(ctx, player, "Web App", theme, nil); err != nil {
		t.Fatal(err)
	}
	store.run.Project.EndsAt = t0.Add(-time.Second)
	if _, err := svc.Ship(ctx, player); err != nil {
		t.Fatal(err)
	}
	if len(store.studio.DiscoveredCombos) != before {
		t.Fatalf("duplicate combo recorded: %+v", store.studio.DiscoveredCombos)
	}
}
