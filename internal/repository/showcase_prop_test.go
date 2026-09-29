package repository

import "testing"

func TestShowcasePropAllowsOnlyKnownCharacterProps(t *testing.T) {
	for _, test := range []struct{ equipped, want string }{
		{"character_prop:flower", "flower"},
		{"character_prop:cat-ears", "cat-ears"},
		{"character_prop:egg", "egg"},
		{"character_prop:halo", "halo"},
		{"palette:ocean", ""},
		{"character_prop:unknown", ""},
		{"", ""},
	} {
		if got := showcaseProp(test.equipped); got != test.want {
			t.Fatalf("showcaseProp(%q) = %q, want %q", test.equipped, got, test.want)
		}
	}
}
