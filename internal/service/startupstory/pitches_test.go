package startupstory

import (
	"context"
	"reflect"
	"testing"
	"time"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func hubRun(t *testing.T, seed uint64) *domain.StartupRun {
	t.Helper()
	run := NewRun(primitive.NewObjectID(), 12, "learner", domain.StartupModeFree, seed, t0)
	if err := PickFounder(run, 0); err != nil {
		t.Fatal(err)
	}
	return run
}

func TestPitchesRolledAtHub(t *testing.T) {
	run := hubRun(t, 42)
	if len(run.Pitches) != 3 {
		t.Fatalf("expected 3 pitches at the hub, got %d", len(run.Pitches))
	}
	seen := map[string]bool{}
	for _, p := range run.Pitches {
		if _, ok := findType(p.Type); !ok {
			t.Fatalf("pitch has unknown product type %q", p.Type)
		}
		if !validTheme(p.Theme) {
			t.Fatalf("pitch has unknown theme %q", p.Theme)
		}
		if p.Title == "" {
			t.Fatal("every pitch needs a funny title")
		}
		key := ComboKey(p.Type, p.Theme)
		if seen[key] {
			t.Fatalf("duplicate pitch rolled: %s", key)
		}
		seen[key] = true
	}
}

func TestPitchesDeterministicPerSeed(t *testing.T) {
	a := hubRun(t, 7)
	b := hubRun(t, 7)
	if !reflect.DeepEqual(a.Pitches, b.Pitches) {
		t.Fatalf("same seed must roll same pitches:\n%+v\n%+v", a.Pitches, b.Pitches)
	}
	differ := false
	for seed := uint64(1); seed <= 10; seed++ {
		if !reflect.DeepEqual(hubRun(t, seed).Pitches, a.Pitches) {
			differ = true
			break
		}
	}
	if !differ {
		t.Fatal("pitches should vary across seeds")
	}
}

func TestPitchesRerollAfterShip(t *testing.T) {
	run := hubRun(t, 42)
	before := append([]domain.StartupPitch{}, run.Pitches...)
	if err := StartProject(run, before[0].Type, before[0].Theme, nil, t0); err != nil {
		t.Fatal(err)
	}
	if err := Ship(run, run.Project.EndsAt); err != nil {
		t.Fatal(err)
	}
	if reflect.DeepEqual(before, run.Pitches) {
		t.Fatal("pitches must reroll after each ship")
	}
}

func TestPitchesNeverRerollOnRefresh(t *testing.T) {
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
	first, err := svc.Overview(ctx, player)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Run.Pitches) != 3 {
		t.Fatalf("expected stored pitches on refresh, got %d", len(first.Run.Pitches))
	}
	second, err := svc.Overview(ctx, player)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first.Run.Pitches, second.Run.Pitches) {
		t.Fatalf("refresh must never reroll pitches:\n%+v\n%+v", first.Run.Pitches, second.Run.Pitches)
	}
}

