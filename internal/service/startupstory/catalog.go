package startupstory

import (
	"math"

	"gofiber-baro/internal/domain"
)

const (
	StartingMoney   = 10000
	ProjectsPerRun  = 9
	ProjectDuration = 30
	BossDuration    = 90
	FounderOffers   = 3
	CandidateOffers = 3
	ReviewScale     = 0.6
	MoneyPerPoint   = 15
	FansPerPoint    = 2
	SalaryRate      = 150
	MinDurationSecs = 15
	BossBonusMoney  = 2000
	BossBonusFans   = 200
	BossWinBonus    = 2000
	BossPassBonus   = 500
	BugPenalty      = 0.3
)

func actScale(act int) float64 {
	switch act {
	case 2:
		return 2.1
	case 3:
		return 3.4
	default:
		return 1
	}
}

const (
	BossDemoDay      = "demo-day"
	BossRequirements = "changing-requirements"
	BossOutage       = "outage-3am"
	BossIPO          = "ipo-pitch"
)

var bossPool = []string{BossDemoDay, BossRequirements, BossOutage}

var outageWeights = [4]float64{0.1, 0.2, 0.1, 0.6}

func bossThreshold(act int) int {
	switch act {
	case 2:
		return 26
	case 3:
		return 36
	default:
		return 18
	}
}

func isBossIndex(projectIndex int) bool {
	return projectIndex%3 == 2
}

func bossForRun(projectIndex int, order []string) string {
	act := actFor(projectIndex)
	if act >= 3 {
		return BossIPO
	}
	if act-1 < len(order) {
		return order[act-1]
	}
	return BossDemoDay
}

const (
	TraitNightOwl  = "night_owl"
	TraitTabs      = "tabs_zealot"
	TraitTenX      = "tenx_dev"
	TraitMeeting   = "meeting_lover"
	TraitSOSurfer  = "so_surfer"
	TraitPixelPerf = "pixel_perfectionist"
)

var traits = []string{TraitNightOwl, TraitTabs, TraitTenX, TraitMeeting, TraitSOSurfer, TraitPixelPerf}

func TeamCap(act int) int {
	switch act {
	case 2:
		return 4
	case 3:
		return 6
	default:
		return 2
	}
}

func baseDurationSecs(act int) int {
	switch act {
	case 2:
		return 45
	case 3:
		return 60
	default:
		return ProjectDuration
	}
}

func statMax(act int) int {
	if act < 1 {
		act = 1
	}
	return 3 + 2*(act-1)
}

func actFor(projectIndex int) int {
	act := 1 + projectIndex/3
	if act > 3 {
		act = 3
	}
	return act
}

func salaryFor(fe, be, design, debug int, trait string) int {
	salary := SalaryRate * (fe + be + design + debug)
	if trait == TraitTenX {
		salary *= 2
	}
	return salary
}

var founders = []domain.StartupDev{
	{Title: "Fullstack Hustler", Role: RolePM, Sprite: "hustler", Perk: "Does a bit of everything", Frontend: 3, Backend: 3, Design: 3, Debug: 3},
	{Title: "Design Nerd", Role: RoleDesigner, Sprite: "designer", Perk: "Pixels over everything", Frontend: 3, Backend: 1, Design: 6, Debug: 2},
	{Title: "Backend Wizard", Role: RoleBE, Sprite: "wizard", Perk: "Speaks fluent SQL", Frontend: 1, Backend: 6, Design: 1, Debug: 4},
	{Title: "Bootcamp Grad", Role: RoleFE, Sprite: "grad", Perk: "Fresh from Generation, hungry to learn", Frontend: 4, Backend: 3, Design: 2, Debug: 2},
}

var founderNames = []string{"Ploy", "Nat", "Kan", "Mint", "Tee", "Bank", "Fah", "Aom", "Alex", "Sam", "Maya", "Leo", "Noah", "Yui", "Arjun", "Sofia"}

var unlockableFounders = []domain.StartupDev{
	{Title: "Ex-FAANG Refugee", Role: RoleSA, Sprite: "faang", Perk: "Knows where the bodies are buried", Frontend: 4, Backend: 5, Design: 3, Debug: 4},
	{Title: "Genmate Legend", Role: RolePO, Sprite: "legend", Perk: "Your batch's finest", Frontend: 5, Backend: 4, Design: 4, Debug: 4},
}

var unlockableItems = []StartupItem{
	{ID: "copilot-subscription", Name: "Copilot Subscription", Icon: "🤖", Rarity: ItemRarityRare, Desc: "+2 team Backend, +1 Debug", BE: 2, Debug: 1, PowerMult: 1, DurationMult: 1},
	{ID: "standing-desk", Name: "Standing Desk", Icon: "🪑", Rarity: ItemRarityCommon, Desc: "+1 Frontend, +1 Design", FE: 1, Design: 1, PowerMult: 1, DurationMult: 1},
}

