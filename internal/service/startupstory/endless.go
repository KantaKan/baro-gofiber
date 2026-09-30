package startupstory

import (
	"time"

	"gofiber-baro/internal/domain"
)

func endlessCheckpoint(run *domain.StartupRun, boss string, now time.Time) bool {
	if run.ProjectIndex < ProjectsPerRun {
		return false
	}
	outcome := domain.StartupOutcomePivot
	if boss == BossIPO {
		outcome = domain.StartupOutcomeIPO
	}
	endRun(run, outcome, now)
	return true
}

func endlessScoreBonus(run *domain.StartupRun) int {
	return 0
}
