package handler

import (
	"context"
	"testing"

	"gofiber-baro/internal/domain"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type groupMem struct {
	users map[string]domain.User
	set   map[string]string
}

func (g *groupMem) GetUserByID(id string) (*domain.User, error) {
	if u, ok := g.users[id]; ok {
		return &u, nil
	}
	return nil, context.Canceled
}
func (g *groupMem) SetGenmateGroup(_ context.Context, id primitive.ObjectID, group string) error {
	g.set[id.Hex()] = group
	return nil
}

func TestMoveGroups(t *testing.T) {
	a, other := primitive.NewObjectID(), primitive.NewObjectID()
	g := &groupMem{users: map[string]domain.User{a.Hex(): {ID: a, CohortNumber: 13, Role: "learner"}, other.Hex(): {ID: other, CohortNumber: 12, Role: "learner"}}, set: map[string]string{}}
	h := NewCohortAdminHandler(nil, g)
	app := fiber.New()
	app.Put("/admin/cohorts/:cohort/groups", h.MoveGroups)
	if code, b := send(app, "PUT", "/admin/cohorts/13/groups", `{"moves":[{"user_id":"`+a.Hex()+`","group":"C"}]}`); code != 200 || g.set[a.Hex()] != "C" {
		t.Fatalf("valid move → %d %s %v", code, b, g.set)
	}
	g.set = map[string]string{}
	if code, _ := send(app, "PUT", "/admin/cohorts/13/groups", `{"moves":[{"user_id":"`+a.Hex()+`","group":"D"},{"user_id":"`+other.Hex()+`","group":"D"}]}`); code != 400 || len(g.set) != 0 {
		t.Errorf("learner from another cohort → %d, wrote %v (want 400, nothing written)", code, g.set)
	}
	if code, _ := send(app, "PUT", "/admin/cohorts/13/groups", `{"moves":[{"user_id":"`+a.Hex()+`","group":"zz"}]}`); code != 400 {
		t.Errorf("bad group → %d", code)
	}
}
