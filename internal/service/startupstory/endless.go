package startupstory

import (
	"time"

	"gofiber-baro/internal/domain"
)

var endlessBossCycle = []string{BossDemoDay, BossRequirements, BossOutage, BossIPO}

func endlessBoss(act int, order []string) string {
	cycle := endlessBossCycle
	if len(order) > 2 {
		cycle = order[2:]
	}
	return cycle[(act-4)%len(cycle)]
}

func actForRun(run *domain.StartupRun) int {
	if run.Endless {
		return 1 + run.ProjectIndex/3
	}
	return actFor(run.ProjectIndex)
}

func bossPassMark(act int, boss string) int {
	if act <= 3 {
		return bossThreshold(act)
	}
	mark := min(EndlessPassBase+EndlessPassStep*(act-4), EndlessPassCap)
	if boss == BossIPO {
		mark = mark * 5 / 4
	}
	return mark
}

func refreshNextBoss(run *domain.StartupRun) {
	run.NextBoss = bossForRun((run.Act-1)*3+2, run.BossOrder)
	run.NextPassMark = bossPassMark(run.Act, run.NextBoss)
}

func endlessCheckpoint(run *domain.StartupRun, boss string, now time.Time) bool {
	if run.Endless || run.ProjectIndex < ProjectsPerRun {
		return false
	}
	run.Stage = domain.StartupStageIPOChoice
	return true
}

func ChooseAfterIPO(run *domain.StartupRun, keepGoing bool, now time.Time) error {
	if run.Stage != domain.StartupStageIPOChoice {
		return domain.ErrStartupWrongStage
	}
	if !keepGoing {
		endRun(run, domain.StartupOutcomeIPO, now)
		return nil
	}
	run.Endless = true
	r := rngFor(run)
	for _, i := range r.Perm(len(endlessBossCycle)) {
		run.BossOrder = append(run.BossOrder, endlessBossCycle[i])
	}
	run.Act = actForRun(run)
	run.MaxAct = max(run.MaxAct, run.Act)
	onNewAct(run)
	run.Candidates = rollCandidates(run, CandidateOffers)
	refreshPitches(run)
	refreshNextBoss(run)
	run.Stage = domain.StartupStageHub
	return nil
}

func endlessScoreBonus(run *domain.StartupRun) int {
	if !run.Endless {
		return 0
	}
	return BossWinBonus + EndlessActBonus*max(0, run.MaxAct-3)
}
