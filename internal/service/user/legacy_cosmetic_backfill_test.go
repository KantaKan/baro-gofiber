package user

import (
	"reflect"
	"testing"

	"gofiber-baro/internal/domain"
)

func TestLegacyCosmeticBackfillPreservesExistingAppearance(t *testing.T) {
	learner := &domain.User{SelectedPalette: "Ocean", SelectedPot: "trophy"}
	plan := PlanLegacyCosmeticBackfill(learner)
	if !reflect.DeepEqual(plan.OwnedIDs, []string{"palette:ocean", "pot:trophy"}) {
		t.Fatalf("unexpected ownership: %v", plan.OwnedIDs)
	}
	if plan.Equip["palette"] != "palette:ocean" || plan.Equip["pot"] != "pot:trophy" {
		t.Fatalf("unexpected loadout: %v", plan.Equip)
	}
	if learner.SelectedPalette != "Ocean" || learner.SelectedPot != "trophy" {
		t.Fatal("legacy selections changed")
	}
}

func TestLegacyCosmeticBackfillIsIdempotentAndPartial(t *testing.T) {
	learner := &domain.User{
		SelectedPalette:   "Midnight",
		SelectedPot:       "crystal",
		OwnedCosmeticIDs:  []string{"palette:midnight"},
		EquippedCosmetics: map[string]string{"palette": "palette:ocean"},
	}
	plan := PlanLegacyCosmeticBackfill(learner)
	if !reflect.DeepEqual(plan.OwnedIDs, []string{"pot:crystal"}) || !reflect.DeepEqual(plan.Equip, map[string]string{"pot": "pot:crystal"}) {
		t.Fatalf("partial plan overwrote existing progress: %+v", plan)
	}
	learner.OwnedCosmeticIDs = append(learner.OwnedCosmeticIDs, plan.OwnedIDs...)
	learner.EquippedCosmetics["pot"] = plan.Equip["pot"]
	second := PlanLegacyCosmeticBackfill(learner)
	if len(second.OwnedIDs) != 0 || len(second.Equip) != 0 {
		t.Fatalf("second run was not empty: %+v", second)
	}
}

func TestLegacyCosmeticBackfillCoversEveryLegacyStyle(t *testing.T) {
	palettes := []string{"Forest", "Sunset", "Ocean", "Desert", "Rose", "Lavender", "Sunshine", "Mint", "Coral", "Autumn", "Jade", "Berry", "Citrus", "Slate", "Blush", "Midnight"}
	pots := []string{"round", "square", "tall", "bowl", "trophy", "starlight", "rainbow", "crystal", "sweetheart", "laurel", "constellation", "mosaic", "royal", "firework"}
	for _, palette := range palettes {
		if PlanLegacyCosmeticBackfill(&domain.User{SelectedPalette: palette}).Equip["palette"] == "" {
			t.Fatalf("missing palette %q", palette)
		}
	}
	for _, pot := range pots {
		if PlanLegacyCosmeticBackfill(&domain.User{SelectedPot: pot}).Equip["pot"] == "" {
			t.Fatalf("missing pot %q", pot)
		}
	}
}
