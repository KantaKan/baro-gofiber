package startupstory

import (
	"context"
	"errors"
	"testing"
	"time"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func runAtIPOChoice(t *testing.T, seed uint64) *domain.StartupRun {
	t.Helper()
	run := NewRun(primitive.NewObjectID(), 12, "learner", domain.StartupModeFree, seed, t0)
	_ = PickFounder(run, 0)
	run.Staff[0].Frontend, run.Staff[0].Backend, run.Staff[0].Design, run.Staff[0].Debug = 40, 40, 40, 40
	for run.Stage != domain.StartupStageIPOChoice {
		switch run.Stage {
		case domain.StartupStageItem:
			_ = PickItem(run, 0)
		case domain.StartupStageHub:
			run.Staff[0].Burnout = 0
			if err := StartProject(run, "Web App", "Education", nil, t0); err != nil {
				t.Fatal(err)
			}
		case domain.StartupStageDeveloping:
			if err := Ship(run, run.Project.EndsAt); err != nil {
				t.Fatal(err)
			}
		default:
			t.Fatalf("run ended before the IPO: %s %s", run.Stage, run.Outcome)
		}
	}
	return run
}

func TestCashOutEndsAsIPO(t *testing.T) {
	run := runAtIPOChoice(t, 3)
	if err := ChooseAfterIPO(run, false, t0); err != nil {
		t.Fatal(err)
	}
	if run.Status != domain.StartupStatusEnded || run.Outcome != domain.StartupOutcomeIPO || run.Endless {
		t.Fatalf("cash out should end the run as an IPO: %+v", run)
	}
	if err := ChooseAfterIPO(run, true, t0); !errors.Is(err, domain.ErrStartupWrongStage) {
		t.Fatalf("the choice can only be made once, got %v", err)
	}
}

func TestKeepGoingEntersEndless(t *testing.T) {
	run := runAtIPOChoice(t, 3)
	if err := ChooseAfterIPO(run, true, t0); err != nil {
		t.Fatal(err)
	}
	if !run.Endless || run.Act != 4 || run.MaxAct != 4 || run.Stage != domain.StartupStageHub || run.Status != domain.StartupStatusActive {
		t.Fatalf("keep going should open Act 4 in the hub: %+v", run)
	}
	if len(run.Candidates) == 0 || run.NextBoss == "" || run.NextPassMark < EndlessPassBase {
		t.Fatalf("endless act needs candidates and a known boss: %+v", run)
	}
	if TeamCap(4) != 7 || TeamCap(9) != EndlessTeamCap {
		t.Fatalf("team cap should climb to %d: %d %d", EndlessTeamCap, TeamCap(4), TeamCap(9))
	}
}

func TestEndlessGetsHarderEveryAct(t *testing.T) {
	for act := 4; act < 12; act++ {
		if actScale(act+1) <= actScale(act) {
			t.Fatalf("act %d should be harder than act %d", act+1, act)
		}
		if bossPassMark(act+1, BossDemoDay) < bossPassMark(act, BossDemoDay) || bossPassMark(act, BossDemoDay) > 40 {
			t.Fatalf("pass marks must rise but stay reachable (out of 40): act %d = %d", act, bossPassMark(act, BossDemoDay))
		}
	}
	if bossPassMark(4, BossIPO) <= bossPassMark(4, BossDemoDay) {
		t.Fatal("IPO-style bosses count Investor double, so they need a higher mark")
	}
}

func TestEndlessBossOrderIsSeededPerRun(t *testing.T) {
	a, b := runAtIPOChoice(t, 3), runAtIPOChoice(t, 3)
	_ = ChooseAfterIPO(a, true, t0)
	_ = ChooseAfterIPO(b, true, t0)
	for act := 4; act < 9; act++ {
		if bossForRun((act-1)*3+2, a.BossOrder) != bossForRun((act-1)*3+2, b.BossOrder) {
			t.Fatal("the same seed must give the same endless bosses (Weekly Seed fairness)")
		}
	}
}

func TestEndlessDeathKeepsDepthAndBonus(t *testing.T) {
	run := runAtIPOChoice(t, 3)
	_ = ChooseAfterIPO(run, true, t0)
	run.MaxAct = 6
	endRun(run, domain.StartupOutcomePivot, t0)
	base := run.Money/10 + run.Fans + BossPassBonus*run.BossesPassed
	if run.Score != base+BossWinBonus+EndlessActBonus*3 {
		t.Fatalf("endless score should add the IPO bonus and a depth bonus, got %d want %d", run.Score, base+BossWinBonus+EndlessActBonus*3)
	}
}

func TestDeepestBoardRanksByActThenScore(t *testing.T) {
	week := weekKeyFor(t0)
	mk := func(name string, maxAct, score int) *domain.StartupRun {
		return &domain.StartupRun{ID: primitive.NewObjectID(), OwnerID: primitive.NewObjectID(), Cohort: 12, Mode: domain.StartupModeRanked, WeekKey: week, Status: domain.StartupStatusEnded, MaxAct: maxAct, Score: score}
	}
	deep, rich, mid := mk("deep", 7, 100), mk("rich", 3, 99999), mk("mid", 7, 50)
	store := &fakeStore{boardRuns: []*domain.StartupRun{rich, mid, deep}, boardNames: map[string]string{}, boardRoles: map[string]string{}}
	svc := &Service{store: store, now: func() time.Time { return t0 }}
	rows, err := svc.Leaderboard(context.Background(), Player{ID: primitive.NewObjectID().Hex(), Cohort: 12}, domain.StartupBoardDeepest)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 || rows[0].OwnerID != deep.OwnerID || rows[1].OwnerID != mid.OwnerID || rows[2].OwnerID != rich.OwnerID {
		t.Fatalf("deepest board should rank act first, then score: %+v", rows)
	}
}
