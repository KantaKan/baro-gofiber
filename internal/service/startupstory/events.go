package startupstory

import (
	"math"

	"gofiber-baro/internal/domain"
)

const eventChance = 35

type WorldEventInfo struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Desc        string  `json:"desc"`
	PowerMult   float64 `json:"-"`
	Bugs        int     `json:"-"`
	DurMult     float64 `json:"-"`
	HireMult    float64 `json:"-"`
	MoneyMult   float64 `json:"-"`
	FansMult    float64 `json:"-"`
	AIProducts  bool    `json:"-"`
	CryptoTheme bool    `json:"-"`
}

var worldEventCatalog = []WorldEventInfo{
	{ID: "ai-hype", Name: "AI Hype Wave 🤖", Desc: "AI Chatbots get +50% power. Investors clapping at autocomplete.", AIProducts: true},
	{ID: "crypto-winter", Name: "Crypto Winter 🥶", Desc: "Crypto projects earn 40% less. Winter is coming.", CryptoTheme: true},
	{ID: "layoff-season", Name: "Layoff Season 🧑‍💻", Desc: "Hiring is cheap this act: −40% salary.", HireMult: 0.6},
	{ID: "hackathon", Name: "Hackathon Week 🏃", Desc: "Ships 25% faster, sloppier: +2 bugs.", DurMult: 0.75, Bugs: 2},
	{ID: "songkran", Name: "Songkran Holiday 💦", Desc: "Half the team is at the water fight: −25% power.", PowerMult: 0.75},
	{ID: "rainy-season", Name: "Rainy Season Traffic 🌧️", Desc: "Bangkok traffic: +30% build time.", DurMult: 1.3},
	{ID: "viral-tiktok", Name: "A TikTok Went Viral 🎵", Desc: "+50% fans this act. The algorithm loves you.", FansMult: 1.5},
	{ID: "sponsor-week", Name: "Tech Sponsor Week 🎪", Desc: "+25% money this act. Swag budget unlocked.", MoneyMult: 1.25},
}

var worldEventPool = []string{
	"ai-hype", "crypto-winter", "layoff-season", "hackathon",
	"songkran", "rainy-season", "viral-tiktok", "sponsor-week",
}

type eventEffect struct {
	Money     int
	Fans      int
	Burnout   int
	NextBugs  int
	NextPower float64
}

type choiceOption struct {
	Label    string
	Effect   eventEffect
	Odds     int
	Risk     eventEffect
	RiskNote string
	SafeNote string
}

type ChoiceEventInfo struct {
	ID      string
	Title   string
	Options [2]choiceOption
}

var choiceEventCatalog = []ChoiceEventInfo{
	{ID: "blockchain", Title: "Client Wants It On Blockchain", Options: [2]choiceOption{
		{Label: "Sure, we'll blockchain it", Effect: eventEffect{Money: 5000, Fans: -400}},
		{Label: "Explain why not", Effect: eventEffect{Fans: 150}},
	}},
	{ID: "friday-deploy", Title: "Push to Prod on Friday?", Options: [2]choiceOption{
		{Label: "Do it. YOLO 😈", Effect: eventEffect{Money: 3000, Burnout: 10}},
		{Label: "Wait for Monday", Effect: eventEffect{Burnout: -8}},
	}},
	{ID: "code-review", Title: "Senior Dev Offers a Code Review", Options: [2]choiceOption{
		{Label: "Accept, buy them coffee", Effect: eventEffect{Money: -400, Burnout: -12}},
		{Label: "We're gods, no thanks", Effect: eventEffect{Burnout: 8}},
	}},
	{ID: "intern-db", Title: "The Intern Deleted the Prod DB 😱", Options: [2]choiceOption{
		{Label: "Restore from backup", Effect: eventEffect{Money: -800}},
		{Label: "Blame the intern publicly", Effect: eventEffect{Fans: -300}},
	}},
	{ID: "youtuber", Title: "A YouTuber Wants to Review Your App", Options: [2]choiceOption{
		{Label: "Send the build", Effect: eventEffect{Money: -600, Fans: 900}},
		{Label: "Too nervous, skip", Effect: eventEffect{Burnout: -8}},
	}},
	{ID: "team-lunch", Title: "Team Lunch at the Mall", Options: [2]choiceOption{
		{Label: "Go all in 🍜", Effect: eventEffect{Money: -900, Burnout: -18}},
		{Label: "Instant noodles", Effect: eventEffect{Money: -100, Burnout: -4}},
	}},
	{ID: "grant", Title: "Government Digital Grant", Options: [2]choiceOption{
		{Label: "Grind the paperwork", Effect: eventEffect{Money: 4000, Burnout: 10}},
		{Label: "Skip it, ship instead", Effect: eventEffect{Fans: 250}},
	}},
	{ID: "ads", Title: "Ad Budget Request", Options: [2]choiceOption{
		{Label: "Buy the ads 📣", Effect: eventEffect{Money: -1200, Fans: 700}},
		{Label: "Organic only", Effect: eventEffect{Fans: 150}},
	}},
	{ID: "oss-pr", Title: "A Stranger Sent a Big PR", Options: [2]choiceOption{
		{Label: "Merge it 🙌", Effect: eventEffect{Money: -300, Fans: 400}},
		{Label: "Close it, too risky", Effect: eventEffect{Burnout: 6}},
	}},
	{ID: "office-dog", Title: "Office Dog Adoption Day 🐶", Options: [2]choiceOption{
		{Label: "Adopt the good boy", Effect: eventEffect{Money: -700, Fans: 200, Burnout: -15}},
		{Label: "Not now", Effect: eventEffect{Burnout: -3}},
	}},
}

