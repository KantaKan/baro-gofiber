package user

import (
	"testing"

	"gofiber-baro/internal/domain"
)

func TestStyleUnlocked(t *testing.T) {
	cases := []struct {
		style           string
		streak          int
		perfect         bool
		champion        bool
		known, unlocked bool
	}{
		{"slide", 0, false, false, true, true},
		{"gacha", 0, false, false, true, true},
		{"rocket", 14, false, false, true, false},
		{"rocket", 15, false, false, true, true},
		{"arcade", 19, false, false, true, false},
		{"arcade", 20, false, false, true, true},
		{"dj", 29, false, false, true, false},
		{"dj", 30, false, false, true, true},
		{"gold", 99, false, false, true, false},
		{"gold", 0, true, false, true, true},
		{"victory", 99, true, false, true, false},
		{"victory", 0, false, true, true, true},
		{"victory-lap", 99, true, true, false, false},
	}
	for _, c := range cases {
		k, u := StyleUnlocked(c.style, c.streak, c.perfect, c.champion)
		if k != c.known || u != c.unlocked {
			t.Errorf("%s streak=%d perfect=%v: got (%v,%v) want (%v,%v)", c.style, c.streak, c.perfect, k, u, c.known, c.unlocked)
		}
	}
}

func str(s string) *string { return &s }

func TestAvatarAllowed(t *testing.T) {
	ok := domain.Avatar{Style: "notionists", Seed: "x", Bg: "d1fae5", Options: map[string]map[string]*string{"notionists": {"hair": str("variant20"), "glasses": nil}}}
	if err := AvatarAllowed(ok, 0); err != nil {
		t.Errorf("valid avatar rejected: %v", err)
	}
	bad := ok
	bad.Style = "bitmoji"
	if AvatarAllowed(bad, 0) == nil {
		t.Error("unknown style accepted")
	}
	bad = ok
	bad.Bg = "#fff"
	if AvatarAllowed(bad, 0) == nil {
		t.Error("bad bg accepted")
	}
	sat := domain.Avatar{Style: "notionists", Seed: "x", Bg: "d1fae5", Options: map[string]map[string]*string{"notionists": {"bodyIcon": str("saturn")}}}
	if AvatarAllowed(sat, 14) == nil {
		t.Error("saturn at 14 accepted")
	}
	if err := AvatarAllowed(sat, 15); err != nil {
		t.Errorf("saturn at 15 rejected: %v", err)
	}
	gal := domain.Avatar{Style: "notionists", Seed: "x", Bg: "d1fae5", Options: map[string]map[string]*string{"notionists": {"bodyIcon": str("galaxy")}}}
	if AvatarAllowed(gal, 29) == nil {
		t.Error("galaxy at 29 accepted")
	}
}

func TestAvatarAllowedBlobatar(t *testing.T) {
	if err := AvatarAllowed(domain.Avatar{Style: "blobatar", Seed: "x", Bg: "d1fae5"}, 0); err != nil {
		t.Errorf("blobatar rejected: %v", err)
	}
	if err := AvatarAllowed(domain.Avatar{Style: "blobatar", Seed: "x"}, 0); err != nil {
		t.Errorf("blobatar without bg rejected: %v", err)
	}
}

func TestAvatarLocksOnlyCheckActiveStyle(t *testing.T) {
	// galaxy kept from when the streak was 30, but the learner now uses Blobatar
	a := domain.Avatar{Style: "blobatar", Seed: "x", Bg: "fce7f3", Options: map[string]map[string]*string{"notionists": {"bodyIcon": str("galaxy")}}}
	if err := AvatarAllowed(a, 0); err != nil {
		t.Errorf("an unused style's locked item must not block saving: %v", err)
	}
	a.Style = "notionists"
	if err := AvatarAllowed(a, 0); err != ErrAvatarLocked {
		t.Errorf("active locked item must still be rejected, got %v", err)
	}
}
