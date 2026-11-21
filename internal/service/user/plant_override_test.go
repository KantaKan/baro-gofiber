package user

import (
	"reflect"
	"testing"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type plantOverrideRepoStub struct {
	domain.UserRepository
	user    *domain.User
	granted []string
	update  map[string]interface{}
}

func (s *plantOverrideRepoStub) FindByID(_ interface{}, _ primitive.ObjectID) (*domain.User, error) {
	return s.user, nil
}

func (s *plantOverrideRepoStub) GrantCosmetic(_ interface{}, _ primitive.ObjectID, cosmeticID string) (bool, error) {
	s.granted = append(s.granted, cosmeticID)
	return true, nil
}

func (s *plantOverrideRepoStub) Update(_ interface{}, _ primitive.ObjectID, update interface{}) error {
	s.update = update.(map[string]interface{})
	return nil
}

func TestPlantOverrideUpdatesChangedCosmeticsWithoutClearingOtherSlots(t *testing.T) {
	userID := primitive.NewObjectID()
	repo := &plantOverrideRepoStub{user: &domain.User{ID: userID, SelectedPalette: "Ocean", SelectedPot: "round"}}
	service := NewService(repo)
	if err := service.UpdatePlantOverride(userID.Hex(), map[string]interface{}{"selected_palette": "Ocean", "selected_pot": "trophy"}, "Ocean", "trophy"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(repo.granted, []string{"pot:trophy"}) {
		t.Fatalf("unexpected grants: %v", repo.granted)
	}
	if repo.update["equipped_cosmetics.pot"] != "pot:trophy" {
		t.Fatalf("pot was not equipped: %v", repo.update)
	}
	if _, changed := repo.update["equipped_cosmetics.palette"]; changed {
		t.Fatalf("unchanged palette equipment was overwritten: %v", repo.update)
	}
}
