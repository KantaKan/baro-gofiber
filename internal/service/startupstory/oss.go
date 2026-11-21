package startupstory

import "gofiber-baro/internal/domain"

const (
	OSSUnlockShips = 3
	OSSMoneyMult   = 0.6
	OSSFansMult    = 1.6
)

var ossProducts = map[string]bool{"Dev Tool/CLI": true, "Open-Source Library": true}

var ossFounder = domain.StartupDev{
	Title: "Open Source Maintainer", Role: RoleSA, Sprite: "octo",
	Perk:     "Maintains 40 repos for free. Send help (and sponsors)",
	Frontend: 4, Backend: 5, Design: 2, Debug: 5,
}

var ossReviewers = []reviewer{
	{Name: "Maintainers", Alias: "Tech Lead", Favor: 3, Lines: [3][]string{
		{"PR closed: 'please read CONTRIBUTING.md'", "This breaks 14 downstream packages."},
		{"LGTM with nits. 37 nits.", "Merged. Don't make it weird."},
		{"Cleanest PR I've seen since 2009", "Adding you to the core team. No take-backs."},
	}},
	{Name: "Contributors", Alias: "Users", Favor: 1, Lines: [3][]string{
		{"The README says 'TODO: docs'", "Opened issue #4012: 'does not work'"},
		{"Starred it, will read it someday", "Fixed a typo, counting it as a contribution"},
		{"I'm putting this on my resume", "Forked, starred, told my whole Discord"},
	}},
	{Name: "Hacker News", Alias: "Dev Community", Favor: 0, Lines: [3][]string{
		{"'Why not just use a bash script?' (top comment)", "Flagged: 'yet another framework'"},
		{"Mildly interesting. 47 points.", "Someone rewrote it in Rust in the comments"},
		{"#1 on the front page all day", "Even the Rust people are impressed"},
	}},
	{Name: "Big Tech", Alias: "Investor", Favor: -1, Lines: [3][]string{
		{"We'll just build our own version. Thanks!", "Our lawyers would like a word."},
		{"Interesting. We might sponsor $5/month.", "Can you add SSO? For free?"},
		{"We want to sponsor you (and maybe acqui-hire)", "Our whole stack runs on this now"},
	}},
}

func IsOSSProduct(typeName string) bool {
	return ossProducts[typeName]
}

func ossFounderUnlocked(studio *domain.StartupStudio) bool {
	return studio != nil && studio.OSSShips >= OSSUnlockShips
}

func founderPoolFor(studio *domain.StartupStudio) []domain.StartupDev {
	pool := founderPool(studio.UnlockedFounders)
	if ossFounderUnlocked(studio) {
		pool = append(pool, ossFounder)
	}
	return pool
}

func onFounderPicked(run *domain.StartupRun) {
	run.OSS = run.Founder == ossFounder.Title
}

func applyOSS(sc *scoring) {
	if !sc.run.OSS {
		return
	}
	sc.reviewers = ossReviewers
	sc.moneyMult *= OSSMoneyMult
	sc.fansMult *= OSSFansMult
}