var memeEvents = []ChoiceEventInfo{
	{ID: "dns", Title: "It's Always DNS", Options: [2]choiceOption{
		{Label: "Hire an expert", Effect: eventEffect{Money: -800}},
		{Label: "Debug it ourselves", Effect: eventEffect{Burnout: 10}},
	}},
	{ID: "leaked-env", Title: "Leaked .env on GitHub", Options: [2]choiceOption{
		{Label: "Rotate every key", Effect: eventEffect{Money: -600}},
		{Label: "Nobody saw it (30%: someone did)", Odds: 30, Risk: eventEffect{Money: -3000}, RiskNote: "Someone saw the .env. The cloud bill says crypto miners did too.", SafeNote: "Nobody saw the .env. This time."},
	}},
	{ID: "rust-rewrite", Title: "Tech Twitter Says Rewrite It In Rust", Options: [2]choiceOption{
		{Label: "Rewrite it in Rust", Effect: eventEffect{Fans: 300, NextPower: -0.15}},
		{Label: "No.", Effect: eventEffect{Fans: 100}},
	}},
	{ID: "gpu-bill", Title: "Someone Left the GPU Instance On", Options: [2]choiceOption{
		{Label: "Pay the bill", Effect: eventEffect{Money: -1500}},
		{Label: "Beg support (50%: refund)", Effect: eventEffect{Money: -1500}, Odds: 50, Risk: eventEffect{Money: 1500}, RiskNote: "Support refunded the GPU bill. Legends.", SafeNote: "Support said no. The GPU bill stands."},
	}},
	{ID: "left-pad", Title: "A Tiny Dependency Vanished", Options: [2]choiceOption{
		{Label: "Vendor a copy", Effect: eventEffect{Money: -400}},
		{Label: "Write it ourselves", Effect: eventEffect{Burnout: 8}},
	}},
	{ID: "dst-bug", Title: "Daylight Saving Time Bug", Options: [2]choiceOption{
		{Label: "Hotfix tonight", Effect: eventEffect{Burnout: 10}},
		{Label: "Ship it anyway (+2 bugs)", Effect: eventEffect{NextBugs: 2}},
	}},
	{ID: "so-down", Title: "Stack Overflow Is Down", Options: [2]choiceOption{
		{Label: "Read the actual docs", Effect: eventEffect{Burnout: 8, NextPower: 0.1}},
		{Label: "Everyone go home", Effect: eventEffect{Burnout: -10}},
	}},
	{ID: "copilot-api", Title: "Copilot Invented an API", Options: [2]choiceOption{
		{Label: "Rewrite it by hand", Effect: eventEffect{Burnout: 8}},
		{Label: "Trust it (50%: +3 bugs)", Odds: 50, Risk: eventEffect{NextBugs: 3}, RiskNote: "The API Copilot invented does not exist. +3 bugs next project.", SafeNote: "Somehow the made-up API worked."},
	}},
	{ID: "merge-marathon", Title: "Merge Conflict Marathon", Options: [2]choiceOption{
		{Label: "Pair it out", Effect: eventEffect{Burnout: 10}},
		{Label: "git push --force (40%: +4 bugs)", Odds: 40, Risk: eventEffect{NextBugs: 4}, RiskNote: "The force push ate someone's work. +4 bugs next project.", SafeNote: "The force push went through clean. Nobody will ever know."},
	}},
	{ID: "center-div", Title: "Nobody Can Center the Div", Options: [2]choiceOption{
		{Label: "Flexbox", Effect: eventEffect{Fans: 100}},
		{Label: "Table layout (+1 bug, retro fans)", Effect: eventEffect{Fans: 250, NextBugs: 1}},
	}},
	{ID: "npm-audit", Title: "npm audit: 900 Vulnerabilities", Options: [2]choiceOption{
		{Label: "npm audit fix --force (50%: chaos)", Effect: eventEffect{Fans: 100}, Odds: 50, Risk: eventEffect{NextBugs: 4}, RiskNote: "npm audit fix --force upgraded React three majors. +4 bugs next project.", SafeNote: "npm audit fix --force actually fixed things."},
		{Label: "Close the terminal", Effect: eventEffect{}},
	}},
	{ID: "recruiter", Title: "Recruiters Are Messaging Your Team", Options: [2]choiceOption{
		{Label: "Counter-offer bonuses", Effect: eventEffect{Money: -1000, Burnout: -10}},
		{Label: "Ignore it", Effect: eventEffect{Burnout: 6}},
	}},
}

