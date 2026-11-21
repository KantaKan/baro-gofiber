package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"gofiber-baro/internal/domain"
	"gofiber-baro/internal/service/character"
	"gofiber-baro/pkg/middleware"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type characterHTTPStore struct {
	mu         sync.Mutex
	starters   map[primitive.ObjectID]domain.BaroCharacter
	extras     map[primitive.ObjectID][]domain.BaroCharacter
	selections map[primitive.ObjectID]domain.CharacterSelection
}

type characterGrowthUsers struct {
	users map[primitive.ObjectID]*domain.User
}

func (s characterGrowthUsers) FindByID(_ interface{}, id primitive.ObjectID) (*domain.User, error) {
	return s.users[id], nil
}

type characterGrowthHolidays struct{}

func (characterGrowthHolidays) GetHolidayDatesInRange(_, _ string) (map[string]bool, error) {
	return map[string]bool{}, nil
}

func (s *characterHTTPStore) AccountExists(context.Context, primitive.ObjectID) (bool, error) {
	return true, nil
}
func (s *characterHTTPStore) FindStarter(_ context.Context, id primitive.ObjectID) (*domain.BaroCharacter, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.starters[id]
	if !ok {
		return nil, nil
	}
	return &value, nil
}
func (s *characterHTTPStore) ListForOwner(_ context.Context, id primitive.ObjectID) ([]domain.BaroCharacter, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.starters[id]
	items := append([]domain.BaroCharacter{}, s.extras[id]...)
	if ok {
		items = append([]domain.BaroCharacter{value}, items...)
	}
	return items, nil
}

func (s *characterHTTPStore) FindOwned(_ context.Context, owner, id primitive.ObjectID) (*domain.BaroCharacter, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if item, exists := s.starters[owner]; exists && item.ID == id {
		return &item, nil
	}
	for _, item := range s.extras[owner] {
		if item.ID == id {
			found := item
			return &found, nil
		}
	}
	return nil, nil
}
func (s *characterHTTPStore) InsertCharacter(_ context.Context, item domain.BaroCharacter) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.extras[item.OwnerID] = append(s.extras[item.OwnerID], item)
	return nil
}
func (s *characterHTTPStore) ReadSelection(_ context.Context, owner primitive.ObjectID) (domain.CharacterSelection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.selections[owner], nil
}
func (s *characterHTTPStore) SetEquipped(_ context.Context, owner, id primitive.ObjectID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	item := s.selections[owner]
	item.EquippedID = id.Hex()
	s.selections[owner] = item
	return nil
}
func (s *characterHTTPStore) SetPinned(_ context.Context, owner primitive.ObjectID, id *primitive.ObjectID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	item := s.selections[owner]
	item.PinnedID = ""
	if id != nil {
		item.PinnedID = id.Hex()
	}
	s.selections[owner] = item
	return nil
}
func (s *characterHTTPStore) InsertStarter(_ context.Context, value domain.BaroCharacter) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.starters[value.OwnerID]; exists {
		return domain.ErrCharacterConflict
	}
	s.starters[value.OwnerID] = value
	return nil
}

func characterToken(t *testing.T, owner primitive.ObjectID, role string) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, &middleware.Claims{UserID: owner.Hex(), Role: role})
	signed, err := token.SignedString([]byte("test-character-secret"))
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

