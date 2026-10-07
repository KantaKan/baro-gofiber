package startupstory

import "gofiber-baro/internal/domain"

const (
	BossVisitEvery   = 4
	BossVisitBurnout = 5
)

var bossVisitLines = []string{
	"The Boss dropped by: \"Just testing farming. Side job. Not quitting. Yet.\" Homegrown morning glory for everyone (−5 burnout).",
	"The Boss dropped by: \"These chillies have better uptime than our servers.\" Fresh chillies for everyone (−5 burnout).",
	"The Boss dropped by: \"In five years: durian farm. Mark my words.\" Homegrown snacks for everyone (−5 burnout).",
	"The Boss dropped by: \"ปลูกผักดีกว่าแก้บั๊ก\" Veggies for the whole team (−5 burnout).",
}

func bossVisit(run *domain.StartupRun) {
	run.BossVisiting = run.ProjectIndex > 0 && run.ProjectIndex%BossVisitEvery == 0
	if !run.BossVisiting {
		return
	}
	run.BossVisits++
	for i := range run.Staff {
		run.Staff[i].Burnout = max(0, run.Staff[i].Burnout-BossVisitBurnout)
	}
	addLog(run, bossVisitLines[rngFor(run).IntN(len(bossVisitLines))])
}
