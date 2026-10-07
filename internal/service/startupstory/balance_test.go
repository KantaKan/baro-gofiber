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
	deaths                       []int
}

func (b botStats) medianDeathAct() int {
	if len(b.deaths) == 0 {
		return 0
	}
	sorted := append([]int{}, b.deaths...)
	for i := range sorted {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j] < sorted[i] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	return sorted[len(sorted)/2]
}

func (b botStats) ipoRate() float64     { return float64(b.ipo) / float64(b.runs) }
func (b botStats) perfectRate() float64 { return float64(b.perfect) / float64(b.projects) }
func (b botStats) firstAvg() float64    { return float64(b.firstTotal) / float64(b.runs) }

const BotRestAt = 60

func maxBurnout(run *domain.StartupRun) int {
	m := 0
	for _, d := range run.Staff {
		m = max(m, d.Burnout)
	}
	return m
}

func restedStaff(run *domain.StartupRun) []string {
	var ids []string
	for _, d := range run.Staff {
		if d.Burnout < BotRestAt {
			ids = append(ids, d.ID)
		}
	}
	if len(ids) == 0 {
		best := run.Staff[0]
		for _, d := range run.Staff {
			if d.Burnout < best.Burnout {
				best = d
			}
		}
		ids = []string{best.ID}
	}
	return ids
}

func smartHire(run *domain.StartupRun) {
	defer smartUpgrade(run)
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
		desk := 0
		if len(run.Staff) >= len(run.Desks) {
			desk = NextDeskPrice(run)
		}
		if run.Money-desk-c.Salary < (payroll+c.Salary)*2 {
			return
		}
		if desk > 0 && BuyDesk(run) != nil {
			return
		}
		if Hire(run, c.ID) != nil {
			return
		}
	}
}

func smartUpgrade(run *domain.StartupRun) {
	payroll := 0
	for _, d := range run.Staff {
		payroll += d.Salary
	}
	for i := range run.Desks {
		if run.Desks[i] < MaxDeskTier && run.Money-DeskUpgradePrice(run.Desks[i]) >= payroll*3 {
			_ = UpgradeDesk(run, i)
		}
	}
}

func smartInfra(run *domain.StartupRun) {
	for range 8 {
		l := currentLoad(run)
		if l == nil {
			return
		}
		inf := run.Infra
		app, db := l.App*13/10, l.DB*13/10
		var err error
		switch {
		case db > l.DBCap && !hasPart(inf, "index"):
			err = InfraAction(run, "part", 0, "index")
		case db > l.DBCap && inf.DB == "sqlite":
			err = InfraAction(run, "db", 0, "postgres")
		case db > l.DBCap && !hasPart(inf, "cache") && run.Act >= 2:
			err = InfraAction(run, "part", 0, "cache")
		case db > l.DBCap && run.Act >= 2:
			err = InfraAction(run, "replica", 0, "")
		case app > l.AppCap:
			weak := 0
			for i, s := range inf.Servers {
				if min(s.CPU, s.RAM) < min(inf.Servers[weak].CPU, inf.Servers[weak].RAM) {
					weak = i
				}
			}
			s := inf.Servers[weak]
			switch {
			case min(s.CPU, s.RAM) < 3 && s.CPU <= s.RAM:
				err = InfraAction(run, "cpu", weak, "")
			case min(s.CPU, s.RAM) < 3:
				err = InfraAction(run, "ram", weak, "")
			case !hasPart(inf, "lb"):
				err = InfraAction(run, "part", 0, "lb")
			case !hasPart(inf, "cdn") && run.Act >= 2:
				err = InfraAction(run, "part", 0, "cdn")
			default:
				err = InfraAction(run, "server", 0, "")
			}
		default:
			return
		}
		if err != nil {
			return
		}
	}
}

func playBot(t *testing.T, seed uint64, smart bool) (reachedIPO bool, deathAct int, bosses int, totals []int) {
	reachedIPO, deathAct, bosses, totals, _ = playBotInfra(t, seed, smart, smart)
	return
}

