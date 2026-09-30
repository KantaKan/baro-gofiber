package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"gofiber-baro/internal/domain"
	"gofiber-baro/internal/service/showcase"
	"gofiber-baro/pkg/middleware"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type showcaseHTTPStore struct {
	viewers   map[primitive.ObjectID]showcase.Viewer
	owned     map[primitive.ObjectID]domain.BaroCharacter
	entries   map[primitive.ObjectID]showcase.Entry
	reactions map[string]bool
	audit     []string
}

func (s *showcaseHTTPStore) Viewer(_ context.Context, id primitive.ObjectID) (*showcase.Viewer, error) {
	viewer, ok := s.viewers[id]
	if !ok {
		return nil, nil
	}
	return &viewer, nil
}
func (s *showcaseHTTPStore) FindOwned(_ context.Context, owner, id primitive.ObjectID) (*domain.BaroCharacter, error) {
	character, ok := s.owned[id]
	if !ok || character.OwnerID != owner {
		return nil, nil
	}
	return &character, nil
}
func (s *showcaseHTTPStore) Save(_ context.Context, owner primitive.ObjectID, id *primitive.ObjectID, message string, at time.Time) error {
	if id == nil {
		delete(s.entries, owner)
		return nil
	}
	viewer := s.viewers[owner]
	entry := s.entries[owner]
	entry.OwnerID = owner.Hex()
	entry.Cohort = viewer.Cohort
	entry.Team = "Alpha"
	entry.Character = s.owned[*id]
	entry.Message = message
	entry.UpdatedAt = at
	s.entries[owner] = entry
	return nil
}
func (s *showcaseHTTPStore) List(_ context.Context, cohort int, team string, includeHidden bool, viewerID primitive.ObjectID) ([]showcase.Entry, error) {
	items := []showcase.Entry{}
	for ownerID, entry := range s.entries {
		if (cohort == 0 || cohort == entry.Cohort) && (team == "" || team == entry.Team) && (includeHidden || !entry.Hidden) {
			entry.Reactions = []showcase.ReactionSummary{{Emoji: "❤️", Count: 0, Reacted: s.reactions[ownerID.Hex()+":"+viewerID.Hex()+":❤️"]}}
			for key, active := range s.reactions {
				if active && strings.HasPrefix(key, ownerID.Hex()+":") {
					entry.Reactions[0].Count++
				}
			}
			items = append(items, entry)
		}
	}
	return items, nil
}
func (s *showcaseHTTPStore) Own(_ context.Context, ownerID primitive.ObjectID) (*showcase.Entry, error) {
	entry, ok := s.entries[ownerID]
	if !ok {
		return nil, nil
	}
	return &entry, nil
}
func (s *showcaseHTTPStore) ReactionTarget(_ context.Context, ownerID primitive.ObjectID) (*showcase.Target, error) {
	entry, ok := s.entries[ownerID]
	if !ok {
		return nil, nil
	}
	return &showcase.Target{Cohort: entry.Cohort, Hidden: entry.Hidden}, nil
}
func (s *showcaseHTTPStore) ToggleReaction(_ context.Context, ownerID, actorID primitive.ObjectID, emoji string) (bool, error) {
	if s.reactions == nil {
		s.reactions = map[string]bool{}
	}
	key := ownerID.Hex() + ":" + actorID.Hex() + ":" + emoji
	s.reactions[key] = !s.reactions[key]
	return s.reactions[key], nil
}
func (s *showcaseHTTPStore) SetMood(_ context.Context, ownerID primitive.ObjectID, mood string, until time.Time) error {
	entry, ok := s.entries[ownerID]
	if !ok {
		return showcase.ErrEntryNotFound
	}
	entry.Mood, entry.MoodUntil = mood, &until
	s.entries[ownerID] = entry
	return nil
}
func (s *showcaseHTTPStore) Moderate(_ context.Context, ownerID, adminID primitive.ObjectID, hidden bool, reason string, at time.Time) error {
	entry, ok := s.entries[ownerID]
	if !ok {
		return showcase.ErrEntryNotFound
	}
	entry.Hidden = hidden
	s.entries[ownerID] = entry
	s.audit = append(s.audit, adminID.Hex()+":"+reason)
	return nil
}

