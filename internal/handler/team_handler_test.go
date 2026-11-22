package handler

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gofiber-baro/internal/domain"
	"gofiber-baro/internal/service/staff"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type stubTeam struct {
	acceptErr error
	invited   string
}

func (s *stubTeam) Members() ([]domain.User, error) {
	return []domain.User{{ID: primitive.NewObjectID(), FirstName: "Kan", Role: "admin"}}, nil
}
func (s *stubTeam) Invites() ([]staff.Invite, error) { return nil, nil }
func (s *stubTeam) Invite(_ primitive.ObjectID, email, role string, _ []int, _ []string) (string, *staff.Invite, error) {
	s.invited = email + ":" + role
	return "tok", &staff.Invite{ID: primitive.NewObjectID(), Email: email, Role: role, ExpiresAt: time.Now()}, nil
}
func (s *stubTeam) Renew(primitive.ObjectID, primitive.ObjectID) (string, *staff.Invite, error) {
	return "", nil, staff.ErrInviteInvalid
}
func (s *stubTeam) CancelInvite(primitive.ObjectID, primitive.ObjectID) error { return nil }
func (s *stubTeam) Lookup(string) (*staff.Invite, bool, error) {
	return nil, false, staff.ErrInviteInvalid
}
func (s *stubTeam) Accept(string, string, string, string) error { return s.acceptErr }
func (s *stubTeam) Update(primitive.ObjectID, primitive.ObjectID, string, []int, []string) error {
	return staff.ErrSelf
}
func (s *stubTeam) Remove(primitive.ObjectID, primitive.ObjectID) error { return nil }

func teamApp(t *stubTeam) *fiber.App {
	app := fiber.New()
	h := NewTeamHandler(t)
	admin := app.Group("/admin", func(c *fiber.Ctx) error {
		c.Locals("staff", staff.Ctx{UserID: primitive.NewObjectID().Hex(), Role: staff.Owner})
		return c.Next()
	})
	admin.Get("/team", h.List)
	admin.Post("/team/invites", h.Invite)
	admin.Patch("/team/:id", h.Update)
	app.Get("/invites/:token", h.Lookup)
	app.Post("/invites/:token/accept", h.Accept)
	return app
}

func teamSend(app *fiber.App, method, url, body string) int {
	req := httptest.NewRequest(method, url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res, _ := app.Test(req)
	return res.StatusCode
}

func TestTeamHandler(t *testing.T) {
	s := &stubTeam{}
	app := teamApp(s)
	if c := teamSend(app, "GET", "/admin/team", ""); c != 200 {
		t.Errorf("list %d", c)
	}
	if c := teamSend(app, "POST", "/admin/team/invites", `{"email":"a@x.co","role":"ta","cohorts":[13],"groups":["B"]}`); c != 201 || s.invited != "a@x.co:ta" {
		t.Errorf("invite %d %s", c, s.invited)
	}
	if c := teamSend(app, "PATCH", "/admin/team/"+primitive.NewObjectID().Hex(), `{"role":"viewer"}`); c != 400 {
		t.Errorf("self update should 400, got %d", c)
	}
	if c := teamSend(app, "GET", "/invites/whatever", ""); c != 404 {
		t.Errorf("bad token %d", c)
	}
	s.acceptErr = staff.ErrWrongPassword
	if c := teamSend(app, "POST", "/invites/tok/accept", `{"password":"x"}`); c != 401 {
		t.Errorf("wrong password %d", c)
	}
	s.acceptErr = nil
	if c := teamSend(app, "POST", "/invites/tok/accept", `{"first_name":"M","password":"longenough"}`); c != 200 {
		t.Errorf("accept %d", c)
	}
}
