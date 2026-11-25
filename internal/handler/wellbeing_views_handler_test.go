package handler

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gofiber-baro/internal/domain"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type wbUsersFake struct{ list []domain.User }

func (u wbUsersFake) ListCohortLearners(int, bson.M) ([]domain.User, error) {
	return u.list, nil
}
func (u wbUsersFake) GetUserByID(id string) (*domain.User, error) {
	for _, x := range u.list {
		if x.ID.Hex() == id {
			return &x, nil
		}
	}
	return nil, context.Canceled
}

type careReadFake struct{}

func (careReadFake) Notes(context.Context, primitive.ObjectID) ([]domain.AdminNote, error) {
	return []domain.AdminNote{{Text: "hello"}}, nil
}
func (careReadFake) Items(context.Context, primitive.ObjectID) ([]domain.SupportItem, error) {
	return nil, nil
}
func (careReadFake) ItemsFor(context.Context, []primitive.ObjectID) ([]domain.SupportItem, error) {
	return nil, nil
}
func (careReadFake) AcksFor(context.Context, []primitive.ObjectID, time.Time) ([]domain.SupportAck, error) {
	return nil, nil
}

func wbApp(users []domain.User) *fiber.App {
	h := NewWellbeingViewsHandler(wbUsersFake{users}, avRecords{}, avHolidays{}, careReadFake{})
	loc, _ := time.LoadLocation("Asia/Bangkok")
	h.now = func() time.Time { return time.Date(2026, 9, 9, 18, 0, 0, 0, loc) }
	app := fiber.New()
	app.Get("/admin/reflections/board", h.Board)
	app.Get("/admin/learners", h.Learners)
	app.Get("/admin/learners/:id/overview", h.Overview)
	app.Get("/admin/support", h.Support)
	app.Get("/admin/reflections/weeks", h.Weeks)
	return app
}

func TestWellbeingViews(t *testing.T) {
	u := domain.User{ID: primitive.NewObjectID(), FirstName: "Mint", Role: "learner", Reflections: []domain.Reflection{
		{Day: "2026-09-08", ReflectionData: domain.ReflectionContent{Barometer: "Panic Zone"}},
		{Day: "2026-09-09", ReflectionData: domain.ReflectionContent{Barometer: "Panic Zone"}},
	}}
	app := wbApp([]domain.User{u})
	cases := map[string]string{
		"/admin/reflections/board?cohort=13":          `"date":"2026-09-09"`,
		"/admin/learners?cohort=13":                   `"support":true`,
		"/admin/learners/" + u.ID.Hex() + "/overview": `"panic_run"`,
		"/admin/support?cohort=13":                    `"pattern:panic_run"`,
		"/admin/reflections/weeks?cohort=13&weeks=2":  `"week":"2026-09-07"`,
	}
	for url, want := range cases {
		code, body := send(app, "GET", url, "")
		if code != 200 || !strings.Contains(body, want) {
			t.Errorf("%s → %d, want body containing %s: %s", url, code, want, body)
		}
	}
	for _, bad := range []string{"/admin/reflections/weeks", "/admin/reflections/weeks?cohort=13&weeks=99", "/admin/reflections/board", "/admin/reflections/board?cohort=13&range=year", "/admin/learners/" + primitive.NewObjectID().Hex() + "/overview"} {
		if code, _ := send(app, "GET", bad, ""); code != 400 && code != 404 {
			t.Errorf("%s → %d, want 400/404", bad, code)
		}
	}
	_ = httptest.NewRequest
}
