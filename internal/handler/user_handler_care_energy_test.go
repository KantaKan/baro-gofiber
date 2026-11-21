package handler

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"gofiber-baro/internal/domain"
	userService "gofiber-baro/internal/service/user"
	"gofiber-baro/pkg/middleware"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type careEnergyHTTPRepo struct {
	domain.UserRepository
	user   *domain.User
	grants int
}

func (r *careEnergyHTTPRepo) FindByID(_ interface{}, _ primitive.ObjectID) (*domain.User, error) {
	return r.user, nil
}

func (r *careEnergyHTTPRepo) UseCareEnergyCharacter(_ interface{}, _ primitive.ObjectID, effect string) error {
	if r.user.CareEnergyBalance < 1 {
		return domain.ErrInsufficientCareEnergy
	}
	r.user.CareEnergyBalance--
	r.user.CharacterCareCount++
	now := time.Now().UTC()
	r.user.LastCaredAt = &now
	r.user.CareEnergyLog = append(r.user.CareEnergyLog, domain.CareEnergyLogEntry{Kind: "character-care", Amount: 1, Note: effect, CreatedAt: now})
	return nil
}

func (r *careEnergyHTTPRepo) GrantCareEnergy(_ interface{}, _ primitive.ObjectID, amount int, _, _ string) error {
	r.user.CareEnergyBalance += amount
	r.grants++
	return nil
}

func TestCareEnergyAPIUsesCanonicalContract(t *testing.T) {
	t.Setenv("JWT_SECRET_KEY", "test-character-secret")
	owner := primitive.NewObjectID()
	boxID := primitive.NewObjectID()
	repo := &careEnergyHTTPRepo{user: &domain.User{
		ID:                owner,
		CareEnergyBalance: 3,
		GrowthPoints:      40,
		GiftBoxes:         []domain.TeacherGiftBox{{ID: boxID}},
	}}
	handler := &UserHandler{careEnergyService: userService.NewCareEnergyService(repo, nil)}
	app := fiber.New()
	users := app.Group("/users", middleware.AuthMiddleware)
	users.Get("/:id/care-energy", handler.GetCareEnergy)
	users.Post("/:id/care-energy/character-care", handler.CareForCharacter)
	token := characterToken(t, owner, "learner")

	status, beforeBody := characterRequest(t, app, http.MethodGet, "/users/"+owner.Hex()+"/care-energy", token)
	if status != http.StatusOK {
		t.Fatalf("care energy status = %d", status)
	}
	var before domain.CareEnergyState
	if err := json.Unmarshal(beforeBody["data"], &before); err != nil {
		t.Fatal(err)
	}
	if before.Balance != 3 {
		t.Fatalf("account balance was lost: %+v", before)
	}

	status, careBody := characterRequest(t, app, http.MethodPost, "/users/"+owner.Hex()+"/care-energy/character-care", token)
	if status != http.StatusOK {
		t.Fatalf("character care status = %d", status)
	}
	var result domain.CharacterCareResult
	if err := json.Unmarshal(careBody["data"], &result); err != nil {
		t.Fatal(err)
	}
	if result.State.Balance != 2 || result.State.CareCount != 1 {
		t.Fatalf("care response is inconsistent: %+v", result)
	}
	if repo.user.GrowthPoints != 40 || len(repo.user.GiftBoxes) != 1 || repo.user.GiftBoxes[0].ID != boxID {
		t.Fatalf("care changed growth or draw rewards: %+v", repo.user)
	}

	otherToken := characterToken(t, primitive.NewObjectID(), "learner")
	if status, _ := characterRequest(t, app, http.MethodPost, "/users/"+owner.Hex()+"/care-energy/character-care", otherToken); status != http.StatusForbidden {
		t.Fatalf("another learner care status = %d", status)
	}
}

func TestAdminCareEnergyRoutesGrantExistingBalance(t *testing.T) {
	t.Setenv("JWT_SECRET_KEY", "test-character-secret")
	adminID := primitive.NewObjectID()
	learnerID := primitive.NewObjectID()
	repo := &careEnergyHTTPRepo{user: &domain.User{ID: learnerID, CareEnergyBalance: 5}}
	handler := &AdminHandler{careEnergyService: userService.NewCareEnergyService(repo, nil)}
	app := fiber.New()
	admin := app.Group("/admin", middleware.AuthMiddleware, middleware.CheckAdminRole)
	admin.Post("/users/:id/care-energy", handler.GrantCareEnergy)
	admin.Post("/care-energy/bulk", handler.BulkGrantCareEnergy)
	token := characterToken(t, adminID, "admin")

	status, _ := characterRequest(t, app, http.MethodPost, "/admin/users/"+learnerID.Hex()+"/care-energy", token, `{"amount":2}`)
	if status != http.StatusOK {
		t.Fatalf("individual Care Energy grant status = %d", status)
	}
	status, _ = characterRequest(t, app, http.MethodPost, "/admin/care-energy/bulk", token, `{"userIds":["`+learnerID.Hex()+`"],"amount":3}`)
	if status != http.StatusOK {
		t.Fatalf("bulk Care Energy grant status = %d", status)
	}
	if repo.user.CareEnergyBalance != 10 || repo.grants != 2 {
		t.Fatalf("balance was not preserved through grants: %+v", repo.user)
	}
}
