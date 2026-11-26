package handler

import (
	"strings"
	"testing"

	"gofiber-baro/internal/domain"
	userService "gofiber-baro/internal/service/user"
	"gofiber-baro/pkg/middleware"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// byIDRepo serves one user and records which loader the handler used.
type byIDRepo struct {
	domain.UserRepository
	user *domain.User
	full bool
	slim bson.M
}

func (r *byIDRepo) FindByID(interface{}, primitive.ObjectID) (*domain.User, error) {
	r.full = true
	u := *r.user
	return &u, nil
}

func (r *byIDRepo) FindByIDWithFields(_ interface{}, _ primitive.ObjectID, p bson.M) (*domain.User, error) {
	r.slim = p
	u := *r.user
	u.Reflections = nil
	return &u, nil
}

func TestGetUserByIDLoadsFullUserForAdminsOnly(t *testing.T) {
	id := primitive.NewObjectID()
	stored := &domain.User{ID: id, FirstName: "Mint", CohortNumber: 13, Reflections: []domain.Reflection{{Day: "2026-09-08"}}}
	cases := []struct {
		name   string
		claims middleware.Claims
		code   int
		full   bool
		reflec bool
	}{
		{"admin", middleware.Claims{UserID: primitive.NewObjectID().Hex(), Role: "admin"}, 200, true, true},
		{"owner", middleware.Claims{UserID: id.Hex(), Role: "learner", Cohort: 13}, 200, false, false},
		{"other cohort", middleware.Claims{UserID: primitive.NewObjectID().Hex(), Role: "learner", Cohort: 12}, 403, false, false},
	}
	for _, tc := range cases {
		repo := &byIDRepo{user: stored}
		h := &UserHandler{userService: userService.NewService(repo)}
		app := fiber.New()
		claims := tc.claims
		app.Get("/users/:id", func(c *fiber.Ctx) error { c.Locals("user", &claims); return c.Next() }, h.GetUserByID)
		code, body := send(app, "GET", "/users/"+id.Hex(), "")
		if code != tc.code {
			t.Errorf("%s → %d, want %d: %s", tc.name, code, tc.code, body)
			continue
		}
		if repo.full != tc.full {
			t.Errorf("%s: full user loaded = %v, want %v", tc.name, repo.full, tc.full)
		}
		if !tc.full && (repo.slim["reflections"] != 0 || repo.slim["password"] != 0) {
			t.Errorf("%s: slim projection = %v", tc.name, repo.slim)
		}
		if got := strings.Contains(body, `"reflections"`); code == 200 && got != tc.reflec {
			t.Errorf("%s: reflections in body = %v, want %v: %s", tc.name, got, tc.reflec, body)
		}
	}
}
