package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"gofiber-baro/internal/domain"
	"gofiber-baro/internal/service/giftbox"
	"gofiber-baro/internal/service/reward"
	"gofiber-baro/pkg/middleware"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type giftBoxHTTPStore struct {
	boxes          []domain.TeacherGiftBox
	cohortLearners []primitive.ObjectID
	teamLearners   []primitive.ObjectID
	grantKeys      map[string]bool
	recipientRoles map[primitive.ObjectID]string
}

func (s *giftBoxHTTPStore) Create(_ context.Context, box domain.TeacherGiftBox) error {
	s.boxes = append(s.boxes, box)
	return nil
}
func (s *giftBoxHTTPStore) CreateOnce(_ context.Context, box domain.TeacherGiftBox) (bool, error) {
	if s.grantKeys[box.GrantKey] {
		return false, nil
	}
	if s.grantKeys != nil {
		s.grantKeys[box.GrantKey] = true
	}
	s.boxes = append(s.boxes, box)
	return true, nil
}
func (s *giftBoxHTTPStore) ListForUser(_ context.Context, id primitive.ObjectID) ([]domain.TeacherGiftBox, error) {
	items := []domain.TeacherGiftBox{}
	for _, box := range s.boxes {
		if box.UserID == id {
			items = append(items, box)
		}
	}
	return items, nil
}
func (s *giftBoxHTTPStore) ListCohortLearners(context.Context, int) ([]primitive.ObjectID, error) {
	return s.cohortLearners, nil
}
func (s *giftBoxHTTPStore) ListTeamLearners(context.Context, int, string) ([]primitive.ObjectID, error) {
	return s.teamLearners, nil
}
func (s *giftBoxHTTPStore) SearchRecipients(context.Context, string, primitive.ObjectID) ([]giftbox.Recipient, error) {
	return []giftbox.Recipient{}, nil
}
func (s *giftBoxHTTPStore) IsLearner(_ context.Context, userID primitive.ObjectID) (bool, error) {
	return s.recipientRoles[userID] == "learner", nil
}
func (s *giftBoxHTTPStore) Transfer(_ context.Context, boxID, fromID, toID primitive.ObjectID) (*domain.TeacherGiftBox, error) {
	for index := range s.boxes {
		if s.boxes[index].ID == boxID && s.boxes[index].UserID == fromID && s.boxes[index].Status == "unopened" {
			s.boxes[index].UserID = toID
			return &s.boxes[index], nil
		}
	}
	return nil, giftbox.ErrBoxNotFound
}

type giftBoxHTTPDrawer struct {
	request reward.DrawRequest
}

func (d *giftBoxHTTPDrawer) Odds(_ context.Context, request reward.DrawRequest) (reward.Eligibility, error) {
	d.request = request
	return reward.Eligibility{EligibleCount: 1, Odds: map[string]float64{"Rare": 1}}, nil
}
func (d *giftBoxHTTPDrawer) Open(_ context.Context, request reward.DrawRequest) (*reward.DrawResult, error) {
	d.request = request
	return &reward.DrawResult{IdempotencyKey: request.IdempotencyKey, Pool: request.Pool}, nil
}

func TestCharacterGiftBoxRequiresAdminGrantAndOwnerOdds(t *testing.T) {
	t.Setenv("JWT_SECRET_KEY", "test-character-secret")
	learner := primitive.NewObjectID()
	other := primitive.NewObjectID()
	admin := primitive.NewObjectID()
	store := &giftBoxHTTPStore{recipientRoles: map[primitive.ObjectID]string{learner: "learner", other: "learner", admin: "admin"}}
	drawer := &giftBoxHTTPDrawer{}
	h := NewGiftBoxHandler(giftbox.NewService(store, drawer))
	app := fiber.New()
	self := app.Group("/gift-boxes", middleware.AuthMiddleware)
	self.Get("/:id/odds", h.Odds)
	self.Post("/:id/open", h.Open)
	self.Post("/:id/transfer", h.Transfer)
	adminGroup := app.Group("/admin", middleware.AuthMiddleware, middleware.CheckAdminRole)
	adminGroup.Post("/users/:id/gift-boxes", h.Grant)
	path := "/admin/users/" + learner.Hex() + "/gift-boxes"
	payload := `{"minimum_rarity":"Rare","message":"A character gift","reward_pool":"character-box"}`
	if status, _ := characterRequest(t, app, http.MethodPost, path, characterToken(t, learner, "learner"), payload); status != http.StatusForbidden {
		t.Fatalf("learner grant status = %d", status)
	}
	if status, _ := characterRequest(t, app, http.MethodPost, path, characterToken(t, admin, "admin"), payload); status != http.StatusCreated {
		t.Fatalf("admin grant status = %d", status)
	}
	if len(store.boxes) != 1 || store.boxes[0].RewardPool != "character-box" {
		t.Fatalf("saved boxes = %+v", store.boxes)
	}
	oddsPath := "/gift-boxes/" + store.boxes[0].ID.Hex() + "/odds"
	if status, _ := characterRequest(t, app, http.MethodGet, oddsPath, characterToken(t, other, "learner")); status != http.StatusBadRequest {
		t.Fatalf("other account odds status = %d", status)
	}
	if status, _ := characterRequest(t, app, http.MethodGet, oddsPath, characterToken(t, learner, "learner")); status != http.StatusOK {
		t.Fatalf("owner odds status = %d", status)
	}
	if drawer.request.Pool != "character-box" || drawer.request.UserID != learner.Hex() {
		t.Fatalf("odds request = %+v", drawer.request)
	}
	transferPath := "/gift-boxes/" + store.boxes[0].ID.Hex() + "/transfer"
	transferBody := "{\"recipient_id\":\"" + admin.Hex() + "\"}"
	if status, _ := characterRequest(t, app, http.MethodPost, transferPath, characterToken(t, other, "learner"), transferBody); status != http.StatusNotFound {
		t.Fatalf("nonowner transfer status = %d", status)
	}
	if status, _ := characterRequest(t, app, http.MethodPost, transferPath, characterToken(t, learner, "learner"), transferBody); status != http.StatusNotFound {
		t.Fatalf("owner transfer status = %d", status)
	}
	if store.boxes[0].UserID != learner || store.boxes[0].GrantedBy != admin {
		t.Fatalf("transferred box = %+v", store.boxes[0])
	}
}

