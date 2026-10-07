package startupstory

import (
	"sort"

	"gofiber-baro/internal/domain"
)

const (
	StartingDesks  = 2
	DeskBasePrice  = 1500
	DeskStepPrice  = 500
	MaxDeskTier    = 3
	DeskUpgradeAct = 2
	DeskTierBonus  = 1
	startDeskTier  = 1
	deskTier2Price = 1000
	deskTier3Price = 2500
)

type DeskPrices struct {
	Base    int   `json:"base"`
	Step    int   `json:"step"`
	Upgrade []int `json:"upgrade"`
	MaxTier int   `json:"max_tier"`
	FromAct int   `json:"upgrade_act"`
}

var deskPrices = DeskPrices{Base: DeskBasePrice, Step: DeskStepPrice, Upgrade: []int{deskTier2Price, deskTier3Price}, MaxTier: MaxDeskTier, FromAct: DeskUpgradeAct}

func prepareRun(run *domain.StartupRun) {
	if run == nil {
		return
	}
	ensureDesks(run)
	run.DeskLimit = TeamCap(run.Act)
	run.Load = currentLoad(run)
}

func ensureDesks(run *domain.StartupRun) {
	if run.Desks != nil {
		return
	}
	run.Desks = make([]int, TeamCap(run.Act))
	for i := range run.Desks {
		run.Desks[i] = startDeskTier
	}
}

func NextDeskPrice(run *domain.StartupRun) int {
	return DeskBasePrice + DeskStepPrice*max(0, len(run.Desks)-StartingDesks)
}

func DeskUpgradePrice(tier int) int {
	if tier == 1 {
		return deskTier2Price
	}
	return deskTier3Price
}

func BuyDesk(run *domain.StartupRun) error {
	if run.Stage != domain.StartupStageHub {
		return domain.ErrStartupWrongStage
	}
	ensureDesks(run)
	if len(run.Desks) >= TeamCap(run.Act) {
		return domain.ErrStartupDeskLimit
	}
	price := NextDeskPrice(run)
	if run.Money < price {
		return domain.ErrStartupNoFunds
	}
	run.Money -= price
	run.Desks = append(run.Desks, startDeskTier)
	return nil
}

func UpgradeDesk(run *domain.StartupRun, index int) error {
	if run.Stage != domain.StartupStageHub {
		return domain.ErrStartupWrongStage
	}
	ensureDesks(run)
	if run.Act < DeskUpgradeAct {
		return domain.ErrStartupDeskLimit
	}
	if index < 0 || index >= len(run.Desks) || run.Desks[index] >= MaxDeskTier {
		return domain.ErrStartupInvalidChoice
	}
	price := DeskUpgradePrice(run.Desks[index])
	if run.Money < price {
		return domain.ErrStartupNoFunds
	}
	run.Money -= price
	run.Desks[index]++
	return nil
}

func deskBonus(run *domain.StartupRun) map[string]int {
	tiers := append([]int(nil), run.Desks...)
	sort.Sort(sort.Reverse(sort.IntSlice(tiers)))
	bonus := map[string]int{}
	for i, d := range run.Staff {
		if i < len(tiers) {
			bonus[d.ID] = (tiers[i] - startDeskTier) * DeskTierBonus
		}
	}
	return bonus
}

func bestStat(s [4]int) int {
	best := 0
	for i := range s {
		if s[i] > s[best] {
			best = i
		}
	}
	return best
}
