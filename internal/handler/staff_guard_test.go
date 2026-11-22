package handler

import (
	"errors"
	"net/http/httptest"
	"testing"

	"gofiber-baro/internal/domain"
	"gofiber-baro/pkg/middleware"

	"github.com/gofiber/fiber/v2"
)

type fakeStaffLookup map[string]*domain.User

func (f fakeStaffLookup) GetUserByID(id string) (*domain.User, error) {
	if u, ok := f[id]; ok {
		return u, nil
	}
	return nil, errors.New("not found")
}

func guardApp(uid string, users fakeStaffLookup) *fiber.App {
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", &middleware.Claims{UserID: uid, Role: "admin"})
		return c.Next()
	})
	g := app.Group("/admin", StaffGuard(users))
	ok := func(c *fiber.Ctx) error { return c.SendStatus(200) }
	g.Get("/today", ok)
	g.Post("/attendance/mark", ok)
	g.Get("/cohorts/:cohort/settings", ok)
	g.Get("/me", StaffMe)
	return app
}

func status(t *testing.T, app *fiber.App, method, url string) int {
	t.Helper()
	res, err := app.Test(httptest.NewRequest(method, url, nil))
	if err != nil {
		t.Fatal(err)
	}
	return res.StatusCode
}

func TestStaffGuard(t *testing.T) {
	users := fakeStaffLookup{
		"v": {Role: "admin", StaffRole: "viewer", StaffCohorts: []int{13}},
		"t": {Role: "admin", StaffRole: "ta", StaffCohorts: []int{13}},
		"o": {Role: "admin"},
		"x": {Role: "disabled", StaffRole: "ta"},
	}
	cases := []struct {
		uid, method, url string
		want             int
	}{
		{"v", "GET", "/admin/today?cohort=13", 200},
		{"v", "POST", "/admin/attendance/mark", 403},
		{"t", "POST", "/admin/attendance/mark", 200},
		{"t", "GET", "/admin/today?cohort=14", 403},
		{"t", "GET", "/admin/cohorts/14/settings", 403},
		{"t", "GET", "/admin/cohorts/13/settings", 200},
		{"o", "GET", "/admin/cohorts/99/settings", 200},
		{"x", "GET", "/admin/today", 403},
		{"gone", "GET", "/admin/today", 403},
		{"t", "GET", "/admin/me", 200},
	}
	for _, c := range cases {
		if got := status(t, guardApp(c.uid, users), c.method, c.url); got != c.want {
			t.Errorf("%s %s %s = %d, want %d", c.uid, c.method, c.url, got, c.want)
		}
	}
}
