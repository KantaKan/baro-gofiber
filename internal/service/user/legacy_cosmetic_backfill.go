package user

import "gofiber-baro/internal/domain"

type LegacyCosmeticBackfill struct {
	OwnedIDs []string
	Equip    map[string]string
}

func PlanLegacyCosmeticBackfill(learner *domain.User) LegacyCosmeticBackfill {
	plan := LegacyCosmeticBackfill{Equip: make(map[string]string)}
	if learner == nil {
		return plan
	}
	owned := make(map[string]bool, len(learner.OwnedCosmeticIDs))
	for _, id := range learner.OwnedCosmeticIDs {
		owned[id] = true
	}
	selected := map[string]string{"palette": learner.SelectedPalette, "pot": learner.SelectedPot}
	for _, slot := range []string{"palette", "pot"} {
		value := selected[slot]
		if value == "" {
			continue
		}
		item, found := cosmeticForLegacySelection(slot, value)
		if !found {
			continue
		}
		if !item.Starter && !owned[item.ID] {
			plan.OwnedIDs = append(plan.OwnedIDs, item.ID)
		}
		if learner.EquippedCosmetics[slot] == "" {
			plan.Equip[slot] = item.ID
		}
	}
	return plan
}

func cosmeticForLegacySelection(slot, value string) (domain.CosmeticCatalogItem, bool) {
	for _, item := range cosmeticCatalog {
		if item.Slot == slot && item.PreviewValue == value {
			return item, true
		}
	}
	return domain.CosmeticCatalogItem{}, false
}
