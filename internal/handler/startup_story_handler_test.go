package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"gofiber-baro/internal/domain"
	"gofiber-baro/internal/service/startupstory"
	"gofiber-baro/pkg/middleware"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type startupHTTPStore struct {
	startupstory.Store
	studio  domain.StartupStudio
	optOuts map[string]bool
}

func (s *startupHTTPStore) FindStudio(context.Context, primitive.ObjectID) (*domain.StartupStudio, error) {
	return &s.studio, nil
}
func (s *startupHTTPStore) InsertStudio(_ context.Context, studio domain.StartupStudio) error {
	s.studio = studio
	return nil
}
func (s *startupHTTPStore) FindActiveRun(context.Context, primitive.ObjectID) (*domain.StartupRun, error) {
	return nil, nil
}
func (s *startupHTTPStore) CountRankedRuns(context.Context, primitive.ObjectID, string) (int, error) {
	return 0, nil
}
func (s *startupHTTPStore) SetStartupOptOut(_ context.Context, userID primitive.ObjectID, optOut bool) error {
	if s.optOuts == nil {
		s.optOuts = map[string]bool{}
	}
	s.optOuts[userID.Hex()] = optOut
	return nil
}
func (s *startupHTTPStore) FindStartupOptOut(_ context.Context, userID primitive.ObjectID) (bool, error) {
	return s.optOuts[userID.Hex()], nil
}

func TestOptOutEndpointAndOverviewPreference(t *testing.T) {
	t.Setenv("JWT_SECRET_KEY", "test-character-secret")
	user := primitive.NewObjectID()
	store := &startupHTTPStore{}
	h := NewStartupStoryHandler(startupstory.NewService(store))
	app := fiber.New()
	app.Get("/startup-story", middleware.AuthMiddleware, h.Overview)
	app.Put("/startup-story/opt-out", middleware.AuthMiddleware, h.OptOut)

	if status, _ := characterRequest(t, app, http.MethodPut, "/startup-story/opt-out", ""); status != http.StatusUnauthorized {
		t.Fatalf("anonymous opt-out status = %d", status)
	}
	token := characterToken(t, user, "learner")

	status, body := characterRequest(t, app, http.MethodPut, "/startup-story/opt-out", token, `{"opt_out":true}`)
	if status != http.StatusOK {
		t.Fatalf("opt-out status = %d", status)
	}
	var saved struct {
		OptOut bool `json:"opt_out"`
	}
	if err := json.Unmarshal(body["data"], &saved); err != nil {
		t.Fatal(err)
	}
	if !saved.OptOut {
		t.Fatalf("response must echo the saved preference: %s", body["data"])
	}
	if !store.optOuts[user.Hex()] {
		t.Fatal("preference must be persisted for the genmate pool query")
	}

	status, body = characterRequest(t, app, http.MethodGet, "/startup-story", token)
	if status != http.StatusOK {
		t.Fatalf("overview status = %d", status)
	}
	var overview struct {
		OptOut bool `json:"opt_out"`
	}
	if err := json.Unmarshal(body["data"], &overview); err != nil {
		t.Fatal(err)
	}
	if !overview.OptOut {
		t.Fatal("overview must carry opt_out so the Lobby toggle renders the saved state")
	}

	status, body = characterRequest(t, app, http.MethodPut, "/startup-story/opt-out", token, `{"opt_out":false}`)
	if status != http.StatusOK {
		t.Fatalf("opt-in status = %d", status)
	}
	if err := json.Unmarshal(body["data"], &saved); err != nil {
		t.Fatal(err)
	}
	if saved.OptOut {
		t.Fatalf("opt-in must echo false: %s", body["data"])
	}
}
