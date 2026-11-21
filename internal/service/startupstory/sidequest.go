package startupstory

import (
	"fmt"

	"gofiber-baro/internal/domain"
)

const (
	DistractedAt    = 60
	SideQuestChance = 10
)

var statNames = [4]string{"Frontend", "Backend", "Design", "Debug"}

var sideQuestFinds = []string{"a cool library", "a conference talk", "a 3-hour YouTube rabbit hole", "an old blog post", "a Discord server"}

func sideQuests(run *domain.StartupRun, team []domain.StartupDev) {
	onTeam := map[string]bool{}
	for _, d := range team {
		onTeam[d.ID] = true
	}
	r := rngFor(run)
	for i := range run.Staff {
		d := &run.Staff[i]
		if !onTeam[d.ID] || d.Burnout < DistractedAt || d.QuestAct == run.Act || r.IntN(100) >= SideQuestChance {
			continue
		}
		d.QuestAct = run.Act
		stat := r.IntN(len(statNames))
		switch stat {
		case 0:
			d.Frontend++
		case 1:
			d.Backend++
		case 2:
			d.Design++
		default:
			d.Debug++
		}
		addLog(run, fmt.Sprintf("%s wandered off on a side quest, found %s, and came back with +1 %s.", d.Name, sideQuestFinds[r.IntN(len(sideQuestFinds))], statNames[stat]))
	}
}
