package startupstory

import "gofiber-baro/internal/domain"

type GimmickInfo struct {
	ID         string             `json:"id"`
	Name       string             `json:"name"`
	Desc       string             `json:"desc"`
	Weights    *[4]float64        `json:"-"`
	WeightMult *[4]float64        `json:"-"`
	PowerMult  float64            `json:"-"`
	Bugs       int                `json:"-"`
	Reviewer   map[string]float64 `json:"-"`
	MoneyMult  float64            `json:"-"`
	FansMult   float64            `json:"-"`
}

var gimmickCatalog = []GimmickInfo{
	{ID: "readme-only", Name: "Investor Only Reads the README", Desc: "+1.5 Dev Community, −1 Investor. Skimmed it between meetings.", Reviewer: map[string]float64{"Dev Community": 1.5, "Investor": -1}},
	{ID: "hates-js", Name: "Tech Lead Hates JavaScript Today", Desc: "Frontend counts half. It's a phase.", WeightMult: &[4]float64{0.5, 1, 1, 1}},
	{ID: "wifi-down", Name: "Demo Day WiFi Is Down", Desc: "Backend doesn't count. Radio silence on port 8080.", Weights: &[4]float64{0.4, 0, 0.4, 0.2}},
	{ID: "nephew-joins", Name: "The CEO's Nephew Joins the Demo", Desc: "+1.5 Users, −1 Dev Community. He pressed one button.", Reviewer: map[string]float64{"Users": 1.5, "Dev Community": -1}},
	{ID: "flaky-ci", Name: "CI Is Flaky Today", Desc: "+2 bugs. It was green in staging, we promise.", Bugs: 2},
	{ID: "coffee-budget", Name: "Emergency Coffee Budget Approved", Desc: "+12% build power. Blood type: espresso.", PowerMult: 1.12},
	{ID: "office-dog", Name: "The Office Dog Stole the Demo", Desc: "−7% power, +1 Users. Worth it.", PowerMult: 0.93, Reviewer: map[string]float64{"Users": 1}},
	{ID: "jira-avalanche", Name: "Jira Avalanche", Desc: "+1 bug, −0.5 Tech Lead. Twelve new tickets, all urgent.", Bugs: 1, Reviewer: map[string]float64{"Tech Lead": -0.5}},
	{ID: "sponsored-deck", Name: "Investor Forwarded Your Deck", Desc: "+25% money. Someone said yes to a meeting.", MoneyMult: 1.25},
	{ID: "standup-marathon", Name: "All-Hands Standup Marathon", Desc: "−5% power, +0.5 Investor. Status: also a meeting.", PowerMult: 0.95, Reviewer: map[string]float64{"Investor": 0.5}},
}

var gimmickPool = []string{
	"readme-only", "hates-js", "wifi-down", "nephew-joins", "flaky-ci",
	"coffee-budget", "office-dog", "jira-avalanche", "sponsored-deck", "standup-marathon",
}

func findGimmick(id string) (GimmickInfo, bool) {
	for _, g := range gimmickCatalog {
		if g.ID == id {
			return g, true
		}
	}
	return GimmickInfo{}, false
}

func onBossStart(run *domain.StartupRun) {
	if run.Status != domain.StartupStatusActive {
		return
	}
	r := rngFor(run)
	run.BossGimmick = gimmickPool[r.IntN(len(gimmickPool))]
}

func applyGimmicks(sc *scoring) {
	if g, ok := findGimmick(sc.run.BossGimmick); ok {
		if g.Weights != nil {
			sc.weights = *g.Weights
		}
		if g.WeightMult != nil {
			for i, m := range *g.WeightMult {
				sc.weights[i] *= m
			}
		}
		sc.powerMult *= nonZero(g.PowerMult)
		sc.bugs += g.Bugs
		for name, bias := range g.Reviewer {
			sc.reviewer[name] += bias
		}
		sc.moneyMult *= nonZero(g.MoneyMult)
		sc.fansMult *= nonZero(g.FansMult)
	}
	applyWorldScoring(sc)
}
