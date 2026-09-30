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

func newHubRun(t *testing.T, seed uint64) *domain.StartupRun {
	t.Helper()
	run := NewRun(primitive.NewObjectID(), 12, "learner", domain.StartupModeFree, seed, t0)
	if err := PickFounder(run, 0); err != nil {
		t.Fatal(err)
	}
	return run
}

func TestPickFounderDealsStoredCandidates(t *testing.T) {
	run := newHubRun(t, 11)
	if run.Act != 1 {
		t.Fatalf("expected act 1, got %d", run.Act)
	}
	if len(run.Candidates) != CandidateOffers {
		t.Fatalf("expected %d candidates, got %d", CandidateOffers, len(run.Candidates))
	}
	seen := map[string]bool{}
	for _, c := range run.Candidates {
		if c.ID == "" || seen[c.ID] {
			t.Fatalf("candidates need unique IDs: %+v", run.Candidates)
		}
		seen[c.ID] = true
		if c.Salary != salaryFor(c.Frontend, c.Backend, c.Design, c.Debug, c.Trait) {
			t.Fatalf("salary not derived from stats: %+v", c)
		}
	}
}

func TestStatBoundsScaleByAct(t *testing.T) {
	if statMax(1) != 3 || statMax(2) != 5 || statMax(3) != 7 {
		t.Fatalf("bad statMax: %d %d %d", statMax(1), statMax(2), statMax(3))
	}
	for act := 1; act <= 3; act++ {
		run := NewRun(primitive.NewObjectID(), 12, "learner", domain.StartupModeFree, 99, t0)
		run.Act = act
		for i := 0; i < 20; i++ {
			for _, c := range rollCandidates(run, CandidateOffers) {
				for _, s := range [4]int{c.Frontend, c.Backend, c.Design, c.Debug} {
					if s < 1 || s > statMax(act) {
						t.Fatalf("act %d stat out of bounds: %+v", act, c)
					}
				}
			}
		}
	}
}

func TestHireCostsSalaryAndRespectsCap(t *testing.T) {
	run := newHubRun(t, 21)
	before := run.Money
	cand := run.Candidates[0]
	if err := Hire(run, cand.ID); err != nil {
		t.Fatal(err)
	}
	if run.Money != before-cand.Salary {
		t.Fatalf("hire should cost one salary: before %d after %d salary %d", before, run.Money, cand.Salary)
	}
	if len(run.Staff) != 2 || len(run.Candidates) != CandidateOffers-1 {
		t.Fatalf("unexpected team sizes: staff %d candidates %d", len(run.Staff), len(run.Candidates))
	}
	if err := Hire(run, run.Candidates[0].ID); !errors.Is(err, domain.ErrStartupTeamFull) {
		t.Fatalf("expected team full at cap %d, got %v", TeamCap(run.Act), err)
	}
}

func TestHireRejectsNoFundsAndUnknown(t *testing.T) {
	run := newHubRun(t, 22)
	run.Money = 0
	if err := Hire(run, run.Candidates[0].ID); !errors.Is(err, domain.ErrStartupNoFunds) {
		t.Fatalf("expected no funds, got %v", err)
	}
	run.Money = StartingMoney
	if err := Hire(run, "cand-missing"); !errors.Is(err, domain.ErrStartupInvalidChoice) {
		t.Fatalf("expected invalid choice, got %v", err)
	}
}

func TestDismissIsFreeAndKeepsOne(t *testing.T) {
	run := newHubRun(t, 23)
	if err := Dismiss(run, run.Staff[0].ID); !errors.Is(err, domain.ErrStartupInvalidChoice) {
		t.Fatalf("expected last-member guard, got %v", err)
	}
	cand := run.Candidates[0]
	if err := Hire(run, cand.ID); err != nil {
		t.Fatal(err)
	}
	afterHire := run.Money
	founderID := run.Staff[0].ID
	if err := Dismiss(run, founderID); err != nil {
		t.Fatal(err)
	}
	if run.Money != afterHire {
		t.Fatalf("dismiss should be free: %d vs %d", run.Money, afterHire)
	}
	if len(run.Staff) != 1 {
		t.Fatalf("expected 1 staff left, got %d", len(run.Staff))
	}
	if err := Dismiss(run, "nobody"); !errors.Is(err, domain.ErrStartupInvalidChoice) {
		t.Fatalf("expected invalid choice, got %v", err)
	}
}

func TestHireDismissWrongStage(t *testing.T) {
	run := NewRun(primitive.NewObjectID(), 12, "learner", domain.StartupModeFree, 24, t0)
	if err := Hire(run, "cand-0-0"); !errors.Is(err, domain.ErrStartupWrongStage) {
		t.Fatalf("expected wrong stage (founder), got %v", err)
	}
	if err := Dismiss(run, "founder-0"); !errors.Is(err, domain.ErrStartupWrongStage) {
		t.Fatalf("expected wrong stage (founder), got %v", err)
	}
	hub := newHubRun(t, 25)
	if err := StartProject(hub, "Web App", "Education", nil, t0); err != nil {
		t.Fatal(err)
	}
	if err := Hire(hub, "cand-0-0"); !errors.Is(err, domain.ErrStartupWrongStage) {
		t.Fatalf("expected wrong stage (developing), got %v", err)
	}
	if err := Dismiss(hub, hub.Staff[0].ID); !errors.Is(err, domain.ErrStartupWrongStage) {
		t.Fatalf("expected wrong stage (developing), got %v", err)
	}
}

