package startupstory

import (
	"context"
	"strings"
	"testing"
	"time"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestCompanyBossFailIsASetback(t *testing.T) {
	run := hubRunAt(t, 62, 2, []domain.StartupDev{mkDev("d", 2, 2, 2, 2)})
	run.BossOrder = []string{BossDemoDay, BossOutage}
	run.Fans = 1000
	shipNow(t, run, "Game", "Fintech")
	if run.Status != domain.StartupStatusActive || run.BossesPassed != 0 {
		t.Fatalf("a company survives a failed boss: status %s", run.Status)
	}
	if run.ProjectIndex != 0 || run.Act != 1 || !strings.Contains(strings.Join(run.Log, "|"), "demo flopped") {
		t.Fatalf("setback should replay the act's two projects before a rematch: index %d act %d", run.ProjectIndex, run.Act)
	}
	if run.Staff[0].Burnout < SetbackBurnout {
		t.Fatalf("setback should add burnout, got %d", run.Staff[0].Burnout)
	}
}

func TestCompanyBankruptcyTakesALoan(t *testing.T) {
	dev := mkDev("d", 1, 1, 1, 1)
	dev.Salary = 5000
	run := hubRunAt(t, 67, 0, []domain.StartupDev{dev})
	run.Money = 100
	shipNow(t, run, "Game", "Fintech")
	if run.Status != domain.StartupStatusActive || run.Money != LoanCash || run.Debt != LoanDebt {
		t.Fatalf("broke company should take a loan: money %d debt %d status %s", run.Money, run.Debt, run.Status)
	}
	run.Stage = domain.StartupStageHub
	run.Act = 2
	if err := MoveOffice(run, "shophouse"); err != domain.ErrStartupInDebt {
		t.Fatalf("no office moves while in debt, got %v", err)
	}
	if err := UpgradeDesk(run, 0); err != domain.ErrStartupInDebt {
		t.Fatalf("no desk upgrades while in debt, got %v", err)
	}
	repayDebt(run, 4000)
	if run.Debt != LoanDebt-1000 {
		t.Fatalf("25%% of earnings repays the loan, debt %d", run.Debt)
	}
	repayDebt(run, 1000000)
	if run.Debt != 0 {
		t.Fatalf("big earnings clear the loan, debt %d", run.Debt)
	}
}

func TestCompanyFounderTakesABreak(t *testing.T) {
	founder := person("founder-0", RolePM, 99)
	run := burnoutRun(founder, person("cand-1", RoleFE, 0))
	if afterShipBurnout(run, []domain.StartupDev{founder}, t0) {
		t.Fatal("a company founder should not end the run")
	}
	if run.Status == domain.StartupStatusEnded || run.Staff[0].Burnout != FounderBreakReset {
		t.Fatalf("founder should rest, not quit: %+v", run.Staff[0])
	}
}

func TestCompanyEarnsFameFromMilestones(t *testing.T) {
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
	for ships := 0; ships < 20 && store.run.Status == domain.StartupStatusActive; {
		switch store.run.Stage {
		case domain.StartupStageHub:
			if _, err := svc.StartProject(ctx, player, "Web App", "Education", nil); err != nil {
				t.Fatal(err)
			}
			store.run.Project.EndsAt = t0.Add(-time.Second)
			if _, err := svc.Ship(ctx, player); err != nil {
				t.Fatal(err)
			}
			ships++
		case domain.StartupStageItem:
			_, _ = svc.PickItem(ctx, player, 0)
		case domain.StartupStagePerk:
			_, _ = svc.PickPerk(ctx, player, 0)
		case domain.StartupStageEvent:
			_, _ = svc.PickEvent(ctx, player, 0)
		case domain.StartupStageIPOChoice:
			_, _ = svc.IPOChoice(ctx, player, true)
		default:
			t.Fatalf("unexpected stage %s", store.run.Stage)
		}
	}
	if store.run.Status != domain.StartupStatusActive {
		t.Fatalf("a company only ends by choice, got %s", store.run.Outcome)
	}
	if store.run.MaxAct > 1 && (store.studio == nil || store.studio.Fame < FameNewStage) {
		t.Fatalf("reaching a new stage should grant fame: max act %d, studio %+v", store.run.MaxAct, store.studio)
	}
	if store.run.PendingFame != 0 {
		t.Fatalf("pending fame should be paid out, got %d", store.run.PendingFame)
	}
}

func TestCompaniesSurviveLongRuns(t *testing.T) {
	botMode, botMaxAct = domain.StartupModeFree, 6
	defer func() { botMode, botMaxAct = domain.StartupModeRanked, 60 }()
	active, inDebt, reached := 0, 0, 0
	for seed := uint64(1); seed <= 100; seed++ {
		playBotInfra(t, seed, true, true)
		if botLast.Status == domain.StartupStatusActive {
			active++
		}
		if botLast.Debt > 0 {
			inDebt++
		}
		if botLast.MaxAct >= 6 {
			reached++
		}
	}
	t.Logf("companies: %d/100 still running, %d reached act 6, %d in debt at the end", active, reached, inDebt)
	if active < 95 {
		t.Errorf("companies should only end by choice, %d/100 running", active)
	}
	if inDebt > 30 {
		t.Errorf("debt should be recoverable, %d/100 still in debt", inDebt)
	}
}
