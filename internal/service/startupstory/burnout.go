package startupstory

import (
	"math/rand/v2"
	"strings"
	"time"

	"gofiber-baro/internal/domain"
)

const (
	BurnoutWork        = 9
	BurnoutPerAct      = 2
	BurnoutBoss        = 6
	BurnoutNightOwl    = 4
	BurnoutCrunch      = 6
	BurnoutRest        = 16
	BurnoutRetreat     = 25
	BurnoutPMDamp      = 0.7
	BurnoutQuitAt      = 100
	SkipItemForRetreat = -1
	runLogLimit        = 20
)

var quitRoasts = []string{
	"{name} quit to open a café. The café already makes more than your startup",
	"{name} left for a company that has a PM",
	"{name} rage-quit after the 4th 'quick sync' today",
	"{name} is now a full-time TikTok coding influencer",
	"{name} uninstalled VS Code and moved to a farm in Chiang Mai",
	"{name} saw the codebase one last time and whispered 'nope'",
	"{name} rewrote their resume in Rust and left",
	"{name} said 'I use Arch btw' and was never seen again",
	"{name} left a 3,000-line PR called 'final fix' and vanished",
	"{name} quit to sell mookata. Honestly? Smart",
	"{name} burned out so hard their keyboard is still warm",
	"{name} joined a crypto startup. Pray for them",
	"{name} found out the 'unlimited leave' was a lie",
	"{name} pushed to main on Friday and fled the country",
	"{name} is 'taking a break' (they are on LinkedIn 'Open to work')",
	"{name} left a sticky note: 'it works on my machine' and walked out",
	"{name} became a monk. Inner peace > your sprint",
	"{name} said ok boomer to the deadline and left",
	"{name} is now a barista who refuses to talk about JavaScript",
	"{name} quit. Their last commit message: 'good luck lol'",
	"{name} ran out of coffee and willpower at the same time",
	"{name} closed 47 Jira tickets as 'won't fix' and left",
}

var gentleBreaks = []string{
	"{name} took a well-deserved break. Thanks for the ride!",
	"{name} is recharging for a while. See you next run!",
	"{name} stepped away to rest. Proud of the work!",
}

var founderBurnouts = []string{
	"{name} the founder burned out and moved to Pai to 'find themselves'",
	"{name} the founder fell asleep on the keyboard and woke up in a different career",
	"{name} the founder logged off. Forever. The startup pivots",
}

func isFounder(d domain.StartupDev) bool {
	return strings.HasPrefix(d.ID, "founder-")
}

func burnoutGain(run *domain.StartupRun, d domain.StartupDev, boss bool, hasPM bool) int {
	gain := float64(BurnoutWork + BurnoutPerAct*(run.Act-1) + effectsOf(run.Items).burnout)
	if boss {
		gain += BurnoutBoss
	}
	if d.Trait == TraitNightOwl {
		gain += BurnoutNightOwl
	}
	for _, it := range run.Items {
		if it == "crunch-culture" {
			gain += BurnoutCrunch
		}
	}
	if hasPM {
		gain *= BurnoutPMDamp
	}
	return max(0, int(gain+0.5))
}

func pickLine(r *rand.Rand, pool []string, name string) string {
	return strings.ReplaceAll(pool[r.IntN(len(pool))], "{name}", name)
}

func addLog(run *domain.StartupRun, line string) {
	run.Log = append(run.Log, line)
	if len(run.Log) > runLogLimit {
		run.Log = run.Log[len(run.Log)-runLogLimit:]
	}
}

func afterShipBurnout(run *domain.StartupRun, team []domain.StartupDev, now time.Time) bool {
	onTeam := map[string]bool{}
	for _, d := range team {
		onTeam[d.ID] = true
	}
	boss := run.ProjectIndex > 0 && isBossIndex(run.ProjectIndex-1)
	hasPM := jobsFor(team).hasPM
	for i := range run.Staff {
		d := &run.Staff[i]
		if onTeam[d.ID] {
			d.Burnout = min(BurnoutQuitAt, d.Burnout+burnoutGain(run, *d, boss, hasPM))
		} else {
			d.Burnout = max(0, d.Burnout-BurnoutRest)
		}
	}
	var stay []domain.StartupDev
	var r *rand.Rand
	for _, d := range run.Staff {
		if d.Burnout < BurnoutQuitAt {
			stay = append(stay, d)
			continue
		}
		if r == nil {
			r = rngFor(run)
		}
		if isFounder(d) && isCompany(run) {
			founderBreak(run, &d)
			stay = append(stay, d)
			continue
		}
		if isFounder(d) {
			addLog(run, pickLine(r, founderBurnouts, d.Name))
			endRun(run, domain.StartupOutcomePivot, now)
			return true
		}
		pool := quitRoasts
		if d.GenmateID != "" {
			pool = gentleBreaks
		}
		addLog(run, pickLine(r, pool, d.Name))
	}
	run.Staff = stay
	return false
}

func teamRetreat(run *domain.StartupRun) {
	for i := range run.Staff {
		run.Staff[i].Burnout = max(0, run.Staff[i].Burnout-BurnoutRetreat)
	}
	addLog(run, "Team retreat to Hua Hin! Everyone feels human again")
}
