package user

import (
	"errors"
	"fmt"
	"regexp"

	"gofiber-baro/internal/domain"
)

// streak needed per check-in style; 0 = free. "gold" needs a perfect week instead.
var styleStreak = map[string]int{
	"slide": 0, "hold": 0, "ring": 0, "run": 0, "coffee": 0, "red": 0, "cat": 0,
	"bubble": 0, "scratch": 0, "alarm": 0, "slot": 0, "gacha": 0,
	"rocket": 15, "arcade": 20, "dj": 30, "gold": 0,
}

// StyleUnlocked reports whether style exists and whether the learner may use it.
func StyleUnlocked(style string, streak int, perfectWeek bool) (known, unlocked bool) {
	need, ok := styleStreak[style]
	if !ok {
		return false, false
	}
	if style == "gold" {
		return true, perfectWeek
	}
	return true, streak >= need
}

var (
	avatarStyles = map[string]bool{"notionists": true, "lorelei": true, "adventurer": true, "fun-emoji": true, "pixel-art": true, "thumbs": true}
	hexColor     = regexp.MustCompile(`^[0-9a-fA-F]{6}$`)
	// locked avatar options: style -> part -> value -> streak needed
	avatarLocks = map[string]map[string]map[string]int{
		"notionists": {"bodyIcon": {"saturn": 15, "galaxy": 30}},
	}
	ErrAvatarLocked = errors.New("locked: keep your streak going")
)

func AvatarAllowed(a domain.Avatar, streak int) error {
	if !avatarStyles[a.Style] {
		return fmt.Errorf("unknown avatar style %q", a.Style)
	}
	if !hexColor.MatchString(a.Bg) {
		return fmt.Errorf("background must be a 6-digit hex color")
	}
	for style, parts := range a.Options {
		for part, v := range parts {
			if v == nil {
				continue
			}
			if need, ok := avatarLocks[style][part][*v]; ok && streak < need {
				return ErrAvatarLocked
			}
		}
	}
	return nil
}