func TestStartProjectByPitchIndex(t *testing.T) {
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
	run, err := svc.StartProjectPitch(ctx, player, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := store.run.Pitches[1]
	if run.Project.Type != want.Type || run.Project.Theme != want.Theme {
		t.Fatalf("pitch 1 should start %s × %s, got %s × %s", want.Type, want.Theme, run.Project.Type, run.Project.Theme)
	}
	if _, err := svc.StartProjectPitch(ctx, player, 3, nil); err == nil {
		t.Fatal("out-of-range pitch index must be rejected")
	}
	if _, err := svc.StartProjectPitch(ctx, player, -1, nil); err == nil {
		t.Fatal("negative pitch index must be rejected")
	}
}

func TestOSSProductHiddenUnlessOSSPath(t *testing.T) {
	const ossProduct = "Open-Source Library"
	for seed := uint64(1); seed <= 50; seed++ {
		run := hubRun(t, seed)
		for _, p := range run.Pitches {
			if p.Type == ossProduct {
				t.Fatalf("seed %d: OSS product leaked into pitches without the OSS path", seed)
			}
		}
	}
	found := false
	for seed := uint64(1); seed <= 50 && !found; seed++ {
		run := NewRun(primitive.NewObjectID(), 12, "learner", domain.StartupModeFree, seed, t0)
		run.OSS = true
		if err := PickFounder(run, 0); err != nil {
			t.Fatal(err)
		}
		for _, p := range run.Pitches {
			if p.Type == ossProduct {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("OSS path runs should sometimes pitch the Open-Source Library")
	}
	run := hubRun(t, 3)
	if err := StartProject(run, ossProduct, "Education", nil, t0); err == nil {
		t.Fatal("OSS product must be rejected outside the OSS path")
	}
}

func TestCatalogHas13ProductsAnd15Themes(t *testing.T) {
	if len(productTypes) != 13 {
		t.Fatalf("expected 13 products, got %d", len(productTypes))
	}
	if len(themes) != 15 {
		t.Fatalf("expected 15 themes, got %d", len(themes))
	}
}

func TestEveryPairHasComboValue(t *testing.T) {
	pairs := 0
	for _, pt := range productTypes {
		if len(pt.Great) == 0 || len(pt.Meh) == 0 {
			t.Fatalf("%s needs at least one great and one meh theme", pt.Name)
		}
		for _, th := range themes {
			combo := comboFor(pt, th)
			if _, ok := comboMultipliers[combo]; !ok {
				t.Fatalf("%s × %s has no combo value (%q)", pt.Name, th, combo)
			}
			pairs++
		}
	}
	if pairs != 13*15 {
		t.Fatalf("expected 195 pairs, checked %d", pairs)
	}
	meme := []struct {
		typ, theme, want string
	}{
		{"LINE Bot", "Street Food", "great"},
		{"LINE Bot", "Thai Culture", "great"},
		{"Web3 dApp", "Crypto", "great"},
		{"Browser Extension", "Crypto", "great"},
		{"Game", "Esports", "great"},
		{"Mobile App", "Dating", "great"},
		{"Dev Tool/CLI", "Government/Tax", "great"},
		{"VR Game", "K-pop/Idols", "great"},
		{"Data Dashboard", "Fintech", "great"},
		{"AI Chatbot", "Dating", "meh"},
		{"Web3 dApp", "Government/Tax", "meh"},
		{"Game", "Fintech", "meh"},
	}
	for _, m := range meme {
		pt, ok := findType(m.typ)
		if !ok {
			t.Fatalf("unknown product %q", m.typ)
		}
		if got := comboFor(pt, m.theme); got != m.want {
			t.Fatalf("%s × %s should be %s, got %s", m.typ, m.theme, m.want, got)
		}
	}
}

func TestOverviewSharesRatingsForDiscoveredCombos(t *testing.T) {
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
	discovered := ComboKey("Web App", "Education")
	hidden := ComboKey("Game", "Fintech")
	before, err := svc.Overview(ctx, player)
	if err != nil {
		t.Fatal(err)
	}
	if len(before.ComboRatings) != 0 {
		t.Fatalf("nothing discovered yet, got ratings %+v", before.ComboRatings)
	}
	if _, err := svc.StartProject(ctx, player, "Web App", "Education", nil); err != nil {
		t.Fatal(err)
	}
	store.run.Project.EndsAt = t0.Add(-time.Second)
	if _, err := svc.Ship(ctx, player); err != nil {
		t.Fatal(err)
	}
	after, err := svc.Overview(ctx, player)
	if err != nil {
		t.Fatal(err)
	}
	if after.ComboRatings[discovered] != "great" {
		t.Fatalf("discovered combo should be rated, got %+v", after.ComboRatings)
	}
	if _, ok := after.ComboRatings[hidden]; ok {
		t.Fatalf("undiscovered combo must not be rated: %+v", after.ComboRatings)
	}
}
