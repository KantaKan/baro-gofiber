package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"gofiber-baro/internal/domain"
	"gofiber-baro/internal/service/godevent"
	"gofiber-baro/pkg/middleware"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type godEventHTTPStore struct {
	viewers map[primitive.ObjectID]godevent.Viewer
	events  []domain.GodEvent
}

func (s *godEventHTTPStore) Viewer(_ context.Context, id primitive.ObjectID) (*godevent.Viewer, error) {
	viewer, ok := s.viewers[id]
	if !ok {
		return nil, nil
	}
	return &viewer, nil
}
func (s *godEventHTTPStore) CohortExists(_ context.Context, cohort int) (bool, error) {
	return cohort == 16 || cohort == 17, nil
}
func (s *godEventHTTPStore) AdminCharacter(context.Context, primitive.ObjectID) (*domain.BaroCharacter, error) {
	return nil, nil
}
func (s *godEventHTTPStore) Create(_ context.Context, event domain.GodEvent) error {
	s.events = append(s.events, event)
	return nil
}
func (s *godEventHTTPStore) ListVisible(_ context.Context, viewer godevent.Viewer) ([]domain.GodEvent, error) {
	items := []domain.GodEvent{}
	for _, event := range s.events {
		if viewer.Role == "admin" || event.Cohort == 0 || event.Cohort == viewer.Cohort {
			items = append(items, event)
		}
	}
	return items, nil
}

func TestGodEventRoutesEnforceAudienceCaptionAndCastRateLimit(t *testing.T) {
	t.Setenv("JWT_SECRET_KEY", "test-character-secret")
	adminID := primitive.NewObjectID()
	learner16 := primitive.NewObjectID()
	learner17 := primitive.NewObjectID()
	store := &godEventHTTPStore{viewers: map[primitive.ObjectID]godevent.Viewer{
		adminID: {Role: "admin"}, learner16: {Role: "learner", Cohort: 16}, learner17: {Role: "learner", Cohort: 17},
	}}
	h := NewGodEventHandler(godevent.NewService(store))
	app := fiber.New()
	app.Get("/god-events", middleware.AuthMiddleware, h.List)
	admin := app.Group("/admin", middleware.AuthMiddleware, middleware.CheckAdminRole)
	admin.Post("/god-events", GodEventCastLimiter(), h.Cast)
	adminToken := characterToken(t, adminID, "admin")
	learnerToken := characterToken(t, learner16, "learner")
	path := "/admin/god-events"
	cohortPayload := `{"preset":"star_rain","caption":"Great work today ✨","cohort":16}`
	if status, _ := characterRequest(t, app, http.MethodPost, path, learnerToken, cohortPayload); status != http.StatusForbidden {
		t.Fatalf("learner cast status = %d", status)
	}
	if status, _ := characterRequest(t, app, http.MethodPost, path, adminToken, `{"preset":"star_rain","caption":"<script>alert(1)</script>","cohort":16}`); status != http.StatusBadRequest {
		t.Fatalf("unsafe caption status = %d", status)
	}
	if status, _ := characterRequest(t, app, http.MethodPost, path, adminToken, cohortPayload); status != http.StatusCreated {
		t.Fatalf("cohort cast status = %d", status)
	}
	if status, _ := characterRequest(t, app, http.MethodPost, path, adminToken, `{"preset":"character_parade","caption":"Everyone belongs here","cohort":0}`); status != http.StatusCreated {
		t.Fatalf("all-cohorts cast status = %d", status)
	}
	for _, test := range []struct {
		id   primitive.ObjectID
		want int
	}{{learner16, 2}, {learner17, 1}, {adminID, 2}} {
		status, body := characterRequest(t, app, http.MethodGet, "/god-events", characterToken(t, test.id, store.viewers[test.id].Role))
		if status != http.StatusOK {
			t.Fatalf("history status = %d", status)
		}
		var events []godevent.EventView
		if err := json.Unmarshal(body["data"], &events); err != nil || len(events) != test.want {
			t.Fatalf("history for %s = %+v, err=%v", test.id.Hex(), events, err)
		}
	}
	for attempt := 0; attempt < 3; attempt++ {
		if status, _ := characterRequest(t, app, http.MethodPost, path, adminToken, cohortPayload); status != http.StatusCreated {
			t.Fatalf("cast %d status = %d", attempt, status)
		}
	}
	if status, _ := characterRequest(t, app, http.MethodPost, path, adminToken, cohortPayload); status != http.StatusTooManyRequests {
		t.Fatalf("seventh cast status = %d", status)
	}
	if len(store.events) != 5 || store.events[0].CastBy != adminID {
		t.Fatalf("cast audit or limit = %+v", store.events)
	}
}
