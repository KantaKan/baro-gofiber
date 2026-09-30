package repository

import "testing"

func TestShowcasePropAllowsOnlyKnownCharacterProps(t *testing.T) {
	for _, test := range []struct{ equipped, want string }{
		{"character_prop:flower", "flower"},
		{"character_prop:cat-ears", "cat-ears"},
		{"character_prop:egg", "egg"},
		{"character_prop:halo", "halo"},
		{"character_prop:headphones", "headphones"},
		{"character_prop:pixel-glasses", "pixel-glasses"},
		{"character_prop:tiny-crown", "tiny-crown"},
		{"character_prop:coffee-cup", "coffee-cup"},
		{"character_prop:golden-keyboard", "golden-keyboard"},
		{"palette:ocean", ""},
		{"character_prop:unknown", ""},
		{"", ""},
	} {
		if got := showcaseProp(test.equipped); got != test.want {
			t.Fatalf("showcaseProp(%q) = %q, want %q", test.equipped, got, test.want)
		}
	}
}

func TestShowcaseDisplayNamePrefersZoomThenFirstName(t *testing.T) {
	for _, test := range []struct{ zoom, first, want string }{
		{"Mew", "Kantapon", "Mew"},
		{"  ", "Kantapon", "Kantapon"},
		{"", "", "Baro friend"},
	} {
		if got := showcaseDisplayName(test.zoom, test.first); got != test.want {
			t.Fatalf("showcaseDisplayName(%q, %q) = %q, want %q", test.zoom, test.first, got, test.want)
		}
	}
}
