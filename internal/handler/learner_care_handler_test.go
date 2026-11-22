package handler

import (
	"context"
	"errors"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gofiber-baro/internal/domain"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type careMem struct {
	notes []domain.AdminNote
	items []domain.SupportItem
	acks  []domain.SupportAck
}

func (m *careMem) AddNote(_ context.Context, n *domain.AdminNote) error {
	n.ID = primitive.NewObjectID()
	m.notes = append(m.notes, *n)
	return nil
}
func (m *careMem) Notes(_ context.Context, u primitive.ObjectID) ([]domain.AdminNote, error) {
	out := []domain.AdminNote{}
	for _, n := range m.notes {
		if n.UserID == u {
			out = append(out, n)
		}
	}
	return out, nil
}
func (m *careMem) Note(_ context.Context, id primitive.ObjectID) (*domain.AdminNote, error) {
	for _, n := range m.notes {
		if n.ID == id {
			return &n, nil
		}
	}
	return nil, errors.New("not found")
}
func (m *careMem) DeleteNote(_ context.Context, id primitive.ObjectID) error { return nil }
func (m *careMem) AddItem(_ context.Context, it *domain.SupportItem) error {
	it.ID = primitive.NewObjectID()
	m.items = append(m.items, *it)
	return nil
}
func (m *careMem) Items(_ context.Context, u primitive.ObjectID) ([]domain.SupportItem, error) {
	return m.items, nil
}
func (m *careMem) SetItemDone(_ context.Context, id primitive.ObjectID, done bool) error {
	for i := range m.items {
		if m.items[i].ID == id {
			m.items[i].Done = done
			return nil
		}
	}
	return errors.New("not found")
}
func (m *careMem) DeleteItem(_ context.Context, id primitive.ObjectID) error { return nil }
func (m *careMem) Ack(_ context.Context, a *domain.SupportAck) error {
	m.acks = append(m.acks, *a)
	return nil
}
func (m *careMem) Acks(_ context.Context, u primitive.ObjectID, now time.Time) ([]domain.SupportAck, error) {
	return m.acks, nil
}

type careUsers struct{ known map[string]domain.User }

func (u careUsers) GetUserByID(id string) (*domain.User, error) {
	if x, ok := u.known[id]; ok {
		return &x, nil
	}
	return nil, errors.New("not found")
}

func careApp(m *careMem, learner, me primitive.ObjectID) *fiber.App {
	h := NewLearnerCareHandler(m, careUsers{map[string]domain.User{learner.Hex(): {ID: learner, FirstName: "Mint"}, me.Hex(): {ID: me, FirstName: "Kan"}}})
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error { c.Locals("userID", me.Hex()); return c.Next() })
	app.Get("/admin/learners/:id/notes", h.Notes)
	app.Post("/admin/learners/:id/notes", h.AddNote)
	app.Delete("/admin/notes/:noteId", h.DeleteNote)
	app.Get("/admin/learners/:id/support", h.Support)
	app.Post("/admin/learners/:id/support", h.AddSupport)
	app.Patch("/admin/support/:itemId", h.SetSupportDone)
	app.Post("/admin/learners/:id/support/ack", h.Ack)
	return app
}

func send(app *fiber.App, method, url, body string) (int, string) {
	req := httptest.NewRequest(method, url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res, _ := app.Test(req)
	b, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(b)
}

func TestLearnerCareNotes(t *testing.T) {
	m, learner, me := &careMem{}, primitive.NewObjectID(), primitive.NewObjectID()
	app := careApp(m, learner, me)
	if s, b := send(app, "POST", "/admin/learners/"+learner.Hex()+"/notes", `{"text":"Paired with Fah"}`); s != 201 || !strings.Contains(b, `"author_name":"Kan"`) {
		t.Fatalf("add note → %d %s", s, b)
	}
	if s, b := send(app, "GET", "/admin/learners/"+learner.Hex()+"/notes", ""); s != 200 || !strings.Contains(b, "Paired with Fah") {
		t.Errorf("list notes → %d %s", s, b)
	}
	if s, _ := send(app, "POST", "/admin/learners/"+learner.Hex()+"/notes", `{"text":"  "}`); s != 400 {
		t.Errorf("empty note → %d, want 400", s)
	}
	if s, _ := send(app, "POST", "/admin/learners/"+primitive.NewObjectID().Hex()+"/notes", `{"text":"x"}`); s != 404 {
		t.Errorf("unknown learner → %d, want 404", s)
	}
	other := domain.AdminNote{ID: primitive.NewObjectID(), UserID: learner, AuthorID: primitive.NewObjectID(), Text: "theirs"}
	m.notes = append(m.notes, other)
	if s, _ := send(app, "DELETE", "/admin/notes/"+other.ID.Hex(), ""); s != 403 {
		t.Errorf("delete someone else's note → %d, want 403", s)
	}
}

func TestLearnerCareSupport(t *testing.T) {
	m, learner, me := &careMem{}, primitive.NewObjectID(), primitive.NewObjectID()
	app := careApp(m, learner, me)
	if s, _ := send(app, "POST", "/admin/learners/"+learner.Hex()+"/support", `{"text":"1:1 check-in"}`); s != 201 || len(m.items) != 1 {
		t.Fatalf("add support item → %d", s)
	}
	if s, _ := send(app, "PATCH", "/admin/support/"+m.items[0].ID.Hex(), `{"done":true}`); s != 200 || !m.items[0].Done {
		t.Errorf("toggle done → %d %+v", s, m.items[0])
	}
	if s, _ := send(app, "POST", "/admin/learners/"+learner.Hex()+"/support/ack", `{"reason_key":"leave","reason_text":"Leave 6/8"}`); s != 201 || len(m.acks) != 1 || m.acks[0].Until.Before(time.Now().Add(6*24*time.Hour)) {
		t.Errorf("ack → %d %+v", s, m.acks)
	}
}
