package handler

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"gofiber-baro/internal/domain"
	"gofiber-baro/pkg/middleware"

	"github.com/gofiber/fiber/v2"
)

type fakeStaffLookup map[string]*domain.User

func (f fakeStaffLookup) GetUserByID(id string) (*domain.User, error) {
	if id == "boom" {
		return nil, errors.New("mongo is down")
	}
	if u, ok := f[id]; ok {
		return u, nil
	}
	return nil, domain.ErrUserNotFound
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
	g.Get("/learners/:id/overview", ok)
	return app
}

func status(t *testing.T, app *fiber.App, method, url string) int {
	t.Helper()
	return statusBody(t, app, method, url, "")
}

func statusBody(t *testing.T, app *fiber.App, method, url, body string) int {
	t.Helper()
	req := httptest.NewRequest(method, url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res, err := app.Test(req)
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

func TestStaffGuardScopeByLearnerAndBody(t *testing.T) {
	users := fakeStaffLookup{
		"t":    {Role: "admin", StaffRole: "ta", StaffCohorts: []int{13}},
		"o":    {Role: "admin"},
		"in13": {Role: "learner", CohortNumber: 13},
		"in14": {Role: "learner", CohortNumber: 14},
	}
	app := func(uid string) *fiber.App { return guardApp(uid, users) }
	if c := status(t, app("t"), "GET", "/admin/learners/in13/overview"); c != 200 {
		t.Errorf("own cohort learner: %d", c)
	}
	if c := status(t, app("t"), "GET", "/admin/learners/in14/overview"); c != 403 {
		t.Errorf("other cohort learner by id: %d", c)
	}
	if c := status(t, app("t"), "GET", "/admin/Learners/in14/overview"); c != 403 {
		t.Errorf("other cohort learner, odd case: %d", c)
	}
	if c := status(t, app("o"), "GET", "/admin/learners/in14/overview"); c != 200 {
		t.Errorf("owner: %d", c)
	}
	if c := statusBody(t, app("t"), "POST", "/admin/attendance/mark", `{"user_id":"in14","status":"present"}`); c != 403 {
		t.Errorf("mark a learner from another cohort: %d", c)
	}
	if c := statusBody(t, app("t"), "POST", "/admin/attendance/mark", `{"user_id":"in13","status":"present"}`); c != 200 {
		t.Errorf("mark own cohort: %d", c)
	}
	if c := statusBody(t, app("t"), "POST", "/admin/attendance/mark", `{"user_ids":["in13","in14"]}`); c != 403 {
		t.Errorf("bulk with one outsider: %d", c)
	}
}

func TestStaffGuardDatabaseErrorIsNotARemoval(t *testing.T) {
	if c := status(t, guardApp("boom", fakeStaffLookup{}), "GET", "/admin/today"); c != 500 {
		t.Errorf("lookup error should be 500, got %d", c)
	}
}
