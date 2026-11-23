package handler

import (
	"context"
	"strings"
	"testing"
	"time"

	"gofiber-baro/internal/domain"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestDescribeAction(t *testing.T) {
	cases := map[[2]string]string{
		{"POST", "/admin/attendance/mark"}:         "Marked attendance",
		{"POST", "/admin/attendance/:id/restore"}:  "Undid an attendance change",
		{"PATCH", "/admin/leave-requests/:id"}:     "Reviewed a leave request",
		{"PUT", "/admin/cohorts/:cohort/settings"}: "Changed check-in rules",
		{"PUT", "/admin/cohorts/:cohort/groups"}:   "Moved learners between groups",
		{"POST", "/admin/learners/:id/notes"}:      "Added a TA note",
		{"POST", "/admin/notifications"}:           "Sent an announcement",
		{"POST", "/admin/holidays"}:                "Added a holiday",
		{"POST", "/admin/cup/awards"}:              "Awarded Cup points",
		{"DELETE", "/admin/something/:id"}:         "DELETE /admin/something/:id",
	}
	for in, want := range cases {
		if got := DescribeAction(in[0], in[1]); got != want {
			t.Errorf("%v → %q, want %q", in, got, want)
		}
	}
}

type auditMem struct{ filter bson.M }

func (a *auditMem) Insert(context.Context, *domain.AuditLog) error { return nil }
func (a *auditMem) FindPage(_ context.Context, f bson.M, page, limit int) ([]domain.AuditLog, int64, error) {
	a.filter = f
	return []domain.AuditLog{{ActorID: primitive.NewObjectID(), ActorRole: "admin", Method: "POST", Route: "/admin/attendance/mark", Path: "/admin/attendance/mark", Status: 200, CreatedAt: time.Now(),
		Targets: []string{"6b0438bb46b0813c186aaf4b", "6b0000000000000000000000"}, Detail: map[string]string{"status": "late"}}}, 1, nil
}

func TestActivityFeed(t *testing.T) {
	m := &auditMem{}
	h := NewActivityHandler(m, careUsers{map[string]domain.User{"6b0438bb46b0813c186aaf4b": {FirstName: "Mint"}}})
	app := fiber.New()
	app.Get("/admin/activity", h.List)
	code, body := send(app, "GET", "/admin/activity", "")
	if code != 200 || !strings.Contains(body, `"action":"Marked attendance"`) {
		t.Fatalf("→ %d %s", code, body)
	}
	if !strings.Contains(body, `"targets":["Mint"]`) || !strings.Contains(body, `"facts":{"status":"late"}`) {
		t.Errorf("who it was for and what changed, only names that resolve: %s", body)
	}
	if m.filter["method"] == nil {
		t.Error("only writes (non-GET) should be listed")
	}
}
