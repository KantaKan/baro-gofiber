package startupstory

import (
	"math"
	"math/rand/v2"

	"gofiber-baro/internal/domain"
)

const (
	XPBase       = 10
	XPPerLevel   = 70
	MaxLevel     = 10
	RaisePercent = 10
	PerkOffers   = 3
)

var perkLevels = map[int]bool{3: true, 5: true}

type PerkInfo struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Desc        string             `json:"desc"`
	Bugs        int                `json:"-"`
	PowerMult   float64            `json:"-"`
	DurMult     float64            `json:"-"`
	OutagePower float64            `json:"-"`
	Reviewer    map[string]float64 `json:"-"`
}

var perkCatalog = []PerkInfo{
	{ID: "clean-code", Name: "Clean-code Zealot", Desc: "−1 bug per project. Will lecture you about SOLID.", Bugs: -1},
	{ID: "night-shift", Name: "Night-shift Coder", Desc: "Builds 8% faster. Sleeps at 5am, proudly.", DurMult: 0.92},
	{ID: "demo-whisperer", Name: "Demo Whisperer", Desc: "+1 Investor. Somehow the demo never crashes.", Reviewer: map[string]float64{"Investor": 1}},
	{ID: "so-legend", Name: "Stack Overflow Legend", Desc: "+6% build power. 100k rep, zero friends.", PowerMult: 1.06},
	{ID: "git-blame", Name: "Git Blame Survivor", Desc: "−1 bug, +0.5 Tech Lead. Has seen things.", Bugs: -1, Reviewer: map[string]float64{"Tech Lead": 0.5}},
	{ID: "pixel-wizard", Name: "Pixel Wizard", Desc: "+1 Users. Moves things 1px until it's art.", Reviewer: map[string]float64{"Users": 1}},
	{ID: "arch-btw", Name: "I Use Arch btw", Desc: "+1 Dev Community, −0.5 Users. Will mention it.", Reviewer: map[string]float64{"Dev Community": 1, "Users": -0.5}},
	{ID: "rubber-duck-whisperer", Name: "Rubber Duck Whisperer", Desc: "−2 bugs. The duck does the debugging.", Bugs: -2},
	{ID: "coffee-powered", Name: "Coffee-powered", Desc: "+4% build power. Blood type: Americano.", PowerMult: 1.04},
	{ID: "meeting-ninja", Name: "Meeting Ninja", Desc: "+0.5 Investor, +0.5 Users. Ends meetings early.", Reviewer: map[string]float64{"Investor": 0.5, "Users": 0.5}},
	{ID: "rust-evangelist", Name: "Rust Evangelist", Desc: "+1 Tech Lead, +0.5 Dev Community. Rewrites everything.", Reviewer: map[string]float64{"Tech Lead": 1, "Dev Community": 0.5}},
	{ID: "prod-hotfixer", Name: "3AM Hotfixer", Desc: "+20% power in the 3AM Outage. Lives for it.", OutagePower: 1.2},
	{ID: "ship-it", Name: "Ship-It Energy", Desc: "Builds 10% faster. Tests are optional (they're not).", DurMult: 0.9},
	{ID: "legacy-tamer", Name: "Legacy Tamer", Desc: "−1 bug, +0.3 Tech Lead. Reads COBOL for fun.", Bugs: -1, Reviewer: map[string]float64{"Tech Lead": 0.3}},
}

func nonZero(mult float64) float64 {
	if mult == 0 {
		return 1
	}
	return mult
}

func findPerk(id string) (PerkInfo, bool) {
	for _, p := range perkCatalog {
		if p.ID == id {
			return p, true
		}
	}
	return PerkInfo{}, false
}

func levelOf(d domain.StartupDev) int {
	return max(1, d.Level)
}

func xpToNext(level int) int {
	return XPPerLevel * level
}