type UnlockInfo struct {
	Fame int    `json:"fame"`
	Kind string `json:"kind"`
	ID   string `json:"id"`
	Name string `json:"name"`
}

var unlockTable = []UnlockInfo{
	{Fame: 20, Kind: "founder", ID: "Ex-FAANG Refugee", Name: "Ex-FAANG Refugee"},
	{Fame: 50, Kind: "item", ID: "copilot-subscription", Name: "Copilot Subscription"},
	{Fame: 50, Kind: "item", ID: "standing-desk", Name: "Standing Desk"},
	{Fame: 100, Kind: "founder", ID: "Genmate Legend", Name: "Genmate Legend"},
	{Fame: 200, Kind: "skin", ID: "rooftop-bangkok", Name: "Rooftop Bangkok"},
}

func fameGainFor(score int) int {
	gain := int(math.Floor(float64(score) / 100))
	if gain < 1 {
		gain = 1
	}
	return gain
}

func unlocksFor(fame int) (founderTitles, itemIDs []string, skin string) {
	for _, u := range unlockTable {
		if fame < u.Fame {
			continue
		}
		switch u.Kind {
		case "founder":
			founderTitles = append(founderTitles, u.ID)
		case "item":
			itemIDs = append(itemIDs, u.ID)
		case "skin":
			skin = u.ID
		}
	}
	return founderTitles, itemIDs, skin
}

func founderPool(extraTitles []string) []domain.StartupDev {
	pool := append([]domain.StartupDev{}, founders...)
	for _, title := range extraTitles {
		for _, f := range unlockableFounders {
			if f.Title == title {
				pool = append(pool, f)
			}
		}
	}
	return pool
}

func itemPool(extraIDs []string) []StartupItem {
	pool := append([]StartupItem{}, itemCatalog...)
	for _, id := range extraIDs {
		for _, it := range unlockableItems {
			if it.ID == id {
				pool = append(pool, it)
			}
		}
	}
	return pool
}

type productType struct {
	Name    string
	Weights [4]float64 // frontend, backend, design, debug
	Great   []string
	Meh     []string
}

var productTypes = []productType{
	{Name: "Web App", Weights: [4]float64{0.4, 0.3, 0.2, 0.1}, Great: []string{"Productivity", "Education"}, Meh: []string{"Travel"}},
	{Name: "Mobile App", Weights: [4]float64{0.4, 0.2, 0.3, 0.1}, Great: []string{"Food Delivery", "Health", "Social"}, Meh: []string{"Productivity"}},
	{Name: "Game", Weights: [4]float64{0.3, 0.1, 0.4, 0.2}, Great: []string{"Thai Culture", "Social"}, Meh: []string{"Fintech", "Health"}},
	{Name: "AI Chatbot", Weights: [4]float64{0.2, 0.4, 0.1, 0.3}, Great: []string{"Education", "Health", "Travel"}, Meh: []string{"Thai Culture"}},
	{Name: "API/SaaS", Weights: [4]float64{0.1, 0.5, 0.1, 0.3}, Great: []string{"Fintech", "Productivity"}, Meh: []string{"Social", "Thai Culture"}},
	{Name: "Browser Extension", Weights: [4]float64{0.4, 0.2, 0.2, 0.2}, Great: []string{"Productivity"}, Meh: []string{"Food Delivery", "Travel"}},
}

var themes = []string{"Food Delivery", "Fintech", "Education", "Health", "Thai Culture", "Social", "Productivity", "Travel"}

var comboMultipliers = map[string]float64{"great": 1.4, "good": 1.1, "meh": 0.8}

const (
	MarketHotMult  = 1.5
	MarketColdMult = 0.6
	MarketHotCount = 2
)

func ComboKey(typeName, theme string) string {
	return typeName + "|" + theme
}

const (
	ItemRarityCommon    = "common"
	ItemRarityRare      = "rare"
	ItemRarityLegendary = "legendary"
	ItemRarityCursed    = "cursed"
)

type StartupItem struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Icon         string  `json:"icon"`
	Rarity       string  `json:"rarity"`
	Desc         string  `json:"desc"`
	FE           int     `json:"fe"`
	BE           int     `json:"be"`
	Design       int     `json:"design"`
	Debug        int     `json:"debug"`
	AllStats     int     `json:"all_stats"`
	PowerMult    float64 `json:"power_mult"`
	Bugs         int     `json:"bugs"`
	DurationMult float64 `json:"duration_mult"`
	Investor     int     `json:"investor"`
	DevCommunity int     `json:"dev_community"`
}

