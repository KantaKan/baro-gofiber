package startupstory

import (
	"errors"
	"reflect"
	"testing"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func levelRun(staff ...domain.StartupDev) *domain.StartupRun {
	return &domain.StartupRun{OwnerID: primitive.NewObjectID(), Seed: 11, Act: 1, Status: domain.StartupStatusActive, Stage: domain.StartupStageItem, Staff: staff}
}

func TestShippingGivesXPAndLevelsUp(t *testing.T) {
	dev := domain.StartupDev{ID: "cand-1", Role: RoleBE, Frontend: 2, Backend: 3, Design: 2, Debug: 2, Salary: 1000}
	run := levelRun(dev, domain.StartupDev{ID: "cand-2", Role: RoleFE})
	result := &domain.StartupResult{Total: 40}
	for levelOf(run.Staff[0]) < 2 {
		afterShipLevels(run, []domain.StartupDev{run.Staff[0]}, result)
	}
	got := run.Staff[0]
	if got.Backend < 4 || got.Salary != 1100 || got.XPNext != xpToNext(2) {
		t.Fatalf("level-up should grow the focus stat, raise salary 10%% and set the next XP goal: %+v", got)
	}
	if run.Staff[1].XP != 0 || run.Staff[1].Level != 0 {
		t.Fatal("people who sat the project out get no XP")
	}
}

func TestGrowthItemsBoostLeveling(t *testing.T) {
	base := levelRun(domain.StartupDev{ID: "cand-1", Role: RoleQA})
	afterShipLevels(base, base.Staff, &domain.StartupResult{Total: 20})
	mentored := levelRun(domain.StartupDev{ID: "cand-1", Role: RoleQA})
	mentored.Items = []string{"senior-mentor"}
	afterShipLevels(mentored, mentored.Staff, &domain.StartupResult{Total: 20})
	if mentored.Staff[0].XP <= base.Staff[0].XP {
		t.Fatalf("Senior Mentor should give more XP: %d vs %d", mentored.Staff[0].XP, base.Staff[0].XP)
	}
	plain, studied := domain.StartupDev{ID: "a", Role: RoleQA}, domain.StartupDev{ID: "a", Role: RoleQA}
	r1, r2 := rngFor(levelRun()), rngFor(levelRun())
	levelUp(&plain, r1, 1)
	levelUp(&studied, r2, 2)
	if studied.Debug != plain.Debug+1 {
		t.Fatalf("Udemy Course level-ups should add an extra focus point: %d vs %d", studied.Debug, plain.Debug)
	}
}

func TestPerkPickPausesAtLevelThreeAndResumes(t *testing.T) {
	run := levelRun(domain.StartupDev{ID: "cand-1", Role: RoleFE, Level: 2, XP: xpToNext(2) - 1})
	afterShipLevels(run, run.Staff, &domain.StartupResult{Total: 20})
	queuePerk(run)
	if run.Stage != domain.StartupStagePerk || run.PendingPerk == nil || len(run.PendingPerk.Offer) != PerkOffers {
		t.Fatalf("reaching Lv 3 should pause for a perk pick: %+v", run)
	}
	offer := append([]string{}, run.PendingPerk.Offer...)
	if err := PickPerk(run, PerkOffers); !errors.Is(err, domain.ErrStartupInvalidChoice) {
		t.Fatalf("out-of-range perk should be rejected, got %v", err)
	}
	if !reflect.DeepEqual(offer, run.PendingPerk.Offer) {
		t.Fatal("the stored offer must not change between requests")
	}
	if err := PickPerk(run, 1); err != nil {
		t.Fatal(err)
	}
	if run.Stage != domain.StartupStageItem || run.PendingPerk != nil || len(run.Staff[0].Perks) != 1 || run.Staff[0].Perks[0] != offer[1] {
		t.Fatalf("picking should store the perk and resume the item draft: %+v", run)
	}
}

func TestTwoLevelUpsQueueTwoPerkPicks(t *testing.T) {
	run := levelRun(
		domain.StartupDev{ID: "cand-1", Role: RoleFE, Level: 2, XP: xpToNext(2) - 1},
		domain.StartupDev{ID: "cand-2", Role: RoleBE, Level: 4, XP: xpToNext(4) - 1},
	)
	afterShipLevels(run, run.Staff, &domain.StartupResult{Total: 20})
	queuePerk(run)
	first := run.PendingPerk.DevID
	_ = PickPerk(run, 0)
	if run.Stage != domain.StartupStagePerk || run.PendingPerk == nil || run.PendingPerk.DevID == first {
		t.Fatalf("the second person should get their perk pick next: %+v", run)
	}
	_ = PickPerk(run, 0)
	if run.Stage != domain.StartupStageItem {
		t.Fatalf("after all picks the run should resume, got %s", run.Stage)
	}
}

func TestPerksChangeScoringAndBuildTime(t *testing.T) {
	coder := domain.StartupDev{ID: "a", Perks: []string{"rubber-duck-whisperer", "demo-whisperer", "ship-it"}}
	sc := &scoring{run: &domain.StartupRun{}, team: []domain.StartupDev{coder}, powerMult: 1, reviewer: map[string]float64{}}
	applyPerks(sc)
	if sc.bugs != -2 || sc.reviewer["Investor"] != 1 {
		t.Fatalf("perks should cut bugs and add reviewer bias: %+v", sc)
	}
	if perkDurationMult(nil, []domain.StartupDev{coder}) >= 1 {
		t.Fatal("Ship-It Energy should shorten builds")
	}
	if len(perkCatalog) < 12 {
		t.Fatalf("the spec asks for at least 12 perks, have %d", len(perkCatalog))
	}
}
