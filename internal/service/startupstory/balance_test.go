package startupstory

import (
	"testing"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type botStats struct {
	runs, ipo, projects, perfect int
	bosses                       [4]int
	firstTotal                   int
}

func (b botStats) ipoRate() float64     { return float64(b.ipo) / float64(b.runs) }
func (b botStats) perfectRate() float64 { return float64(b.perfect) / float64(b.projects) }
func (b botStats) firstAvg() float64    { return float64(b.firstTotal) / float64(b.runs) }

func teamPower(run *domain.StartupRun, t productType, theme string) float64 {
	power := 0.0
	for _, d := range run.Staff {
		s := stats(d)
		for i := range s {
			power += float64(s[i]) * t.Weights[i]
		}
	}
	return power * comboMultipliers[comboFor(t, theme)] * marketMult(run.Market, theme)
}

func smartHire(run *domain.StartupRun) {
	for len(run.Staff) < TeamCap(run.Act) && len(run.Candidates) > 0 {
		have := map[string]bool{}
		payroll := 0
		for _, d := range run.Staff {
			have[d.Role] = true
			payroll += d.Salary
		}
		pick := -1
		for i, c := range run.Candidates {
			if !have[c.Role] && (pick < 0 || skillOf(c) > skillOf(run.Candidates[pick])) {
				pick = i
			}
		}
		if pick < 0 {
			for i, c := range run.Candidates {
				if pick < 0 || skillOf(c) > skillOf(run.Candidates[pick]) {
					pick = i
				}
			}
		}
		c := run.Candidates[pick]
		if run.Money-c.Salary < (payroll+c.Salary)*2 {
			return
		}
		if Hire(run, c.ID) != nil {
			return
		}
	}
}

func playBot(t *testing.T, seed uint64, smart bool) (outcome string, bosses int, totals []int) {
	run := NewRun(primitive.NewObjectID(), 12, "learner", domain.StartupModeFree, seed, t0)
	if err := PickFounder(run, 0); err != nil {
		t.Fatal(err)
	}
	for run.Status == domain.StartupStatusActive {
		switch run.Stage {
		case domain.StartupStageItem:
			if err := PickItem(run, 0); err != nil {
				t.Fatal(err)
			}
		case domain.StartupStageHub:
			typ, theme := "Web App", "Education"
			if smart {
				smartHire(run)
				best := -1.0
				for _, pt := range productTypes {
					for _, th := range themes {
						if p := teamPower(run, pt, th); p > best {
							best, typ, theme = p, pt.Name, th
						}
					}
				}
			}
			if err := StartProject(run, typ, theme, nil, t0); err != nil {
				t.Fatal(err)
			}
		case domain.StartupStageDeveloping:
			if err := Ship(run, run.Project.EndsAt); err != nil {
				t.Fatal(err)
			}
			totals = append(totals, run.LastResult.Total)
		default:
			t.Fatalf("unexpected stage %s", run.Stage)
		}
	}
	return run.Outcome, run.BossesPassed, totals
}

func simulate(t *testing.T, smart bool) botStats {
	var b botStats
	for seed := uint64(1); seed <= 300; seed++ {
		outcome, bosses, totals := playBot(t, seed, smart)
		b.runs++
		if outcome == domain.StartupOutcomeIPO {
			b.ipo++
		}
		b.bosses[min(bosses, 3)]++
		b.firstTotal += totals[0]
		for _, total := range totals {
			b.projects++
			if total >= 40 {
				b.perfect++
			}
		}
	}
	return b
}

func TestBalanceIsChallenging(t *testing.T) {
	smart := simulate(t, true)
	naive := simulate(t, false)
	t.Logf("smart: IPO %.0f%%, first project avg %.1f/40, perfect %.1f%%, bosses passed %v", 100*smart.ipoRate(), smart.firstAvg(), 100*smart.perfectRate(), smart.bosses)
	t.Logf("naive: IPO %.0f%%, first project avg %.1f/40, perfect %.1f%%, bosses passed %v", 100*naive.ipoRate(), naive.firstAvg(), 100*naive.perfectRate(), naive.bosses)
	if r := smart.ipoRate(); r < 0.2 || r > 0.45 {
		t.Errorf("a thoughtful player should reach IPO about 1 in 3 runs, got %.0f%%", 100*r)
	}
	if a := smart.firstAvg(); a < 22 || a > 30 {
		t.Errorf("first project should average ~26/40 for a thoughtful player, got %.1f", a)
	}
	if p := smart.perfectRate(); p > 0.1 {
		t.Errorf("perfect scores should be rare, got %.1f%%", 100*p)
	}
	if r := naive.ipoRate(); r > 0.03 {
		t.Errorf("a player who never hires or picks combos should almost never IPO, got %.0f%%", 100*r)
	}
}
