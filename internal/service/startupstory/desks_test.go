package startupstory

import (
	"errors"
	"reflect"
	"testing"

	"gofiber-baro/internal/domain"
)

func TestRunStartsWithTwoDesksAndOfficeLimitsBuying(t *testing.T) {
	run := newHubRun(t, 31)
	if !reflect.DeepEqual(run.Desks, []int{1, 1}) {
		t.Fatalf("a run should start with two tier-1 desks, got %v", run.Desks)
	}
	if err := BuyDesk(run); !errors.Is(err, domain.ErrStartupDeskLimit) {
		t.Fatalf("the garage holds only 2 desks, got %v", err)
	}

	run.Act = 2
	run.Money = 100000
	if err := MoveOffice(run, "shophouse"); err != nil {
		t.Fatal(err)
	}
	before := run.Money
	if err := BuyDesk(run); err != nil {
		t.Fatal(err)
	}
	if err := BuyDesk(run); err != nil {
		t.Fatal(err)
	}
	if spent := before - run.Money; spent != DeskBasePrice+DeskBasePrice+DeskStepPrice {
		t.Fatalf("3rd desk 1500 + 4th desk 2000 should cost 3500, spent %d", spent)
	}
	if err := BuyDesk(run); !errors.Is(err, domain.ErrStartupDeskLimit) {
		t.Fatalf("the shophouse holds only 4 desks, got %v", err)
	}
}

func TestHiringNeedsAFreeDesk(t *testing.T) {
	run := newHubRun(t, 32)
	run.Act = 2
	if err := MoveOffice(run, "shophouse"); err != nil {
		t.Fatal(err)
	}
	if err := Hire(run, run.Candidates[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := Hire(run, run.Candidates[0].ID); !errors.Is(err, domain.ErrStartupTeamFull) {
		t.Fatalf("two desks should mean two people, got %v", err)
	}
	if err := BuyDesk(run); err != nil {
		t.Fatal(err)
	}
	if err := Hire(run, run.Candidates[0].ID); err != nil {
		t.Fatalf("a bought desk should allow a hire: %v", err)
	}
}

func TestUpgradeDeskTiers(t *testing.T) {
	run := newHubRun(t, 33)
	run.Money = 100000
	before := run.Money
	for range 2 {
		if err := UpgradeDesk(run, 1); err != nil {
			t.Fatalf("act 1 upgrades should work: %v", err)
		}
	}
	if run.Desks[1] != 3 || before-run.Money != deskUpgradePrices[2]+deskUpgradePrices[3] {
		t.Fatalf("two upgrades should reach tier 3: desks %v spent %d", run.Desks, before-run.Money)
	}
	if err := UpgradeDesk(run, 1); !errors.Is(err, domain.ErrStartupDeskLimit) {
		t.Fatalf("tier 4 opens in act 3, got %v", err)
	}
	run.Act = 3
	if err := UpgradeDesk(run, 1); err != nil {
		t.Fatal(err)
	}
	if err := UpgradeDesk(run, 1); !errors.Is(err, domain.ErrStartupDeskLimit) {
		t.Fatalf("tier 5 opens in act 4, got %v", err)
	}
	run.Act = 4
	if err := UpgradeDesk(run, 1); err != nil {
		t.Fatal(err)
	}
	if err := UpgradeDesk(run, 1); !errors.Is(err, domain.ErrStartupInvalidChoice) {
		t.Fatalf("tier 5 is the max, got %v", err)
	}
	if err := UpgradeDesk(run, 5); !errors.Is(err, domain.ErrStartupInvalidChoice) {
		t.Fatalf("missing desk should be invalid, got %v", err)
	}
	run.Money = DeskUpgradePrice(1) - 1
	if err := UpgradeDesk(run, 0); !errors.Is(err, domain.ErrStartupNoFunds) {
		t.Fatalf("expected no funds, got %v", err)
	}
	run.Stage = domain.StartupStageDeveloping
	if err := BuyDesk(run); !errors.Is(err, domain.ErrStartupWrongStage) {
		t.Fatalf("desks are bought in the hub only, got %v", err)
	}
}

func TestDeskTierCapByAct(t *testing.T) {
	for act, want := range map[int]int{1: 3, 2: 3, 3: 4, 4: 5, 9: 5} {
		if got := deskTierCap(act); got != want {
			t.Errorf("act %d: tier cap %d, want %d", act, got, want)
		}
	}
}

func TestOldRunsGetTierOneDesksUpToTheActCap(t *testing.T) {
	run := &domain.StartupRun{Act: 3}
	prepareRun(run)
	if !reflect.DeepEqual(run.Desks, []int{1, 1, 1, 1, 1, 1}) || run.DeskLimit != 6 {
		t.Fatalf("an act-3 run saved before desks should get 6 tier-1 desks, got %v limit %d", run.Desks, run.DeskLimit)
	}
}

func TestBestDesksGoToStaffInHireOrder(t *testing.T) {
	run := &domain.StartupRun{Desks: []int{1, 3, 2}, Staff: []domain.StartupDev{{ID: "a"}, {ID: "b"}, {ID: "c"}}}
	if got := deskBonus(run); !reflect.DeepEqual(got, map[string]int{"a": 2, "b": 1, "c": 0}) {
		t.Fatalf("unexpected desk bonus %v", got)
	}
}

func TestDeskTierRaisesReviewScores(t *testing.T) {
	total := func(tier int) int {
		run := newHubRun(t, 34)
		run.Act = 2
		run.Desks = []int{tier, tier}
		if err := StartProject(run, "Web App", "Education", nil, t0); err != nil {
			t.Fatal(err)
		}
		res, err := evaluate(run)
		if err != nil {
			t.Fatal(err)
		}
		return res.Total
	}
	if plain, pro := total(1), total(3); pro <= plain {
		t.Fatalf("tier-3 desks should beat tier-1 on the same seed: %d vs %d", pro, plain)
	}
}