func afterShipLevels(run *domain.StartupRun, team []domain.StartupDev, result *domain.StartupResult) {
	onTeam := map[string]bool{}
	for _, d := range team {
		onTeam[d.ID] = true
	}
	fx := effectsOf(run.Items)
	gain := int(math.Round(float64(XPBase+result.Total/2) * float64(100+fx.xpPct) / 100))
	var r *rand.Rand
	for i := range run.Staff {
		d := &run.Staff[i]
		if !onTeam[d.ID] {
			continue
		}
		d.Level = levelOf(*d)
		d.XP += gain
		for d.Level < MaxLevel && d.XP >= xpToNext(d.Level) {
			d.XP -= xpToNext(d.Level)
			d.Level++
			if r == nil {
				r = rngFor(run)
			}
			levelUp(d, r, 1+fx.levelStats)
			if perkLevels[d.Level] {
				run.PerkQueue = append(run.PerkQueue, d.ID)
			}
		}
		if d.Level >= MaxLevel {
			d.XP = 0
		}
		d.XPNext = xpToNext(d.Level)
	}
}

func levelUp(d *domain.StartupDev, r *rand.Rand, focusGain int) {
	stats := []*int{&d.Frontend, &d.Backend, &d.Design, &d.Debug}
	focus := -1
	if info, ok := roleInfo(d.Role); ok {
		focus = info.Focus
	}
	if focus < 0 {
		focus = 0
		for i, s := range stats {
			if *s > *stats[focus] {
				focus = i
			}
		}
	}
	*stats[focus] += focusGain
	if r.IntN(2) == 0 {
		*stats[r.IntN(4)]++
	}
	d.Salary = d.Salary * (100 + RaisePercent) / 100
}

func queuePerk(run *domain.StartupRun) {
	if run.PendingPerk != nil || run.Status != domain.StartupStatusActive {
		return
	}
	for len(run.PerkQueue) > 0 {
		devID := run.PerkQueue[0]
		run.PerkQueue = run.PerkQueue[1:]
		dev := staffByID(run, devID)
		if dev == nil {
			continue
		}
		run.PendingPerk = &domain.StartupPendingPerk{DevID: devID, Offer: rollPerkOffer(run, *dev)}
		interrupt(run, domain.StartupStagePerk)
		return
	}
}

func staffByID(run *domain.StartupRun, id string) *domain.StartupDev {
	for i := range run.Staff {
		if run.Staff[i].ID == id {
			return &run.Staff[i]
		}
	}
	return nil
}

func rollPerkOffer(run *domain.StartupRun, dev domain.StartupDev) []string {
	owned := map[string]bool{}
	for _, p := range dev.Perks {
		owned[p] = true
	}
	var pool []string
	for _, p := range perkCatalog {
		if !owned[p.ID] {
			pool = append(pool, p.ID)
		}
	}
	r := rngFor(run)
	var offer []string
	for _, i := range r.Perm(len(pool)) {
		if len(offer) == PerkOffers {
			break
		}
		offer = append(offer, pool[i])
	}
	return offer
}

func PickPerk(run *domain.StartupRun, index int) error {
	if run.Stage != domain.StartupStagePerk || run.PendingPerk == nil {
		return domain.ErrStartupWrongStage
	}
	if index < 0 || index >= len(run.PendingPerk.Offer) {
		return domain.ErrStartupInvalidChoice
	}
	if dev := staffByID(run, run.PendingPerk.DevID); dev != nil {
		dev.Perks = append(dev.Perks, run.PendingPerk.Offer[index])
	}
	run.PendingPerk = nil
	resume(run)
	queuePerk(run)
	return nil
}

func teamPerks(team []domain.StartupDev) []PerkInfo {
	var out []PerkInfo
	for _, d := range team {
		for _, id := range d.Perks {
			if p, ok := findPerk(id); ok {
				out = append(out, p)
			}
		}
	}
	return out
}

func applyPerks(sc *scoring) {
	outage := sc.run.Project != nil && sc.run.Project.Boss == BossOutage
	for _, p := range teamPerks(sc.team) {
		sc.bugs += p.Bugs
		sc.powerMult *= nonZero(p.PowerMult)
		if outage {
			sc.powerMult *= nonZero(p.OutagePower)
		}
		for name, bias := range p.Reviewer {
			sc.reviewer[name] += bias
		}
	}
}

func perkDurationMult(run *domain.StartupRun, team []domain.StartupDev) float64 {
	mult := 1.0
	for _, p := range teamPerks(team) {
		mult *= nonZero(p.DurMult)
	}
	return mult
}