func TestShowcaseAuthenticatedPinAndCohortVisibility(t *testing.T) {
	t.Setenv("JWT_SECRET_KEY", "test-character-secret")
	owner := primitive.NewObjectID()
	peer := primitive.NewObjectID()
	outsider := primitive.NewObjectID()
	adminID := primitive.NewObjectID()
	characterID := primitive.NewObjectID()
	store := &showcaseHTTPStore{
		viewers: map[primitive.ObjectID]showcase.Viewer{
			owner: {Cohort: 16, Role: "learner"}, peer: {Cohort: 16, Role: "learner"},
			outsider: {Cohort: 17, Role: "learner"}, adminID: {Role: "admin"},
		},
		owned:   map[primitive.ObjectID]domain.BaroCharacter{characterID: {ID: characterID, OwnerID: owner}},
		entries: map[primitive.ObjectID]showcase.Entry{},
	}
	h := NewShowcaseHandler(showcase.NewService(store))
	app := fiber.New()
	lawn := app.Group("/showcase-lawn", middleware.AuthMiddleware)
	lawn.Get("", h.List)
	lawn.Get("/me", h.Mine)
	lawn.Put("/me", h.Save)
	lawn.Delete("/me", h.Remove)
	lawn.Post("/:ownerId/reactions", h.React)
	admin := app.Group("/admin", middleware.AuthMiddleware, middleware.CheckAdminRole)
	admin.Put("/showcase-lawn/:ownerId/moderation", h.Moderate)
	ownerToken := characterToken(t, owner, "learner")
	if status, _ := characterRequest(t, app, http.MethodGet, "/showcase-lawn", ""); status != http.StatusUnauthorized {
		t.Fatalf("anonymous list status = %d", status)
	}
	if status, _ := characterRequest(t, app, http.MethodPut, "/showcase-lawn/me", ownerToken, `{"character_id":"`+characterID.Hex()+`","message":"`+strings.Repeat("x", 161)+`"}`); status != http.StatusBadRequest {
		t.Fatalf("overlong pin status = %d", status)
	}
	if status, _ := characterRequest(t, app, http.MethodPut, "/showcase-lawn/me", characterToken(t, peer, "learner"), `{"character_id":"`+characterID.Hex()+`","message":"mine"}`); status != http.StatusBadRequest {
		t.Fatalf("foreign pin status = %d", status)
	}
	if status, _ := characterRequest(t, app, http.MethodPut, "/showcase-lawn/me", ownerToken, `{"character_id":"`+characterID.Hex()+`","message":"Hello friends"}`); status != http.StatusOK {
		t.Fatalf("owner pin status = %d", status)
	}
	if status, _ := characterRequest(t, app, http.MethodGet, "/showcase-lawn?cohort=17", characterToken(t, peer, "learner")); status != http.StatusForbidden {
		t.Fatalf("cross-cohort list status = %d", status)
	}
	for _, test := range []struct {
		token string
		want  int
	}{
		{characterToken(t, peer, "learner"), 1},
		{characterToken(t, outsider, "learner"), 0},
		{characterToken(t, adminID, "admin"), 1},
	} {
		status, body := characterRequest(t, app, http.MethodGet, "/showcase-lawn", test.token)
		if status != http.StatusOK {
			t.Fatalf("list status = %d", status)
		}
		var entries []showcase.Entry
		if err := json.Unmarshal(body["data"], &entries); err != nil {
			t.Fatal(err)
		}
		if len(entries) != test.want {
			t.Fatalf("entries = %+v, want %d", entries, test.want)
		}
	}
	reactionPath := "/showcase-lawn/" + owner.Hex() + "/reactions"
	peerToken := characterToken(t, peer, "learner")
	if status, _ := characterRequest(t, app, http.MethodPost, reactionPath, characterToken(t, outsider, "learner"), `{"emoji":"❤️"}`); status != http.StatusNotFound {
		t.Fatalf("outsider reaction status = %d", status)
	}
	if status, _ := characterRequest(t, app, http.MethodPost, reactionPath, peerToken, `{"emoji":"❤️"}`); status != http.StatusOK {
		t.Fatalf("first reaction status = %d", status)
	}
	status, body := characterRequest(t, app, http.MethodGet, "/showcase-lawn", peerToken)
	var reactedEntries []showcase.Entry
	if status != http.StatusOK || json.Unmarshal(body["data"], &reactedEntries) != nil || len(reactedEntries) != 1 || reactedEntries[0].Reactions[0].Count != 1 || !reactedEntries[0].Reactions[0].Reacted {
		t.Fatalf("reaction list status=%d entries=%+v", status, reactedEntries)
	}
	if status, _ := characterRequest(t, app, http.MethodPost, reactionPath, peerToken, `{"emoji":"❤️"}`); status != http.StatusOK {
		t.Fatalf("reaction toggle-off status = %d", status)
	}
	moderationPath := "/admin/showcase-lawn/" + owner.Hex() + "/moderation"
	if status, _ := characterRequest(t, app, http.MethodPut, moderationPath, peerToken, `{"hidden":true,"reason":"unsafe"}`); status != http.StatusForbidden {
		t.Fatalf("learner moderation status = %d", status)
	}
	adminToken := characterToken(t, adminID, "admin")
	if status, _ := characterRequest(t, app, http.MethodPut, moderationPath, adminToken, `{"hidden":true,"reason":"unsafe"}`); status != http.StatusOK || len(store.audit) != 1 {
		t.Fatalf("admin hide status = %d audit=%+v", status, store.audit)
	}
	if status, _ := characterRequest(t, app, http.MethodPut, "/showcase-lawn/me", ownerToken, `{"character_id":"`+characterID.Hex()+`","message":"Edited while hidden"}`); status != http.StatusOK {
		t.Fatalf("owner edit status = %d", status)
	}
	if status, _ := characterRequest(t, app, http.MethodGet, "/showcase-lawn?include_hidden=true", peerToken); status != http.StatusForbidden {
		t.Fatalf("learner hidden list status = %d", status)
	}
	status, body = characterRequest(t, app, http.MethodGet, "/showcase-lawn/me", ownerToken)
	var own showcase.Entry
	if status != http.StatusOK || json.Unmarshal(body["data"], &own) != nil || !own.Hidden || own.Message != "Edited while hidden" {
		t.Fatalf("owner hidden entry status=%d entry=%+v", status, own)
	}
	status, body = characterRequest(t, app, http.MethodGet, "/showcase-lawn", peerToken)
	var publicEntries []showcase.Entry
	if status != http.StatusOK || json.Unmarshal(body["data"], &publicEntries) != nil || len(publicEntries) != 0 {
		t.Fatalf("public hidden list status=%d entries=%+v", status, publicEntries)
	}
	status, body = characterRequest(t, app, http.MethodGet, "/showcase-lawn?include_hidden=true", adminToken)
	var hiddenEntries []showcase.Entry
	if status != http.StatusOK || json.Unmarshal(body["data"], &hiddenEntries) != nil || len(hiddenEntries) != 1 || !hiddenEntries[0].Hidden || hiddenEntries[0].Message != "Edited while hidden" {
		t.Fatalf("admin hidden list status=%d entries=%+v", status, hiddenEntries)
	}
	if status, _ := characterRequest(t, app, http.MethodPost, reactionPath, peerToken, `{"emoji":"✨"}`); status != http.StatusNotFound {
		t.Fatalf("hidden reaction status = %d", status)
	}
	if status, _ := characterRequest(t, app, http.MethodPut, moderationPath, adminToken, `{"hidden":false,"reason":"restored"}`); status != http.StatusOK || len(store.audit) != 2 {
		t.Fatalf("admin restore status = %d audit=%+v", status, store.audit)
	}
	if status, _ := characterRequest(t, app, http.MethodDelete, "/showcase-lawn/me", ownerToken); status != http.StatusOK || len(store.entries) != 0 {
		t.Fatalf("remove status = %d, entries = %+v", status, store.entries)
	}
}