var itemCatalog = []StartupItem{
	{ID: "keyboard", Name: "Mechanical Keyboard", Icon: "⌨️", Rarity: ItemRarityCommon, Desc: "+2 team Frontend", FE: 2, PowerMult: 1, DurationMult: 1},
	{ID: "rubber-duck", Name: "Rubber Duck", Icon: "🦆", Rarity: ItemRarityCommon, Desc: "−2 bugs per ship", Bugs: -2, PowerMult: 1, DurationMult: 1},
	{ID: "so-tab", Name: "Stack Overflow Tab", Icon: "📑", Rarity: ItemRarityCommon, Desc: "+2 team Backend", BE: 2, PowerMult: 1, DurationMult: 1},
	{ID: "figma-pro", Name: "Figma Pro", Icon: "🎨", Rarity: ItemRarityCommon, Desc: "+2 team Design", Design: 2, PowerMult: 1, DurationMult: 1},
	{ID: "energy-drink", Name: "Energy Drink", Icon: "⚡", Rarity: ItemRarityRare, Desc: "−20% duration, +1 bug", Bugs: 1, PowerMult: 1, DurationMult: 0.8},
	{ID: "dark-mode", Name: "Dark Mode", Icon: "🌙", Rarity: ItemRarityRare, Desc: "+1 Dev Community review", DevCommunity: 1, PowerMult: 1, DurationMult: 1},
	{ID: "pitch-deck", Name: "Pitch Deck", Icon: "📊", Rarity: ItemRarityLegendary, Desc: "+2 Investor review", Investor: 2, PowerMult: 1, DurationMult: 1},
	{ID: "legacy-codebase", Name: "Legacy Codebase", Icon: "☠️", Rarity: ItemRarityCursed, Desc: "×1.3 power, +6 bugs", Bugs: 6, PowerMult: 1.3, DurationMult: 1},
	{ID: "crunch-culture", Name: "Crunch Culture", Icon: "☠️", Rarity: ItemRarityCursed, Desc: "−40% duration, −1 all stats", AllStats: -1, PowerMult: 1, DurationMult: 0.6},
}

var rarityWeights = []struct {
	Rarity string
	Weight int
}{
	{ItemRarityCommon, 60},
	{ItemRarityRare, 30},
	{ItemRarityLegendary, 8},
	{ItemRarityCursed, 2},
}

func findItem(id string) (StartupItem, bool) {
	for _, it := range itemCatalog {
		if it.ID == id {
			return it, true
		}
	}
	for _, it := range unlockableItems {
		if it.ID == id {
			return it, true
		}
	}
	return StartupItem{}, false
}

func marketMult(market domain.StartupMarket, theme string) float64 {
	for _, h := range market.Hot {
		if h == theme {
			return MarketHotMult
		}
	}
	for _, c := range market.Cold {
		if c == theme {
			return MarketColdMult
		}
	}
	return 1.0
}

type reviewer struct {
	Name  string
	Favor int // index into stats, -1 = none
	Lines [3][]string
}

var reviewers = []reviewer{
	{Name: "Tech Lead", Favor: 1, Lines: [3][]string{
		{"Who wrote this query? Please, I'm begging.", "It compiles. That's all I'll say."},
		{"Solid enough. Tests would be nice though.", "Clean-ish. I've seen worse in prod."},
		{"Beautiful architecture. I'm stealing this.", "Zero warnings?! สุดยอด."},
	}},
	{Name: "Users", Favor: 2, Lines: [3][]string{
		{"Where is the back button??", "I uninstalled it before lunch."},
		{"Pretty useful, I use it on the BTS.", "Nice! Wish it had dark mode."},
		{"I told my whole family. Even my อาม่า uses it!", "5 stars, would download again."},
	}},
	{Name: "Investor", Favor: -1, Lines: [3][]string{
		{"Interesting... (checks phone)", "Have you considered pivoting to crypto?"},
		{"Promising traction. Let's circle back.", "I see a path to profitability. Maybe."},
		{"Take my money. All of it.", "This is the next unicorn 🦄"},
	}},
	{Name: "Dev Community", Favor: 0, Lines: [3][]string{
		{"Ratio'd on the forums, sorry.", "Someone already built this in 2012."},
		{"Decent! Starred on GitHub.", "Cool side project energy."},
		{"Trending #1 on the dev forums 🔥", "The README alone deserves an award."},
	}},
}

func findType(name string) (productType, bool) {
	for _, t := range productTypes {
		if t.Name == name {
			return t, true
		}
	}
	return productType{}, false
}

func validTheme(name string) bool {
	for _, t := range themes {
		if t == name {
			return true
		}
	}
	return false
}

func comboFor(t productType, theme string) string {
	for _, g := range t.Great {
		if g == theme {
			return "great"
		}
	}
	for _, m := range t.Meh {
		if m == theme {
			return "meh"
		}
	}
	return "good"
}
