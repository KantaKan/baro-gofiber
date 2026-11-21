package domain

import "testing"

func TestResolveCharacterGrowthBoundaries(t *testing.T) {
	cases := []struct {
		best       int
		form       int
		detail     int
		nextDetail int
	}{
		{-1, 0, 1, 1}, {0, 0, 1, 1}, {1, 0, 2, 3}, {6, 0, 3, 7},
		{7, 1, 4, 14}, {29, 1, 6, 30}, {30, 2, 7, 50},
		{74, 2, 8, 75}, {75, 3, 9, 100}, {100, 3, 10, 0},
	}
	for _, test := range cases {
		growth := ResolveCharacterGrowth(test.best)
		if growth.FormIndex != test.form || growth.DetailIndex != test.detail {
			t.Errorf("best %d: form/detail = %d/%d, want %d/%d", test.best, growth.FormIndex, growth.DetailIndex, test.form, test.detail)
		}
		if test.nextDetail == 0 && growth.NextDetailAt != nil {
			t.Errorf("best %d: expected no next detail", test.best)
		}
		if test.nextDetail != 0 && (growth.NextDetailAt == nil || *growth.NextDetailAt != test.nextDetail) {
			t.Errorf("best %d: expected next detail at %d", test.best, test.nextDetail)
		}
	}
}

func TestCharacterDNAFingerprintChangesWithPatternPlacement(t *testing.T) {
	first := CharacterDNA{Version: 1, Body: "pebble", Ears: "cat", Eyes: "dots", Mark: "star", Palette: "mint", Pattern: "spots", PatternSeed: 42, Rarity: CharacterNormal}
	second := first
	second.PatternSeed++
	if first.Fingerprint() == second.Fingerprint() {
		t.Fatal("different pattern seeds must have different fingerprints")
	}
	if first.Fingerprint() != first.Fingerprint() {
		t.Fatal("fingerprint must be stable")
	}
}
