package reward

import (
	"errors"
	"math"
	"testing"

	"gofiber-baro/internal/domain"
)

type sequenceRandom struct {
	values []float64
	index  int
}

func (r *sequenceRandom) Float64() float64 {
	value := r.values[r.index]
	r.index++
	return value
}

func rewardCatalog() []domain.CosmeticCatalogItem {
	return []domain.CosmeticCatalogItem{
		{ID: "common", Rarity: "Common", RewardPools: []string{"standard"}},
		{ID: "rare", Rarity: "Rare", RewardPools: []string{"standard"}},
		{ID: "epic", Rarity: "Epic", RewardPools: []string{"standard"}},
		{ID: "legendary", Rarity: "Legendary", RewardPools: []string{"standard"}},
	}
}

func TestSelectorUsesDisclosedWeightBoundaries(t *testing.T) {
	tests := []struct {
		name   string
		roll   float64
		rarity string
	}{
		{name: "first common value", roll: 0, rarity: "Common"},
		{name: "last common value", roll: 0.549999, rarity: "Common"},
		{name: "first rare value", roll: 0.55, rarity: "Rare"},
		{name: "last rare value", roll: 0.849999, rarity: "Rare"},
		{name: "first epic value", roll: 0.85, rarity: "Epic"},
		{name: "last epic value", roll: 0.969999, rarity: "Epic"},
		{name: "first legendary value", roll: 0.97, rarity: "Legendary"},
		{name: "last legendary value", roll: 0.999999, rarity: "Legendary"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			selector := NewSelector(&sequenceRandom{values: []float64{test.roll, 0}})
			item, err := selector.Select(SelectionInput{Catalog: rewardCatalog(), Pool: "standard", OwnedIDs: map[string]bool{}})
			if err != nil {
				t.Fatalf("Select returned an error: %v", err)
			}
			if item.Rarity != test.rarity {
				t.Fatalf("roll %v selected %s, want %s", test.roll, item.Rarity, test.rarity)
			}
		})
	}
}

func TestSelectorFiltersBeforeDrawing(t *testing.T) {
	tests := []struct {
		name    string
		input   SelectionInput
		wantID  string
		wantErr error
	}{
		{
			name:   "minimum rarity removes lower tiers",
			input:  SelectionInput{Catalog: rewardCatalog(), Pool: "standard", MinimumRarity: "Epic", OwnedIDs: map[string]bool{}},
			wantID: "epic",
		},
		{
			name:   "owned items are excluded",
			input:  SelectionInput{Catalog: rewardCatalog(), Pool: "standard", OwnedIDs: map[string]bool{"common": true, "rare": true, "epic": true}},
			wantID: "legendary",
		},
		{
			name: "incompatible species are excluded",
			input: SelectionInput{Catalog: []domain.CosmeticCatalogItem{
				{ID: "mango-only", Rarity: "Rare", RewardPools: []string{"standard"}, CompatibleSpecies: []string{"Mango"}},
				{ID: "lotus", Rarity: "Rare", RewardPools: []string{"standard"}, CompatibleSpecies: []string{"Lotus"}},
			}, Pool: "standard", Species: "Lotus", OwnedIDs: map[string]bool{}},
			wantID: "lotus",
		},
		{
			name: "completed pool returns an explicit error",
			input: SelectionInput{Catalog: rewardCatalog(), Pool: "standard", OwnedIDs: map[string]bool{
				"common": true, "rare": true, "epic": true, "legendary": true,
			}},
			wantErr: ErrPoolComplete,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			selector := NewSelector(&sequenceRandom{values: []float64{0, 0}})
			item, err := selector.Select(test.input)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("Select error = %v, want %v", err, test.wantErr)
			}
			if item.ID != test.wantID {
				t.Fatalf("Select item = %q, want %q", item.ID, test.wantID)
			}
		})
	}
}

func TestDisclosedOddsRenormalizeAtRarityFloor(t *testing.T) {
	odds := DisclosedOdds("Epic")
	if _, exists := odds["Common"]; exists {
		t.Fatal("Epic floor disclosed Common odds")
	}
	if _, exists := odds["Rare"]; exists {
		t.Fatal("Epic floor disclosed Rare odds")
	}
	if math.Abs(odds["Epic"]-0.8) > 0.000001 || math.Abs(odds["Legendary"]-0.2) > 0.000001 {
		t.Fatalf("Epic floor odds were not renormalized: %v", odds)
	}
}

func TestSelectorChoosesWithinRarityWithControlledRandomness(t *testing.T) {
	catalog := []domain.CosmeticCatalogItem{
		{ID: "rare-a", Rarity: "Rare", RewardPools: []string{"standard"}},
		{ID: "rare-b", Rarity: "Rare", RewardPools: []string{"standard"}},
	}
	selector := NewSelector(&sequenceRandom{values: []float64{0, 0.5}})
	item, err := selector.Select(SelectionInput{Catalog: catalog, Pool: "standard", OwnedIDs: map[string]bool{}})
	if err != nil || item.ID != "rare-b" {
		t.Fatalf("controlled item roll selected %+v with error %v", item, err)
	}
}

func TestEligibilityOddsUseOnlyUnownedItemsInTheSelectedPool(t *testing.T) {
	selector := NewSelector(&sequenceRandom{values: []float64{0, 0}})
	input := SelectionInput{Catalog: []domain.CosmeticCatalogItem{
		{ID: "prop-common", Rarity: "Common", RewardPools: []string{"character-box"}},
		{ID: "prop-rare", Rarity: "Rare", RewardPools: []string{"character-box"}},
		{ID: "plant-rare", Rarity: "Rare", RewardPools: []string{"teacher-box"}},
		{ID: "prop-legendary", Rarity: "Legendary", RewardPools: []string{"character-box"}},
	}, Pool: "character-box", OwnedIDs: map[string]bool{"prop-common": true}, MinimumRarity: "Common"}
	preview := selector.Eligibility(input)
	if preview.Complete || preview.EligibleCount != 2 || math.Abs(preview.Odds["Rare"]-30.0/33.0) > 0.000001 || math.Abs(preview.Odds["Legendary"]-3.0/33.0) > 0.000001 || preview.Odds["Common"] != 0 {
		t.Fatalf("eligibility odds = %+v", preview)
	}
	item, err := selector.Select(input)
	if err != nil || item.ID != "prop-rare" {
		t.Fatalf("selected item = %+v, err=%v", item, err)
	}
	input.OwnedIDs["prop-rare"] = true
	input.OwnedIDs["prop-legendary"] = true
	complete := selector.Eligibility(input)
	if !complete.Complete || complete.EligibleCount != 0 || len(complete.Odds) != 0 {
		t.Fatalf("complete preview = %+v", complete)
	}
	if _, err := selector.Select(input); !errors.Is(err, ErrPoolComplete) {
		t.Fatalf("complete pool draw error = %v", err)
	}
}