func characterRequest(t *testing.T, app *fiber.App, method, path, token string, payload ...string) (int, map[string]json.RawMessage) {
	t.Helper()
	requestBody := ""
	if len(payload) > 0 {
		requestBody = payload[0]
	}
	req := httptest.NewRequest(method, path, strings.NewReader(requestBody))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body := map[string]json.RawMessage{}
	if response.StatusCode >= 400 {
		return response.StatusCode, body
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	return response.StatusCode, body
}

func TestBaroCharacterAuthenticatedRevealAndCollection(t *testing.T) {
	t.Setenv("JWT_SECRET_KEY", "test-character-secret")
	store := &characterHTTPStore{starters: map[primitive.ObjectID]domain.BaroCharacter{}, extras: map[primitive.ObjectID][]domain.BaroCharacter{}, selections: map[primitive.ObjectID]domain.CharacterSelection{}}
	service := character.NewService(store, character.SecurePicker{})
	learner := primitive.NewObjectID()
	admin := primitive.NewObjectID()
	growth := character.NewGrowthService(characterGrowthUsers{users: map[primitive.ObjectID]*domain.User{
		learner: {ID: learner, Reflections: []domain.Reflection{{Day: "2026-09-25"}, {Day: "2026-09-28"}}},
		admin:   {ID: admin},
	}}, characterGrowthHolidays{})
	ownership := character.NewOwnershipService(store, character.SecurePicker{})
	handler := NewBaroCharacterHandler(service, growth, ownership)
	app := fiber.New()
	group := app.Group("/baro-characters", middleware.AuthMiddleware)
	group.Get("", handler.Collection)
	group.Get("/growth", handler.Growth)
	group.Get("/selection", handler.Selection)
	group.Put("/equipped", handler.Equip)
	group.Put("/pinned", handler.Pin)
	group.Post("/reveal", handler.RevealStarter)
	adminGroup := app.Group("/admin", middleware.AuthMiddleware, middleware.CheckAdminRole)
	adminGroup.Post("/users/:id/baro-characters", handler.AdminGrant)
	adminGroup.Put("/users/:id/baro-characters/equipped", handler.AdminEquip)
	adminGroup.Put("/users/:id/baro-characters/pinned", handler.AdminPin)

	if status, _ := characterRequest(t, app, http.MethodGet, "/baro-characters", ""); status != http.StatusUnauthorized {
		t.Fatalf("anonymous collection status = %d", status)
	}
	for _, account := range []struct {
		id   primitive.ObjectID
		role string
	}{{learner, "learner"}, {admin, "admin"}} {
		token := characterToken(t, account.id, account.role)
		status, first := characterRequest(t, app, http.MethodPost, "/baro-characters/reveal", token)
		if status != http.StatusOK {
			t.Fatalf("%s reveal status = %d", account.role, status)
		}
		var firstCharacter domain.BaroCharacter
		if err := json.Unmarshal(first["data"], &firstCharacter); err != nil {
			t.Fatal(err)
		}
		if firstCharacter.OwnerID != account.id || firstCharacter.Serial == "" || firstCharacter.Fingerprint == "" {
			t.Fatalf("bad character: %+v", firstCharacter)
		}
		_, repeat := characterRequest(t, app, http.MethodPost, "/baro-characters/reveal", token)
		var repeated domain.BaroCharacter
		if err := json.Unmarshal(repeat["data"], &repeated); err != nil {
			t.Fatal(err)
		}
		if repeated.ID != firstCharacter.ID {
			t.Fatalf("%s starter changed", account.role)
		}
		_, collection := characterRequest(t, app, http.MethodGet, "/baro-characters", token)
		var owned []domain.BaroCharacter
		if err := json.Unmarshal(collection["data"], &owned); err != nil {
			t.Fatal(err)
		}
		if len(owned) != 1 || owned[0].OwnerID != account.id {
			t.Fatalf("%s collection = %+v", account.role, owned)
		}
		growthStatus, growthBody := characterRequest(t, app, http.MethodGet, "/baro-characters/growth", token)
		if growthStatus != http.StatusOK {
			t.Fatalf("%s growth status = %d", account.role, growthStatus)
		}
		var snapshot domain.CharacterGrowthSnapshot
		if err := json.Unmarshal(growthBody["data"], &snapshot); err != nil {
			t.Fatal(err)
		}
		if account.role == "admin" && snapshot.BestStreak != 0 {
			t.Fatalf("admin growth = %+v", snapshot)
		}
	}
}

func TestBaroCharacterOwnershipAndAdminOverrides(t *testing.T) {
	t.Setenv("JWT_SECRET_KEY", "test-character-secret")
	learner := primitive.NewObjectID()
	other := primitive.NewObjectID()
	admin := primitive.NewObjectID()
	store := &characterHTTPStore{starters: map[primitive.ObjectID]domain.BaroCharacter{}, extras: map[primitive.ObjectID][]domain.BaroCharacter{}, selections: map[primitive.ObjectID]domain.CharacterSelection{}}
	service := character.NewService(store, character.SecurePicker{})
	ownership := character.NewOwnershipService(store, character.SecurePicker{})
	handler := NewBaroCharacterHandler(service, character.NewGrowthService(characterGrowthUsers{users: map[primitive.ObjectID]*domain.User{}}, characterGrowthHolidays{}), ownership)
	app := fiber.New()
	group := app.Group("/baro-characters", middleware.AuthMiddleware)
	group.Get("/selection", handler.Selection)
	group.Put("/equipped", handler.Equip)
	group.Put("/pinned", handler.Pin)
	group.Post("/reveal", handler.RevealStarter)
	adminGroup := app.Group("/admin", middleware.AuthMiddleware, middleware.CheckAdminRole)
	adminGroup.Post("/users/:id/baro-characters", handler.AdminGrant)
	adminGroup.Put("/users/:id/baro-characters/equipped", handler.AdminEquip)
	adminGroup.Put("/users/:id/baro-characters/pinned", handler.AdminPin)
	learnerToken := characterToken(t, learner, "learner")
	otherToken := characterToken(t, other, "learner")
	adminToken := characterToken(t, admin, "admin")
	if status, _ := characterRequest(t, app, http.MethodPost, "/admin/users/"+learner.Hex()+"/baro-characters", learnerToken); status != http.StatusForbidden {
		t.Fatalf("learner admin-grant status = %d", status)
	}
	_, original := characterRequest(t, app, http.MethodPost, "/baro-characters/reveal", learnerToken)
	var starter domain.BaroCharacter
	if err := json.Unmarshal(original["data"], &starter); err != nil {
		t.Fatal(err)
	}
	status, grant := characterRequest(t, app, http.MethodPost, "/admin/users/"+learner.Hex()+"/baro-characters", adminToken)
	if status != http.StatusOK {
		t.Fatalf("admin grant status = %d", status)
	}
	var gifted domain.BaroCharacter
	if err := json.Unmarshal(grant["data"], &gifted); err != nil {
		t.Fatal(err)
	}
	if gifted.ID == starter.ID || gifted.OwnerID != learner || gifted.IsStarter {
		t.Fatalf("gifted = %+v", gifted)
	}
	if status, _ := characterRequest(t, app, http.MethodPut, "/baro-characters/equipped", otherToken, `{"character_id":"`+gifted.ID.Hex()+`"}`); status != http.StatusBadRequest {
		t.Fatalf("foreign equip status = %d", status)
	}
	if status, _ := characterRequest(t, app, http.MethodPut, "/baro-characters/pinned", learnerToken, `{"character_id":"`+gifted.ID.Hex()+`"}`); status != http.StatusOK {
		t.Fatalf("pin status = %d", status)
	}
	if status, _ := characterRequest(t, app, http.MethodPut, "/admin/users/"+learner.Hex()+"/baro-characters/equipped", adminToken, `{"character_id":"`+starter.ID.Hex()+`"}`); status != http.StatusOK {
		t.Fatalf("admin equip status = %d", status)
	}
	_, response := characterRequest(t, app, http.MethodGet, "/baro-characters/selection", learnerToken)
	var selection domain.CharacterSelection
	if err := json.Unmarshal(response["data"], &selection); err != nil {
		t.Fatal(err)
	}
	if selection.EquippedID != starter.ID.Hex() || selection.PinnedID != gifted.ID.Hex() {
		t.Fatalf("selection = %+v", selection)
	}
	if status, _ := characterRequest(t, app, http.MethodPut, "/admin/users/"+learner.Hex()+"/baro-characters/pinned", adminToken, `{"character_id":""}`); status != http.StatusOK {
		t.Fatalf("admin unpin status = %d", status)
	}
}
