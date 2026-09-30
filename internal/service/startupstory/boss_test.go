package startupstory

import (
	"context"
	"errors"
	"testing"
	"time"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func mkDev(id string, fe, be, design, debug int) domain.StartupDev {
	return domain.StartupDev{ID: id, Name: "T", Title: "Dev", Frontend: fe, Backend: be, Design: design, Debug: debug, Salary: salaryFor(fe, be, design, debug, "")}
}

func hubRunAt(t *testing.T, seed uint64, projectIndex int, staff []domain.StartupDev) *domain.StartupRun {
	t.Helper()
	run := NewRun(primitive.NewObjectID(), 12, "learner", domain.StartupModeFree, seed, t0)
	run.Market = domain.StartupMarket{}
	run.Staff = staff
	run.Stage = domain.StartupStageHub
	run.ProjectIndex = projectIndex
	run.Act = actFor(projectIndex)
	return run
}

func shipNow(t *testing.T, run *domain.StartupRun, typeName, theme string) {
	t.Helper()
	if err := StartProject(run, typeName, theme, nil, t0); err != nil {
		t.Fatal(err)
	}
	if err := Ship(run, run.Project.EndsAt); err != nil {
		t.Fatal(err)
	}
}

func TestBossOrderFromSeed(t *testing.T) {
	a := NewRun(primitive.NewObjectID(), 12, "learner", domain.StartupModeFree, 60, t0)
	b := NewRun(primitive.NewObjectID(), 12, "learner", domain.StartupModeFree, 60, t0)
	if len(a.BossOrder) != 2 || len(b.BossOrder) != 2 {
		t.Fatalf("expected 2 bosses, got %+v", a.BossOrder)
	}
	if a.BossOrder[0] != b.BossOrder[0] || a.BossOrder[1] != b.BossOrder[1] {
		t.Fatal("same seed should give same boss order")
	}
	if a.BossOrder[0] == a.BossOrder[1] {
		t.Fatalf("bosses should differ: %+v", a.BossOrder)
	}
	for _, boss := range a.BossOrder {
		found := false
		for _, p := range bossPool {
			if boss == p {
				found = true
			}
		}
		if !found || boss == BossIPO {
			t.Fatalf("bad boss order: %+v", a.BossOrder)
		}
	}
	if got := bossForRun(8, a.BossOrder); got != BossIPO {
		t.Fatalf("final boss should always be IPO, got %s", got)
	}
	if got := bossForRun(2, a.BossOrder); got != a.BossOrder[0] {
		t.Fatalf("act 1 boss should be order[0], got %s", got)
	}
}

func TestBossProjectLasts90s(t *testing.T) {
	run := hubRunAt(t, 61, 2, []domain.StartupDev{mkDev("d", 3, 3, 3, 3)})
	run.BossOrder = []string{BossDemoDay, BossOutage}
	if err := StartProject(run, "Web App", "Education", nil, t0); err != nil {
		t.Fatal(err)
	}
	if run.Project.Boss != BossDemoDay {
		t.Fatalf("expected demo-day boss, got %+v", run.Project)
	}
	if got := int(run.Project.EndsAt.Sub(run.Project.StartedAt).Seconds()); got != BossDuration {
		t.Fatalf("expected %ds boss project, got %d", BossDuration, got)
	}
}

func TestDemoDayFailEndsPivot(t *testing.T) {
	run := hubRunAt(t, 62, 2, []domain.StartupDev{mkDev("d", 2, 2, 2, 2)})
	run.BossOrder = []string{BossDemoDay, BossOutage}
	shipNow(t, run, "Game", "Fintech")
	if run.Status != domain.StartupStatusEnded || run.Outcome != domain.StartupOutcomePivot {
		t.Fatalf("failed boss should pivot: %+v", run.LastResult)
	}
	if run.BossesPassed != 0 {
		t.Fatalf("failed boss should not count: %+v", run)
	}
}

func TestDemoDayPassAdvancesWithoutDraft(t *testing.T) {
	staff := []domain.StartupDev{mkDev("a", 6, 6, 6, 6), mkDev("b", 6, 6, 6, 6)}
	run := hubRunAt(t, 63, 2, staff)
	run.BossOrder = []string{BossDemoDay, BossOutage}
	before := run.Money
	shipNow(t, run, "Game", "Thai Culture")
	if run.Status != domain.StartupStatusActive || run.Stage != domain.StartupStageHub {
		t.Fatalf("passed boss should return to hub: stage=%s status=%s", run.Stage, run.Status)
	}
	if run.BossesPassed != 1 || run.Act != 2 {
		t.Fatalf("expected 1 boss passed and act 2: %+v", run)
	}
	if len(run.ItemOffer) != 0 {
		t.Fatalf("boss ship should skip the item draft: %+v", run.ItemOffer)
	}
	if run.LastResult.MoneyDelta < BossBonusMoney || run.Money <= before {
		t.Fatalf("boss bonus missing: %+v", run.LastResult)
	}
}

func TestRequirementsSwapTheme(t *testing.T) {
	run := hubRunAt(t, 64, 2, []domain.StartupDev{mkDev("d", 5, 5, 5, 5)})
	run.BossOrder = []string{BossRequirements, BossOutage}
	shipNow(t, run, "Web App", "Education")
	if run.LastResult.Theme == "Education" {
		t.Fatalf("requirements boss should swap the theme: %+v", run.LastResult)
	}
}

func TestOutageFavorsDebug(t *testing.T) {
	debugTeam := []domain.StartupDev{mkDev("a", 1, 1, 1, 9)}
	balancedTeam := []domain.StartupDev{mkDev("b", 4, 4, 4, 1)}
	normal := func(team []domain.StartupDev) int {
		run := hubRunAt(t, 65, 0, team)
		shipNow(t, run, "Game", "Education")
		return run.LastResult.Total
	}
	if normal(debugTeam) >= normal(balancedTeam) {
		t.Fatal("balanced team should win a normal project")
	}
	outage := func(team []domain.StartupDev) int {
		run := hubRunAt(t, 65, 2, team)
		run.BossOrder = []string{BossOutage, BossDemoDay}
		shipNow(t, run, "Game", "Education")
		return run.LastResult.Total
	}
	if outage(debugTeam) <= outage(balancedTeam) {
		t.Fatal("debug team should win the outage boss")
	}
}

func TestIPOWinDoublesInvestor(t *testing.T) {
	staff := []domain.StartupDev{mkDev("a", 14, 14, 14, 14), mkDev("b", 14, 14, 14, 14), mkDev("c", 14, 14, 14, 14)}
	run := hubRunAt(t, 66, 8, staff)
	shipNow(t, run, "Web App", "Education")
	if run.Outcome != domain.StartupOutcomeIPO || run.Status != domain.StartupStatusEnded {
		t.Fatalf("expected IPO win: %+v", run)
	}
	sum := 0
	investor := 0
	for _, rv := range run.LastResult.Reviews {
		sum += rv.Score
		if rv.Reviewer == "Investor" {
			investor = rv.Score
		}
	}
	if run.LastResult.Total != sum+investor {
		t.Fatalf("investor should count double: total %d sum %d investor %d", run.LastResult.Total, sum, investor)
	}
	want := run.Money/10 + run.Fans + BossPassBonus*run.BossesPassed + BossWinBonus
	if run.Score != want {
		t.Fatalf("bad IPO score %d want %d: %+v", run.Score, want, run)
	}
}

func TestBankruptcyEndsPivot(t *testing.T) {
	dev := mkDev("d", 1, 1, 1, 1)
	dev.Salary = 5000
	run := hubRunAt(t, 67, 0, []domain.StartupDev{dev})
	run.Money = 100
	shipNow(t, run, "Game", "Fintech")
	if run.Status != domain.StartupStatusEnded || run.Outcome != domain.StartupOutcomePivot {
		t.Fatalf("bankruptcy should pivot: money=%d %+v", run.Money, run.LastResult)
	}
	if run.LastResult == nil {
		t.Fatal("reviews should still be recorded on a pivot")
	}
}

func TestAbandonEndsPivot(t *testing.T) {
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
	run, err := svc.Abandon(ctx, player)
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != domain.StartupStatusEnded || run.Outcome != domain.StartupOutcomePivot || run.Stage != domain.StartupStageEnded {
		t.Fatalf("abandon should end as pivot: %+v", run)
	}
	if _, err := svc.Abandon(ctx, player); !errors.Is(err, domain.ErrStartupNoActiveRun) {
		t.Fatalf("expected no active run after abandon, got %v", err)
	}
}
