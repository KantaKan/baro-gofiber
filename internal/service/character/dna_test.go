package character

import (
	"errors"
	"testing"

	"gofiber-baro/internal/domain"
)

type fakePicker struct {
	values []int
	index  int
}

func (picker *fakePicker) Intn(limit int) (int, error) {
	if picker.index >= len(picker.values) {
		return 0, errors.New("no test value")
	}
	value := picker.values[picker.index]
	picker.index++
	return value, nil
}

func TestRarityForRollBoundaries(t *testing.T) {
	cases := []struct {
		roll int
		want domain.CharacterRarity
	}{
		{0, domain.CharacterNormal}, {8299, domain.CharacterNormal},
		{8300, domain.CharacterMemeRare}, {9799, domain.CharacterMemeRare},
		{9800, domain.CharacterLegendary}, {9999, domain.CharacterLegendary},
	}
	for _, test := range cases {
		got, err := RarityForRoll(test.roll)
		if err != nil || got != test.want {
			t.Errorf("roll %d: got %q, %v; want %q", test.roll, got, err, test.want)
		}
	}
	for _, invalid := range []int{-1, 10000} {
		if _, err := RarityForRoll(invalid); err == nil {
			t.Errorf("roll %d should fail", invalid)
		}
	}
}

func TestGenerateDNAUsesRarePatternPool(t *testing.T) {
	picker := &fakePicker{values: []int{9800, 0, 5, 0, 0, 0, 2, 12345}}
	dna, err := GenerateDNA(picker)
	if err != nil {
		t.Fatal(err)
	}
	if dna.Rarity != domain.CharacterLegendary || dna.Ears != "cat" || dna.Pattern != "ramen" || dna.PatternSeed != 12345 {
		t.Fatalf("unexpected DNA: %+v", dna)
	}
	if dna.Fingerprint() == "" {
		t.Fatal("fingerprint is required")
	}
}

func TestGenerateDNARejectsBadPicker(t *testing.T) {
	if _, err := GenerateDNA(nil); err == nil {
		t.Fatal("nil picker should fail")
	}
	if _, err := GenerateDNA(&fakePicker{values: []int{10000}}); err == nil {
		t.Fatal("invalid rarity roll should fail")
	}
}