func TestCandidatesRerollAfterShip(t *testing.T) {
	run := newHubRun(t, 26)
	first := append([]domain.StartupDev{}, run.Candidates...)
	if err := StartProject(run, "Web App", "Education", nil, t0); err != nil {
		t.Fatal(err)
	}
	if err := Ship(run, run.Project.EndsAt); err != nil {
		t.Fatal(err)
	}
	if run.Stage != domain.StartupStageItem {
		t.Fatalf("expected item draft after ship, got %s", run.Stage)
	}
	if len(run.Candidates) != CandidateOffers {
		t.Fatalf("expected fresh candidates, got %d", len(run.Candidates))
	}
	if reflect.DeepEqual(first, run.Candidates) {
		t.Fatal("candidates should reroll after ship")
	}
	if err := PickItem(run, 0); err != nil {
		t.Fatal(err)
	}
	if run.Stage != domain.StartupStageHub {
		t.Fatalf("expected hub after draft, got %s", run.Stage)
	}
}

func TestServiceCandidatesSurviveRefresh(t *testing.T) {
	ctx := context.Background()
	store := &fakeStore{}
	svc := &Service{store: store, now: func() time.Time { return t0 }}
	player := Player{ID: primitive.NewObjectID().Hex(), Cohort: 12, Role: "learner"}
	if _, err := svc.StartRun(ctx, player, domain.StartupModeFree); err != nil {
		t.Fatal(err)
	}
	picked, err := svc.PickFounder(ctx, player, 0)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := store.FindActiveRun(ctx, primitive.NilObjectID)
	b, _ := store.FindActiveRun(ctx, primitive.NilObjectID)
	if !reflect.DeepEqual(a.Candidates, b.Candidates) || !reflect.DeepEqual(picked.Candidates, a.Candidates) {
		t.Fatal("refresh rerolled candidates")
	}
	hired, err := svc.Hire(ctx, player, a.Candidates[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(hired.Staff) != 2 {
		t.Fatalf("expected hire via service, got %d staff", len(hired.Staff))
	}
	dismissed, err := svc.Dismiss(ctx, player, hired.Staff[1].ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(dismissed.Staff) != 1 {
		t.Fatalf("expected dismiss via service, got %d staff", len(dismissed.Staff))
	}
}

func TestNightOwlShortensDuration(t *testing.T) {
	run := newHubRun(t, 27)
	owl := domain.StartupDev{ID: "owl-1", Name: "Owl", Title: "Debugger", Frontend: 2, Backend: 2, Design: 2, Debug: 4, Trait: TraitNightOwl}
	owl.Salary = salaryFor(owl.Frontend, owl.Backend, owl.Design, owl.Debug, owl.Trait)
	run.Staff = append(run.Staff, owl)
	if err := StartProject(run, "Web App", "Education", []string{owl.ID}, t0); err != nil {
		t.Fatal(err)
	}
	got := int(run.Project.EndsAt.Sub(run.Project.StartedAt).Seconds())
	if got != 26 {
		t.Fatalf("expected 26s night-owl project, got %d", got)
	}
}

func TestTenXSalaryAndPower(t *testing.T) {
	if salaryFor(2, 2, 2, 2, TraitTenX) != 2*SalaryRate*8 {
		t.Fatal("10x salary should double")
	}
	dev := domain.StartupDev{Frontend: 2, Backend: 2, Design: 2, Debug: 2, Trait: TraitTenX}
	if got := effectiveStats(dev); got != [4]int{4, 4, 4, 4} {
		t.Fatalf("10x should add +2 everywhere, got %v", got)
	}
	base := domain.StartupDev{ID: "d", Frontend: 3, Backend: 3, Design: 3, Debug: 3}
	boosted := base
	boosted.ID = "d"
	boosted.Trait = TraitTenX
	for seed := uint64(1); seed <= 10; seed++ {
		mk := func(d domain.StartupDev) *domain.StartupRun {
			r := NewRun(primitive.NewObjectID(), 12, "learner", domain.StartupModeFree, seed, t0)
			r.Market = domain.StartupMarket{}
			r.Staff = []domain.StartupDev{d}
			r.Stage = domain.StartupStageHub
			if err := StartProject(r, "API/SaaS", "Education", []string{d.ID}, t0); err != nil {
				t.Fatal(err)
			}
			return r
		}
		plain, plus := mk(base), mk(boosted)
		plain.Project.EndsAt = t0
		plus.Project.EndsAt = t0
		pr, err := evaluate(plain)
		if err != nil {
			t.Fatal(err)
		}
		tr, err := evaluate(plus)
		if err != nil {
			t.Fatal(err)
		}
		if tr.Total <= pr.Total {
			t.Fatalf("seed %d: 10x %d should beat plain %d", seed, tr.Total, pr.Total)
		}
	}
}