func playBotInfra(t *testing.T, seed uint64, smart, buyInfra bool) (reachedIPO bool, deathAct int, bosses int, totals []int, overloadAct int) {
	run := NewRun(primitive.NewObjectID(), 12, "learner", domain.StartupModeFree, seed, t0)
	if err := PickFounder(run, 0); err != nil {
		t.Fatal(err)
	}
	for run.Status == domain.StartupStatusActive && run.Act < 60 {
		switch run.Stage {
		case domain.StartupStagePerk:
			if err := PickPerk(run, 0); err != nil {
				t.Fatal(err)
			}
		case domain.StartupStageEvent:
			if err := PickEvent(run, 0); err != nil {
				t.Fatal(err)
			}
		case domain.StartupStageIPOChoice:
			reachedIPO = true
			if err := ChooseAfterIPO(run, smart, t0); err != nil {
				t.Fatal(err)
			}
		case domain.StartupStageItem:
			pick := 0
			if smart && maxBurnout(run) >= BotRestAt {
				pick = SkipItemForRetreat
			}
			if err := PickItem(run, pick); err != nil {
				t.Fatal(err)
			}
		case domain.StartupStageHub:
			typ, theme := "Web App", "Education"
			if buyInfra {
				smartInfra(run)
			}
			if smart {
				smartHire(run)
				best := -1.0
				for _, pt := range productTypes {
					if pt.OSSOnly && !run.OSS {
						continue
					}
					for _, th := range themes {
						if score := teamPower(run, pt, th); score > best {
							best, typ, theme = score, pt.Name, th
						}
					}
				}
			}
			var staff []string
			if smart {
				staff = restedStaff(run)
			}
			if err := StartProject(run, typ, theme, staff, t0); err != nil {
				t.Fatal(err)
			}
		case domain.StartupStageDeveloping:
			act, outOf := run.Act, 40
			if run.Project.Boss == BossIPO {
				outOf = 50
			}
			if err := Ship(run, run.Project.EndsAt); err != nil {
				t.Fatal(err)
			}
			if act <= 3 {
				totals = append(totals, run.LastResult.Total*40/outOf)
			}
			if run.LastResult.Overload > 1 && overloadAct == 0 {
				overloadAct = act
			}
		default:
			t.Fatalf("unexpected stage %s", run.Stage)
		}
	}
	if run.Outcome == domain.StartupOutcomeIPO {
		reachedIPO = true
	}
	return reachedIPO, run.MaxAct, run.BossesPassed, totals, overloadAct
}

func simulate(t *testing.T, smart bool) botStats {
	var b botStats
	for seed := uint64(1); seed <= 300; seed++ {
		reachedIPO, deathAct, bosses, totals := playBot(t, seed, smart)
		b.runs++
		if reachedIPO {
			b.ipo++
			b.deaths = append(b.deaths, deathAct)
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
	t.Logf("smart endless: of runs that reached IPO, median death act %d, deaths by act %v", smart.medianDeathAct(), smart.deaths)
	t.Logf("naive: IPO %.0f%%, first project avg %.1f/40, perfect %.1f%%, bosses passed %v", 100*naive.ipoRate(), naive.firstAvg(), 100*naive.perfectRate(), naive.bosses)
	if r := smart.ipoRate(); r < 0.2 || r > 0.4 {
		t.Errorf("a thoughtful player should reach IPO about 1 in 3 runs, got %.0f%%", 100*r)
	}
	if a := smart.firstAvg(); a < 22 || a > 30 {
		t.Errorf("first project should average ~26/40 for a thoughtful player, got %.1f", a)
	}
	if p := smart.perfectRate(); p > 0.1 {
		t.Errorf("perfect scores should be rare in the main game (Acts 1-3), got %.1f%%", 100*p)
	}
	if m := smart.medianDeathAct(); m < 4 || m > 8 {
		t.Errorf("endless runs should usually die around Act 5-6, median was Act %d", m)
	}
	if reached, won := smart.bosses[2]+smart.bosses[3], smart.bosses[3]; reached > 0 && float64(won)/float64(reached) > 0.9 {
		t.Errorf("the IPO Pitch is the climax: at most 90%% of runs that reach it should win, got %d/%d", won, reached)
	}
	if r := naive.ipoRate(); r > 0.03 {
		t.Errorf("a player who never hires or picks combos should almost never IPO, got %.0f%%", 100*r)
	}
}
