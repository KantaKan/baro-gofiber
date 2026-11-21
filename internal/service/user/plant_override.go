package user

import (
	"context"
	"errors"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func (s *Service) UpdatePlantOverride(id string, update map[string]interface{}, palette, pot string) error {
	userID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return domain.ErrUserNotFound
	}
	current, err := s.repo.FindByID(context.Background(), userID)
	if err != nil {
		return err
	}
	for _, selection := range []struct{ slot, value string }{{"palette", palette}, {"pot", pot}} {
		if (selection.slot == "palette" && current.SelectedPalette == selection.value) ||
			(selection.slot == "pot" && current.SelectedPot == selection.value) {
			continue
		}
		cosmeticID := ""
		if selection.value != "" {
			item, found := cosmeticForLegacySelection(selection.slot, selection.value)
			if !found {
				return errors.New("unknown plant cosmetic")
			}
			cosmeticID = item.ID
			if !item.Starter {
				if _, err := s.repo.GrantCosmetic(context.Background(), userID, cosmeticID); err != nil {
					return err
				}
			}
		}
		update["equipped_cosmetics."+selection.slot] = cosmeticID
	}
	return s.repo.Update(context.Background(), userID, update)
}
