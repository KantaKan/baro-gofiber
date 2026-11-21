package startupstory

import (
	"fmt"
	"math/rand/v2"

	"gofiber-baro/internal/domain"
)

const (
	WildcardChance = 5

	WildDuck      = "duck"
	WildCat       = "cat"
	WildTenX      = "tenx"
	WildFine      = "fine"
	WildJester    = "jester"
	WildVim       = "vim"
	WildGreybeard = "greybeard"
	WildIntern    = "intern"
	WildAI        = "ai"
	WildGhost     = "ghost"

	DuckBugCut        = 3
	CatBurnoutCut     = 10
	FineBugs          = 2
	VimBonus          = 3
	GreybeardBonus    = 2
	GreybeardSlowdown = 1.2
	InternBugs        = 3
	AIPowerBoost      = 0.15
	GhostBossBonus    = 4
)

type WildcardInfo struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Title string `json:"title"`
	Desc  string `json:"desc"`
	Fame  int    `json:"fame"`
}

var wildcardCatalog = []WildcardInfo{
	{ID: WildDuck, Name: "Rubber Duck", Title: "Debugs by listening", Desc: "−3 bugs every ship. Doesn't build anything.", Fame: 10},
	{ID: WildCat, Name: "Office Cat", Title: "Sits on keyboards", Desc: "Whole team −10 burnout every project. Almost no skills.", Fame: 10},
	{ID: WildTenX, Name: "10x Dev", Title: "Ships at 3am", Desc: "Huge skills, double salary.", Fame: 40},
	{ID: WildFine, Name: "Everything's Fine", Title: "Prod is on fire", Desc: "Never burns out. +2 bugs every ship.", Fame: 40},
	{ID: WildIntern, Name: "The Intern", Title: "Pushed to main", Desc: "Half salary. Each ship: 50% Users love it, 50% +3 bugs.", Fame: 40},
	{ID: WildAI, Name: "AI Pair Programmer", Title: "Confidently wrong", Desc: "No salary. +15% power, +1 bug every ship.", Fame: 80},
	{ID: WildGreybeard, Name: "Senior Greybeard", Title: "It depends.", Desc: "+2 to every skill. Projects take 20% longer.", Fame: 80},
	{ID: WildJester, Name: "The Jester", Title: "Wildcard", Desc: "Something random happens every project. Good or bad.", Fame: 150},
	{ID: WildVim, Name: "Vim Wizard", Title: "Can't exit. Ever.", Desc: "+3 Backend and Debug. You can never let them go.", Fame: 150},
	{ID: WildGhost, Name: "Ghost Employee", Title: "Only shows up on Demo Day", Desc: "No skills on normal projects. +4 to every skill on boss fights.", Fame: 250},
}

func init() {
	for _, w := range wildcardCatalog {
		unlockTable = append(unlockTable, UnlockInfo{Fame: w.Fame, Kind: "wildcard", ID: w.ID, Name: w.Name})
	}
}

func wildcardsFor(fame int) []string {
	var ids []string
	for _, w := range wildcardCatalog {
		if fame >= w.Fame {
			ids = append(ids, w.ID)
		}
	}
	return ids
}

func findWildcard(id string) (WildcardInfo, bool) {
	for _, w := range wildcardCatalog {
		if w.ID == id {
			return w, true
		}
	}
	return WildcardInfo{}, false
}

func hasWildcard(devs []domain.StartupDev) bool {
	for _, d := range devs {
		if d.Wildcard != "" {
			return true
		}
	}
	return false
}

func rollWildcards(run *domain.StartupRun, r *rand.Rand, cands []domain.StartupDev) {
	if len(run.UnlockedWildcards) == 0 || hasWildcard(run.Staff) {
		return
	}
	for i := range cands {
		if r.IntN(100) >= WildcardChance {
			continue
		}
		makeWildcard(&cands[i], run.UnlockedWildcards[r.IntN(len(run.UnlockedWildcards))])
		return
	}
}

