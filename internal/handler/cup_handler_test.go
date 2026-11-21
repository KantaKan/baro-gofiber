package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"gofiber-baro/internal/domain"
	"gofiber-baro/internal/service/cup"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type cupMem struct{ awards []domain.CupAward }

func (m *cupMem) Insert(_ context.Context, a *domain.CupAward) error {
	a.ID = primitive.NewObjectID()
	m.awards = append(m.awards, *a)
	return nil
}
func (m *cupMem) InsertOnce(ctx context.Context, a *domain.CupAward) (bool, error) {
	return true, m.Insert(ctx, a)
}
func (m *cupMem) List(context.Context, int, string) ([]domain.CupAward, error) { return m.awards, nil }
func (m *cupMem) Delete(context.Context, primitive.ObjectID) error             { return nil }
func (m *cupMem) InsertWinOnce(context.Context, *domain.CupWin) (bool, error)  { return true, nil }
func (m *cupMem) Wins(context.Context, int) ([]domain.CupWin, error)           { return nil, nil }
func (m *cupMem) Groups(context.Context, int) (map[string][]primitive.ObjectID, error) {
	return map[string][]primitive.ObjectID{"A": {primitive.NewObjectID()}, "B": {}}, nil
}

type cupUser struct{ u domain.User }

func (c cupUser) GetUserByID(string) (*domain.User, error) { return &c.u, nil }

func cupApp(store *cupMem, me domain.User) *fiber.App {
	h := NewCupHandler(cup.NewService(store, nil), cupUser{me})
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error { c.Locals("userID", me.ID.Hex()); return c.Next() })
	app.Get("/cup/me", h.Me)
	app.Post("/admin/cup/awards", h.Award)
	return app
}

func TestCupMe(t *testing.T) {
	me := domain.User{ID: primitive.NewObjectID(), CohortNumber: 13, GenmateGroup: "A"}
	res, _ := cupApp(&cupMem{}, me).Test(httptest.NewRequest("GET", "/cup/me", nil))
	if res.StatusCode != 200 {
		t.Fatalf("status %d", res.StatusCode)
	}
	var body struct {
		Data cup.MyCup `json:"data"`
	}
	b, _ := io.ReadAll(res.Body)
	if err := json.Unmarshal(b, &body); err != nil || body.Data.MyGroup != "A" || len(body.Data.Standings) != 2 {
		t.Errorf("body = %s", b)
	}
}

func TestCupAward(t *testing.T) {
	store := &cupMem{}
	admin := domain.User{ID: primitive.NewObjectID(), CohortNumber: 13, Role: "admin"}
	app := cupApp(store, admin)
	req := httptest.NewRequest("POST", "/admin/cup/awards", strings.NewReader(`{"cohort":13,"group":"A","category":"coding","points":20,"reason":"Demo"}`))
	req.Header.Set("Content-Type", "application/json")
	res, _ := app.Test(req)
	if res.StatusCode != 201 || len(store.awards) != 1 {
		t.Fatalf("status %d awards %d", res.StatusCode, len(store.awards))
	}
	bad := httptest.NewRequest("POST", "/admin/cup/awards", strings.NewReader(`{"cohort":13,"group":"A","category":"showing_up","points":20,"reason":"x"}`))
	bad.Header.Set("Content-Type", "application/json")
	if res, _ := app.Test(bad); res.StatusCode != 400 {
		t.Errorf("showing_up by hand should be 400, got %d", res.StatusCode)
	}
}