func TestShowcaseMoodIsSelfOnlyAndIgnoresTargets(t *testing.T) {
	t.Setenv("JWT_SECRET_KEY", "test-character-secret")
	owner := primitive.NewObjectID()
	peer := primitive.NewObjectID()
	characterID := primitive.NewObjectID()
	store := &showcaseHTTPStore{
		viewers: map[primitive.ObjectID]showcase.Viewer{owner: {Cohort: 16, Role: "learner"}, peer: {Cohort: 16, Role: "learner"}},
		owned:   map[primitive.ObjectID]domain.BaroCharacter{characterID: {ID: characterID, OwnerID: owner}},
		entries: map[primitive.ObjectID]showcase.Entry{owner: {OwnerID: owner.Hex(), Cohort: 16, Character: domain.BaroCharacter{ID: characterID, OwnerID: owner}}, peer: {OwnerID: peer.Hex(), Cohort: 16}},
	}
	h := NewShowcaseHandler(showcase.NewService(store))
	app := fiber.New()
	app.Put("/showcase-lawn/me/mood", middleware.AuthMiddleware, h.SetMood)
	if status, _ := characterRequest(t, app, http.MethodPut, "/showcase-lawn/me/mood", "", `{"mood":"playful"}`); status != http.StatusUnauthorized {
		t.Fatalf("anonymous mood status = %d", status)
	}
	if status, _ := characterRequest(t, app, http.MethodPut, "/showcase-lawn/me/mood", characterToken(t, owner, "learner"), `{"mood":"duel"}`); status != http.StatusBadRequest {
		t.Fatalf("invalid mood status = %d", status)
	}
	status, body := characterRequest(t, app, http.MethodPut, "/showcase-lawn/me/mood", characterToken(t, owner, "learner"), `{"mood":"playful","target_id":"`+peer.Hex()+`","owner_id":"`+peer.Hex()+`"}`)
	if status != http.StatusOK {
		t.Fatalf("mood status = %d", status)
	}
	var state showcase.MoodState
	if err := json.Unmarshal(body["data"], &state); err != nil || state.Mood != "playful" || state.Until == nil {
		t.Fatalf("mood state = %+v, %v", state, err)
	}
	if store.entries[owner].Mood != "playful" || store.entries[peer].Mood != "" {
		t.Fatalf("mood leaked to target: owner=%q peer=%q", store.entries[owner].Mood, store.entries[peer].Mood)
	}
	delete(store.entries, owner)
	if status, _ := characterRequest(t, app, http.MethodPut, "/showcase-lawn/me/mood", characterToken(t, owner, "learner"), `{"mood":"quiet"}`); status != http.StatusNotFound {
		t.Fatalf("unpinned mood status = %d", status)
	}
}
