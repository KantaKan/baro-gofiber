package startupstory

import (
	"math/rand/v2"
	"sort"
	"strings"

	"gofiber-baro/internal/domain"
)

const PitchCount = 3

var themeEmoji = map[string]string{
	"Food Delivery":  "🛵",
	"Fintech":        "💸",
	"Education":      "🎓",
	"Health":         "💊",
	"Thai Culture":   "🛕",
	"Social":         "💬",
	"Productivity":   "📈",
	"Travel":         "✈️",
	"Crypto":         "🪙",
	"K-pop/Idols":    "🎤",
	"Street Food":    "🍜",
	"Dating":         "💘",
	"Pets":           "🐶",
	"Esports":        "🎮",
	"Government/Tax": "🧾",
}

var memePitchTitles = map[string]string{
	ComboKey("LINE Bot", "Street Food"):           "🍜 Street-food LINE bot",
	ComboKey("Dev Tool/CLI", "Government/Tax"):     "🧾 Tax calculator nobody asked for",
	ComboKey("Mobile App", "Pets"):                 "🐶 Tinder for dogs",
	ComboKey("Web3 dApp", "Crypto"):                "🪙 DAO that does nothing",
	ComboKey("VR Game", "K-pop/Idols"):             "🎤 Front-row idol concert in VR",
}

var pitchTitleTemplates = []string{
	"{emoji} The {theme} {type} nobody asked for",
	"{emoji} Finally, a {type} that gets {theme}",
	"{emoji} {type} × {theme}: chaos edition",
	"{emoji} {theme} {type}, but it slaps",
	"{emoji} Speedrun {theme} with this {type}",
	"{emoji} {type} for {theme} enthusiasts",
	"{emoji} VC already said no to this {type}",
	"{emoji} A {type} powered by pure {theme} vibes",
}

func pitchPool(run *domain.StartupRun) []productType {
	out := make([]productType, 0, len(productTypes))
	for _, t := range productTypes {
		if t.OSSOnly && !run.OSS {
			continue
		}
		out = append(out, t)
	}
	return out
}

func pitchTitle(r *rand.Rand, typeName, theme string) string {
	if special, ok := memePitchTitles[ComboKey(typeName, theme)]; ok {
		return special
	}
	emoji := themeEmoji[theme]
	if emoji == "" {
		emoji = "💡"
	}
	tpl := pitchTitleTemplates[r.IntN(len(pitchTitleTemplates))]
	return strings.NewReplacer(
		"{emoji}", emoji,
		"{type}", typeName,
		"{theme}", theme,
	).Replace(tpl)
}

func teamPower(run *domain.StartupRun, t productType, theme string) float64 {
	power := 0.0
	for _, d := range run.Staff {
		s := stats(d)
		for i := range s {
			power += float64(s[i]) * t.Weights[i]
		}
	}
	return power * comboMultipliers[comboFor(t, theme)] * marketMult(run.Market, theme)
}

type pitchPair struct {
	typ   productType
	theme string
	power float64
}

func refreshPitches(run *domain.StartupRun) {
	r := rngFor(run)
	types := pitchPool(run)
	pairs := make([]pitchPair, 0, len(types)*len(themes))
	for _, t := range types {
		for _, th := range themes {
			pairs = append(pairs, pitchPair{typ: t, theme: th, power: teamPower(run, t, th)})
		}
	}
	sort.SliceStable(pairs, func(i, j int) bool { return pairs[i].power > pairs[j].power })
	prev := map[string]bool{}
	for _, p := range run.Pitches {
		prev[ComboKey(p.Type, p.Theme)] = true
	}
	chosen := make([]pitchPair, 0, PitchCount)
	seen := map[string]bool{}
	add := func(p pitchPair) {
		key := ComboKey(p.typ.Name, p.theme)
		if seen[key] || len(chosen) == PitchCount {
			return
		}
		seen[key] = true
		chosen = append(chosen, p)
	}
	for i := 0; i < 64 && len(chosen) == 0; i++ {
		p := pairs[r.IntN(len(pairs))]
		if !prev[ComboKey(p.typ.Name, p.theme)] {
			add(p)
		}
	}
	for _, p := range pairs {
		add(p)
		if len(chosen) == PitchCount {
			break
		}
	}
	pitches := make([]domain.StartupPitch, 0, len(chosen))
	for _, p := range chosen {
		pitches = append(pitches, domain.StartupPitch{
			Type:  p.typ.Name,
			Theme: p.theme,
			Title: pitchTitle(r, p.typ.Name, p.theme),
		})
	}
	run.Pitches = pitches
}