func makeWildcard(d *domain.StartupDev, id string) {
	w, ok := findWildcard(id)
	if !ok {
		return
	}
	d.Wildcard, d.WildDesc, d.Name, d.Title, d.GenmateID, d.Trait = w.ID, w.Desc, w.Name, w.Title, "", ""
	switch id {
	case WildCat:
		d.Frontend, d.Backend, d.Design, d.Debug = 0, 0, 1, 0
	case WildTenX:
		d.Trait = TraitTenX
	case WildGhost:
		salary := d.Salary
		d.Frontend, d.Backend, d.Design, d.Debug = 0, 0, 0, 0
		d.Salary = salary
		return
	}
	d.Salary = salaryFor(d.Frontend, d.Backend, d.Design, d.Debug, d.Trait)
	switch id {
	case WildIntern:
		d.Salary /= 2
	case WildAI:
		d.Salary = 0
	}
}

func wildcardStats(d domain.StartupDev, s *[4]int) {
	switch d.Wildcard {
	case WildVim:
		s[1] += VimBonus
		s[3] += VimBonus
	case WildGreybeard:
		for i := range s {
			s[i] += GreybeardBonus
		}
	}
}

type wildOutcome struct {
	bugs      int
	powerMult float64
	reviewer  map[string]float64
}

var jesterRolls = []struct {
	note  string
	apply func(*wildOutcome)
}{
	{"The Jester juggled the roadmap and it somehow worked. +20% power.", func(o *wildOutcome) { o.powerMult *= 1.2 }},
	{"The Jester renamed every variable to emoji. −20% power.", func(o *wildOutcome) { o.powerMult *= 0.8 }},
	{"The Jester found three bugs by sitting on the keyboard. −3 bugs.", func(o *wildOutcome) { o.bugs -= 3 }},
	{"The Jester shipped a hidden April Fools feature. +3 bugs.", func(o *wildOutcome) { o.bugs += 3 }},
	{"The Jester performed at the demo. Users loved it.", func(o *wildOutcome) { o.reviewer["Users"]++ }},
	{"The Jester pitched to investors in a clown nose. Investors loved it.", func(o *wildOutcome) { o.reviewer["Investor"]++ }},
}

func wildcardEffects(run *domain.StartupRun, team []domain.StartupDev) wildOutcome {
	out := wildOutcome{powerMult: 1, reviewer: map[string]float64{}}
	for _, d := range team {
		switch d.Wildcard {
		case WildDuck:
			out.bugs -= DuckBugCut
		case WildFine:
			out.bugs += FineBugs
		case WildAI:
			out.bugs++
			out.powerMult *= 1 + AIPowerBoost
		case WildIntern:
			if rngFor(run).IntN(2) == 0 {
				out.reviewer["Users"]++
				addLog(run, "The intern pushed straight to main. Users loved the new button.")
			} else {
				out.bugs += InternBugs
				addLog(run, "The intern pushed straight to main. +3 bugs.")
			}
		case WildJester:
			roll := jesterRolls[rngFor(run).IntN(len(jesterRolls))]
			roll.apply(&out)
			addLog(run, roll.note)
		}
	}
	return out
}

func ghostBossBonus(d domain.StartupDev, boss string, s *[4]int) {
	if d.Wildcard != WildGhost || boss == "" {
		return
	}
	for i := range s {
		s[i] += GhostBossBonus
	}
}

func wildcardBurnout(run *domain.StartupRun) {
	cat := false
	for _, d := range run.Staff {
		if d.Wildcard == WildCat {
			cat = true
		}
	}
	for i := range run.Staff {
		d := &run.Staff[i]
		if d.Wildcard == WildFine {
			d.Burnout = 0
		} else if cat {
			d.Burnout = max(0, d.Burnout-CatBurnoutCut)
		}
	}
	if cat {
		addLog(run, fmt.Sprintf("The office cat sat on %d keyboards. Everyone feels a bit better.", len(run.Staff)))
	}
}