var choiceEventPool = []string{
	"blockchain", "friday-deploy", "code-review", "intern-db", "youtuber",
	"team-lunch", "grant", "ads", "oss-pr", "office-dog",
	"dns", "leaked-env", "rust-rewrite", "gpu-bill", "left-pad", "dst-bug",
	"so-down", "copilot-api", "merge-marathon", "center-div", "npm-audit", "recruiter",
}

func findWorldEvent(id string) (WorldEventInfo, bool) {
	for _, w := range worldEventCatalog {
		if w.ID == id {
			return w, true
		}
	}
	return WorldEventInfo{}, false
}

func findChoiceEvent(id string) (ChoiceEventInfo, bool) {
	for _, e := range append(choiceEventCatalog[:len(choiceEventCatalog):len(choiceEventCatalog)], memeEvents...) {
		if e.ID == id {
			return e, true
		}
	}
	return ChoiceEventInfo{}, false
}

func onNewAct(run *domain.StartupRun) {
	if run.Status != domain.StartupStatusActive {
		return
	}
	r := rngFor(run)
	run.WorldEvent = worldEventPool[r.IntN(len(worldEventPool))]
}

func worldDurationMult(run *domain.StartupRun) float64 {
	w, ok := findWorldEvent(run.WorldEvent)
	if !ok {
		return 1
	}
	return nonZero(w.DurMult)
}

func hireCost(run *domain.StartupRun, salary int) int {
	w, ok := findWorldEvent(run.WorldEvent)
	if !ok || w.HireMult == 0 {
		return salary
	}
	return int(math.Round(float64(salary) * w.HireMult))
}

func applyWorldScoring(sc *scoring) {
	w, ok := findWorldEvent(sc.run.WorldEvent)
	if !ok {
		return
	}
	sc.powerMult *= nonZero(w.PowerMult)
	sc.bugs += w.Bugs
	sc.moneyMult *= nonZero(w.MoneyMult)
	sc.fansMult *= nonZero(w.FansMult)
	if w.AIProducts && sc.run.Project != nil && sc.run.Project.Type == "AI Chatbot" {
		sc.powerMult *= 1.5
	}
	if w.CryptoTheme && sc.run.Project != nil && sc.run.Project.Theme == "Crypto" {
		sc.moneyMult *= 0.6
		sc.fansMult *= 0.6
	}
}

func queueEvent(run *domain.StartupRun) {
	if run.Status != domain.StartupStatusActive || run.ResumeStage != "" || run.PendingEvent != nil {
		return
	}
	r := rngFor(run)
	if r.IntN(100) >= eventChance {
		return
	}
	fresh := unseenEvents(run)
	id := fresh[r.IntN(len(fresh))]
	ev, ok := findChoiceEvent(id)
	if !ok {
		return
	}
	run.SeenEvents = append(run.SeenEvents, id)
	run.PendingEvent = &domain.StartupPendingEvent{
		ID:      ev.ID,
		Title:   ev.Title,
		Options: []string{ev.Options[0].Label, ev.Options[1].Label},
	}
	interrupt(run, domain.StartupStageEvent)
}

func PickEvent(run *domain.StartupRun, index int) error {
	if run.Stage != domain.StartupStageEvent || run.PendingEvent == nil {
		return domain.ErrStartupWrongStage
	}
	if index < 0 || index >= len(run.PendingEvent.Options) {
		return domain.ErrStartupInvalidChoice
	}
	if ev, ok := findChoiceEvent(run.PendingEvent.ID); ok && index < len(ev.Options) {
		opt := ev.Options[index]
		applyEventEffect(run, opt.Effect)
		if opt.Odds > 0 {
			if rngFor(run).IntN(100) < opt.Odds {
				applyEventEffect(run, opt.Risk)
				addLog(run, opt.RiskNote)
			} else {
				addLog(run, opt.SafeNote)
			}
		}
	}
	run.PendingEvent = nil
	resume(run)
	return nil
}

func unseenEvents(run *domain.StartupRun) []string {
	seen := map[string]bool{}
	for _, id := range run.SeenEvents {
		seen[id] = true
	}
	var fresh []string
	for _, id := range choiceEventPool {
		if !seen[id] {
			fresh = append(fresh, id)
		}
	}
	if len(fresh) == 0 {
		run.SeenEvents = nil
		return choiceEventPool
	}
	return fresh
}

func applyEventEffect(run *domain.StartupRun, fx eventEffect) {
	run.Money += fx.Money
	run.Fans = max(0, run.Fans+fx.Fans)
	run.NextBugs += fx.NextBugs
	run.NextPower += fx.NextPower
	if fx.Burnout == 0 {
		return
	}
	for i := range run.Staff {
		run.Staff[i].Burnout = min(BurnoutQuitAt, max(0, run.Staff[i].Burnout+fx.Burnout))
	}
}
