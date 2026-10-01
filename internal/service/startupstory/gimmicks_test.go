package startupstory

import (
	"testing"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestGimmickPoolResolvesInCatalog(t *testing.T) {
	if len(gimmickPool) < 8 {
		t.Fatalf("boss gimmick pool needs at least 8 entries, got %d", len(gimmickPool))
	}
	for _, id := range gimmickPool {
		g, ok := findGimmick(id)
		if !ok {
			t.Fatalf("gimmick %q not in catalog", id)
		}
		if g.Name == "" || g.Desc == "" {
			t.Fatalf("gimmick %q needs a name and desc for the boss intro card", id)
		}
	}
}

func TestStartProjectRollsBossGimmickDeterministically(t *testing.T) {
	roll := func() string {
		run := NewRun(primitive.NewObjectID(), 12, "learner", domain.StartupModeFree, 42, t0)
		if err := PickFounder(run, 0); err != nil {
			t.Fatal(err)
		}
		run.ProjectIndex = 2
		if err := StartProject(run, "Web App", "Education", nil, t0); err != nil {
			t.Fatal(err)
		}
		return run.BossGimmick
	}
	a, b := roll(), roll()
	if a == "" || a != b {
		t.Fatalf("same seed must give the same boss gimmick: %q %q", a, b)
	}
	inPool := false
	for _, id := range gimmickPool {
		if id == a {
			inPool = true
		}
	}
	if !inPool {
		t.Fatalf("gimmick %q rolled outside the pool", a)
	}
}

func TestBossGimmickClearedOnNonBossProject(t *testing.T) {
	run := NewRun(primitive.NewObjectID(), 12, "learner", domain.StartupModeFree, 7, t0)
	if err := PickFounder(run, 0); err != nil {
		t.Fatal(err)
	}
	run.BossGimmick = "wifi-down"
	if err := StartProject(run, "Web App", "Education", nil, t0); err != nil {
		t.Fatal(err)
	}
	if run.BossGimmick != "" {
		t.Fatalf("a non-boss project must clear the stale gimmick, got %q", run.BossGimmick)
	}
}

func TestApplyGimmicksBendsScoring(t *testing.T) {
	newSc := func(run *domain.StartupRun) *scoring {
		return &scoring{
			run:       run,
			weights:   [4]float64{1, 1, 1, 1},
			powerMult: 1,
			reviewer:  map[string]float64{},
			moneyMult: 1,
			fansMult:  1,
		}
	}

	wifi := newSc(&domain.StartupRun{BossGimmick: "wifi-down"})
	applyGimmicks(wifi)
	if wifi.weights != [4]float64{0.4, 0, 0.4, 0.2} {
		t.Fatalf("wifi-down should replace the weight vector, got %v", wifi.weights)
	}

	js := newSc(&domain.StartupRun{BossGimmick: "hates-js"})
	applyGimmicks(js)
	if js.weights != [4]float64{0.5, 1, 1, 1} {
		t.Fatalf("hates-js should halve frontend only, got %v", js.weights)
	}

	flaky := newSc(&domain.StartupRun{BossGimmick: "flaky-ci"})
	applyGimmicks(flaky)
	if flaky.bugs != 2 || flaky.powerMult != 1 {
		t.Fatalf("flaky-ci should add 2 bugs and leave power alone: %+v", flaky)
	}

	readme := newSc(&domain.StartupRun{BossGimmick: "readme-only"})
	applyGimmicks(readme)
	if readme.reviewer["Dev Community"] != 1.5 || readme.reviewer["Investor"] != -1 {
		t.Fatalf("readme-only should bend the reviewers, got %v", readme.reviewer)
	}

	coffee := newSc(&domain.StartupRun{BossGimmick: "coffee-budget"})
	applyGimmicks(coffee)
	if coffee.powerMult != 1.12 {
		t.Fatalf("coffee-budget should add 12%% power, got %v", coffee.powerMult)
	}

	combo := newSc(&domain.StartupRun{BossGimmick: "flaky-ci", WorldEvent: "hackathon"})
	applyGimmicks(combo)
	if combo.bugs != 4 {
		t.Fatalf("gimmick and world event bugs should stack, got %d", combo.bugs)
	}

	none := newSc(&domain.StartupRun{})
	applyGimmicks(none)
	if none.weights != [4]float64{1, 1, 1, 1} || none.bugs != 0 || none.powerMult != 1 || none.moneyMult != 1 || none.fansMult != 1 {
		t.Fatalf("no gimmick and no world event must leave scoring untouched: %+v", none)
	}
}
