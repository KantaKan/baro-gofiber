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

func TestGenerateDNAUsesPatternPoolForRarity(t *testing.T) {
	for _, test := range []struct {
		roll, patternIndex int
		rarity             domain.CharacterRarity
		pattern            string
	}{
		{0, 13, domain.CharacterNormal, "petals"},
		{8300, 5, domain.CharacterMemeRare, "this-is-fine"},
		{9800, 0, domain.CharacterLegendary, "galaxy"},
	} {
		picker := &fakePicker{values: []int{test.roll, 0, 5, 0, 0, 0, test.patternIndex, 12345}}
		dna, err := GenerateDNA(picker)
		if err != nil {
			t.Fatal(err)
		}
		if dna.Rarity != test.rarity || dna.Ears != "cat" || dna.Pattern != test.pattern || dna.PatternSeed != 12345 {
			t.Fatalf("unexpected DNA: %+v", dna)
		}
		if dna.Fingerprint() == "" {
			t.Fatal("fingerprint is required")
		}
	}
}

func TestGenerateEggDNAHonorsEveryEggTier(t *testing.T) {
	tests := []struct {
		name   string
		tier   string
		rolls  []int
		rarity domain.CharacterRarity
	}{
		{name: "standard normal", tier: "Common", rolls: []int{0, 0, 0, 0, 0, 0, 0, 0}, rarity: domain.CharacterNormal},
		{name: "rare meme", tier: "Rare", rolls: []int{0, 0, 0, 0, 0, 0, 0, 0}, rarity: domain.CharacterMemeRare},
		{name: "rare legendary", tier: "Rare", rolls: []int{1699, 0, 0, 0, 0, 0, 0, 0}, rarity: domain.CharacterLegendary},
		{name: "legendary", tier: "Legendary", rolls: []int{0, 0, 0, 0, 0, 0, 0}, rarity: domain.CharacterLegendary},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dna, err := GenerateEggDNA(&fakePicker{values: test.rolls}, test.tier)
			if err != nil || dna.Rarity != test.rarity {
				t.Fatalf("GenerateEggDNA() = %+v, %v", dna, err)
			}
		})
	}
	if _, err := GenerateEggDNA(&fakePicker{values: []int{0}}, "Epic"); err == nil {
		t.Fatal("unsupported egg tier should fail")
	}
}

func TestEggOddsMatchGenerationPolicy(t *testing.T) {
	standard, _ := EggOdds("Common")
	rare, _ := EggOdds("Rare")
	legendary, _ := EggOdds("Legendary")
	if standard["Normal"] != .83 || standard["Meme Rare"] != .15 || standard["Legendary"] != .02 {
		t.Fatalf("standard odds = %+v", standard)
	}
	if rare["Normal"] != 0 || rare["Meme Rare"] != 15.0/17.0 || rare["Legendary"] != 2.0/17.0 {
		t.Fatalf("rare odds = %+v", rare)
	}
	if legendary["Legendary"] != 1 || len(legendary) != 1 {
		t.Fatalf("legendary odds = %+v", legendary)
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
