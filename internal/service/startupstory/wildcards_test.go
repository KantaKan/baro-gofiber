package startupstory

import (
	"errors"
	"testing"

	"gofiber-baro/internal/domain"
)

var botWildcards []string

func allWildcards() []string { return wildcardsFor(1 << 30) }

func TestWildcardsUnlockWithFame(t *testing.T) {
	if len(wildcardsFor(0)) != 0 {
		t.Fatal("a new studio should have no wildcards")
	}
	if got := wildcardsFor(40); len(got) != 5 {
		t.Fatalf("40 fame should unlock 5 wildcards, got %v", got)
	}
	if len(allWildcards()) != len(wildcardCatalog) {
		t.Fatal("enough fame unlocks every wildcard")
	}
}

func TestWildcardRollIsRareAndCapped(t *testing.T) {
	rolls, seen := 0, 0
	for seed := uint64(1); seed <= 2000; seed++ {
		run := newHubRun(t, seed)
		run.UnlockedWildcards = allWildcards()
		cands := rollCandidates(run, CandidateOffers)
		n := 0
		for _, c := range cands {
			if c.Wildcard != "" {
				n++
			}
		}
		if n > 1 {
			t.Fatalf("at most one wildcard per roll, got %d", n)
		}
		rolls++
		seen += n
		run.Staff = append(run.Staff, domain.StartupDev{ID: "w", Wildcard: WildDuck})
		if hasWildcard(rollCandidates(run, CandidateOffers)) {
			t.Fatal("no wildcard candidates while one is already on the team")
		}
	}
	if rate := float64(seen) / float64(rolls); rate < 0.08 || rate > 0.22 {
		t.Fatalf("about 5%% per slot over 3 slots, got %.1f%% of rolls", 100*rate)
	}
}

func TestWildcardSalaries(t *testing.T) {
	base := domain.StartupDev{Frontend: 4, Backend: 4, Design: 4, Debug: 4, Salary: salaryFor(4, 4, 4, 4, "")}
	ai, intern, ghost := base, base, base
	makeWildcard(&ai, WildAI)
	makeWildcard(&intern, WildIntern)
	makeWildcard(&ghost, WildGhost)
	if ai.Salary != 0 || intern.Salary != base.Salary/2 {
		t.Fatalf("AI is free and the intern is half price: %d %d", ai.Salary, intern.Salary)
	}
	if ghost.Salary != base.Salary || stats(ghost) != [4]int{} {
		t.Fatalf("the ghost keeps full salary and has no skills: %d %v", ghost.Salary, stats(ghost))
	}
}

func TestVimWizardCannotBeDismissed(t *testing.T) {
	run := newHubRun(t, 71)
	run.Staff = append(run.Staff, domain.StartupDev{ID: "vim", Wildcard: WildVim})
	if err := Dismiss(run, "vim"); !errors.Is(err, domain.ErrStartupCantExit) {
		t.Fatalf("expected can't exit, got %v", err)
	}
}

func TestWildcardEffects(t *testing.T) {
	run := newHubRun(t, 72)
	out := wildcardEffects(run, []domain.StartupDev{{Wildcard: WildDuck}, {Wildcard: WildFine}, {Wildcard: WildAI}})
	if out.bugs != -DuckBugCut+FineBugs+1 || out.powerMult != 1+AIPowerBoost {
		t.Fatalf("duck -3, fine +2, AI +1 bug and +15%% power: %+v", out)
	}
	if isBuilder(domain.StartupDev{Wildcard: WildDuck, Role: RoleDevOps}) {
		t.Fatal("the duck never builds")
	}
	run.Staff = []domain.StartupDev{{ID: "c", Wildcard: WildCat, Burnout: 30}, {ID: "f", Wildcard: WildFine, Burnout: 70}, {ID: "x", Burnout: 50}}
	wildcardBurnout(run)
	if run.Staff[0].Burnout != 20 || run.Staff[1].Burnout != 0 || run.Staff[2].Burnout != 40 {
		t.Fatalf("cat -10 for everyone, fine never burns out: %+v", run.Staff)
	}
	s := [4]int{}
	ghostBossBonus(domain.StartupDev{Wildcard: WildGhost}, BossDemoDay, &s)
	if s != [4]int{4, 4, 4, 4} {
		t.Fatalf("the ghost shows up for bosses: %v", s)
	}
}

func TestJesterIsDeterministicPerSeed(t *testing.T) {
	a, b := newHubRun(t, 73), newHubRun(t, 73)
	team := []domain.StartupDev{{Wildcard: WildJester}}
	wildcardEffects(a, team)
	wildcardEffects(b, team)
	if a.Log[len(a.Log)-1] != b.Log[len(b.Log)-1] {
		t.Fatal("same seed should give the same jester roll")
	}
}

func TestGreybeardSlowsProjects(t *testing.T) {
	run := newHubRun(t, 74)
	plain := projectDurationSecs(run, run.Staff, "")
	if slow := projectDurationSecs(run, []domain.StartupDev{{Wildcard: WildGreybeard}}, ""); slow <= plain {
		t.Fatalf("it depends: %d vs %d", slow, plain)
	}
}

func TestBalanceHoldsWithWildcards(t *testing.T) {
	botWildcards = allWildcards()
	defer func() { botWildcards = nil }()
	b := simulate(t, true)
	t.Logf("with every wildcard unlocked: IPO %.0f%%", 100*b.ipoRate())
	if r := b.ipoRate(); r < 0.2 || r > 0.45 {
		t.Errorf("wildcards should spice runs up, not break them: IPO %.0f%%", 100*r)
	}
}
