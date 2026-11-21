package handler

import (
	"errors"
	"net/http"
	"testing"

	"gofiber-baro/internal/domain"
	"gofiber-baro/internal/service/user"
	"gofiber-baro/pkg/middleware"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type characterCosmeticUsers struct {
	accounts map[primitive.ObjectID]*domain.User
}

func (s characterCosmeticUsers) FindByID(_ interface{}, id primitive.ObjectID) (*domain.User, error) {
	account := s.accounts[id]
	if account == nil {
		return nil, errors.New("user not found")
	}
	return account, nil
}
func (s characterCosmeticUsers) GrantCosmetic(_ interface{}, id primitive.ObjectID, cosmeticID string) (bool, error) {
	account, err := s.FindByID(nil, id)
	if err != nil {
		return false, err
	}
	for _, existing := range account.OwnedCosmeticIDs {
		if existing == cosmeticID {
			return false, nil
		}
	}
	account.OwnedCosmeticIDs = append(account.OwnedCosmeticIDs, cosmeticID)
	return true, nil
}
func (s characterCosmeticUsers) RevokeCosmetic(_ interface{}, _ primitive.ObjectID, _, _, _ string) (bool, error) {
	return false, nil
}
func (s characterCosmeticUsers) EquipCosmetic(_ interface{}, id primitive.ObjectID, cosmeticID, slot string, requiresOwnership bool) error {
	account, err := s.FindByID(nil, id)
	if err != nil {
		return err
	}
	if requiresOwnership {
		owned := false
		for _, existing := range account.OwnedCosmeticIDs {
			owned = owned || existing == cosmeticID
		}
		if !owned {
			return errors.New("not owned")
		}
	}
	if account.EquippedCosmetics == nil {
		account.EquippedCosmetics = map[string]string{}
	}
	account.EquippedCosmetics[slot] = cosmeticID
	return nil
}
func (s characterCosmeticUsers) UnequipCosmetic(_ interface{}, id primitive.ObjectID, slot string) error {
	account, err := s.FindByID(nil, id)
	if err != nil {
		return err
	}
	delete(account.EquippedCosmetics, slot)
	return nil
}

func TestCharacterCosmeticRoutesAuthorizeAndSeparateInventory(t *testing.T) {
	t.Setenv("JWT_SECRET_KEY", "test-character-secret")
	learner := primitive.NewObjectID()
	admin := primitive.NewObjectID()
	users := characterCosmeticUsers{accounts: map[primitive.ObjectID]*domain.User{
		learner: {ID: learner, OwnedCosmeticIDs: []string{"palette:ocean"}, EquippedCosmetics: map[string]string{"palette": "palette:ocean"}},
		admin:   {ID: admin},
	}}
	h := NewCosmeticHandler(user.NewCosmeticService(users))
	app := fiber.New()
	self := app.Group("/character-cosmetics", middleware.AuthMiddleware)
	self.Get("/collection", h.GetCharacterCollection)
	self.Put("/equipment/:slot", h.EquipCharacterCosmetic)
	adminGroup := app.Group("/admin", middleware.AuthMiddleware, middleware.CheckAdminRole)
	adminGroup.Post("/users/:id/character-cosmetics/:cosmeticId", h.GrantCharacterCosmetic)
	learnerToken := characterToken(t, learner, "learner")
	adminToken := characterToken(t, admin, "admin")
	grantPath := "/admin/users/" + learner.Hex() + "/character-cosmetics/character_prop%3Aflower"
	if status, _ := characterRequest(t, app, http.MethodPost, grantPath, learnerToken, `{}`); status != http.StatusForbidden {
		t.Fatalf("learner grant status = %d", status)
	}
	if status, _ := characterRequest(t, app, http.MethodPut, "/character-cosmetics/equipment/character_prop", learnerToken, `{"cosmetic_id":"character_prop:flower"}`); status != http.StatusBadRequest {
		t.Fatalf("unowned equip status = %d", status)
	}
	if status, _ := characterRequest(t, app, http.MethodPost, grantPath, adminToken, `{}`); status != http.StatusOK {
		t.Fatalf("admin grant status = %d", status)
	}
	if status, _ := characterRequest(t, app, http.MethodPut, "/character-cosmetics/equipment/character_prop", learnerToken, `{"cosmetic_id":"character_prop:flower"}`); status != http.StatusOK {
		t.Fatalf("owned equip status = %d", status)
	}
	if users.accounts[learner].EquippedCosmetics["palette"] != "palette:ocean" {
		t.Fatal("character prop erased plant equipment")
	}
	if status, _ := characterRequest(t, app, http.MethodPut, "/character-cosmetics/equipment/palette", learnerToken, `{"cosmetic_id":"palette:ocean"}`); status != http.StatusBadRequest {
		t.Fatalf("wrong slot status = %d", status)
	}
	if status, _ := characterRequest(t, app, http.MethodGet, "/character-cosmetics/collection", ""); status != http.StatusUnauthorized {
		t.Fatalf("anonymous collection status = %d", status)
	}
}
