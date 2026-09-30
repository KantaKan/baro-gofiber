package startupstory

import "gofiber-baro/internal/domain"

const (
	StartingMoney   = 10000
	ProjectsPerRun  = 3
	ProjectDuration = 30
	FounderOffers   = 3
	ReviewScale     = 1.6
	MoneyPerPoint   = 15
	FansPerPoint    = 2
)

var founders = []domain.StartupDev{
	{Title: "Fullstack Hustler", Sprite: "hustler", Perk: "Does a bit of everything", Frontend: 3, Backend: 3, Design: 3, Debug: 3},
	{Title: "Design Nerd", Sprite: "designer", Perk: "Pixels over everything", Frontend: 3, Backend: 1, Design: 6, Debug: 2},
	{Title: "Backend Wizard", Sprite: "wizard", Perk: "Speaks fluent SQL", Frontend: 1, Backend: 6, Design: 1, Debug: 4},
	{Title: "Bootcamp Grad", Sprite: "grad", Perk: "Fresh from Generation, hungry to learn", Frontend: 4, Backend: 3, Design: 2, Debug: 2},
}

var founderNames = []string{"Ploy", "Nat", "Kan", "Mint", "Tee", "Bank", "Fah", "Aom", "Alex", "Sam", "Maya", "Leo", "Noah", "Yui", "Arjun", "Sofia"}

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
