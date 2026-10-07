package startupstory

import (
	"strings"
	"testing"
)

func TestBossVisitsEveryFourthProject(t *testing.T) {
	run := newHubRun(t, 81)
	run.Staff[0].Burnout = 30
	for idx := 1; idx <= 8; idx++ {
		run.ProjectIndex = idx
		before := run.Staff[0].Burnout
		bossVisit(run)
		visiting := idx%BossVisitEvery == 0
		if run.BossVisiting != visiting {
			t.Fatalf("project %d: visiting=%v", idx, run.BossVisiting)
		}
		if visiting && (run.Staff[0].Burnout != before-BossVisitBurnout || !strings.Contains(run.Log[len(run.Log)-1], "The Boss dropped by")) {
			t.Fatalf("a visit should cut burnout by %d and log a line: %d -> %d", BossVisitBurnout, before, run.Staff[0].Burnout)
		}
	}
	if run.BossVisits != 2 {
		t.Fatalf("8 projects should mean 2 visits, got %d", run.BossVisits)
	}
}