func TestAudienceGrantRequiresAdminAndReachesOnlySelectedTeam(t *testing.T) {
	t.Setenv("JWT_SECRET_KEY", "test-character-secret")
	first := primitive.NewObjectID()
	second := primitive.NewObjectID()
	other := primitive.NewObjectID()
	adminID := primitive.NewObjectID()
	store := &giftBoxHTTPStore{
		cohortLearners: []primitive.ObjectID{first, second, other},
		teamLearners:   []primitive.ObjectID{first, second}, grantKeys: map[string]bool{},
	}
	h := NewGiftBoxHandler(giftbox.NewService(store, &giftBoxHTTPDrawer{}))
	app := fiber.New()
	boxes := app.Group("/gift-boxes", middleware.AuthMiddleware)
	boxes.Get("", h.List)
	admin := app.Group("/admin", middleware.AuthMiddleware, middleware.CheckAdminRole)
	admin.Get("/cohorts/:cohortNumber/gift-boxes/recipients", h.PreviewAudience)
	admin.Post("/cohorts/:cohortNumber/gift-boxes", h.GrantCohort)
	path := "/admin/cohorts/16/gift-boxes"
	preview := path + "/recipients?team=Garden%20Alpha"
	if status, _ := characterRequest(t, app, http.MethodGet, preview, characterToken(t, first, "learner")); status != http.StatusForbidden {
		t.Fatalf("learner preview status = %d", status)
	}
	status, body := characterRequest(t, app, http.MethodGet, preview, characterToken(t, adminID, "admin"))
	if status != http.StatusOK {
		t.Fatalf("admin preview status = %d", status)
	}
	var count struct {
		Total int `json:"total"`
	}
	if err := json.Unmarshal(body["data"], &count); err != nil || count.Total != 2 {
		t.Fatalf("preview count = %+v, err=%v", count, err)
	}
	payload := `{"minimum_rarity":"Rare","message":"Well done","idempotency_key":"team-16","team":"Garden Alpha","reward_pool":"character-box"}`
	if status, _ := characterRequest(t, app, http.MethodPost, path, characterToken(t, first, "learner"), payload); status != http.StatusForbidden {
		t.Fatalf("learner grant status = %d", status)
	}
	for attempt := 0; attempt < 2; attempt++ {
		if status, _ := characterRequest(t, app, http.MethodPost, path, characterToken(t, adminID, "admin"), payload); status != http.StatusOK {
			t.Fatalf("admin grant attempt %d status = %d", attempt, status)
		}
	}
	if len(store.boxes) != 2 {
		t.Fatalf("team should have two boxes, got %d", len(store.boxes))
	}
	for _, id := range []primitive.ObjectID{first, second, other} {
		status, body = characterRequest(t, app, http.MethodGet, "/gift-boxes", characterToken(t, id, "learner"))
		if status != http.StatusOK {
			t.Fatalf("recipient list status = %d", status)
		}
		var items []domain.TeacherGiftBox
		if err := json.Unmarshal(body["data"], &items); err != nil {
			t.Fatal(err)
		}
		want := 1
		if id == other {
			want = 0
		}
		if len(items) != want || (want == 1 && items[0].RewardPool != "character-box") {
			t.Fatalf("recipient %s boxes = %+v", id.Hex(), items)
		}
	}
}
