package handler

import (
	"testing"

	"gofiber-baro/internal/domain"
)

func TestCleanUserUpdate(t *testing.T) {
	for _, k := range []string{"role", "staff_role", "staff_cohorts", "staff_groups", "_id", "Role", "STAFF_ROLE"} {
		if _, ok := cleanUserUpdate(map[string]interface{}{k: "owner", "first_name": "x"}); ok {
			t.Errorf("%q must be refused", k)
		}
	}
	got, ok := cleanUserUpdate(map[string]interface{}{"first_name": "Mint", "genmate_group": "B"})
	if !ok || got["first_name"] != "Mint" {
		t.Error("ordinary fields must pass")
	}
	if _, ok := cleanUserUpdate(map[string]interface{}{}); ok {
		t.Error("empty update is refused")
	}
}

func TestMayEditUsers(t *testing.T) {
	if !mayEditUsers(&domain.User{Role: "admin"}) || !mayEditUsers(&domain.User{Role: "admin", StaffRole: "admin"}) {
		t.Error("owner and admin may")
	}
	for _, u := range []*domain.User{{Role: "admin", StaffRole: "ta"}, {Role: "admin", StaffRole: "viewer"}, {Role: "disabled", StaffRole: "admin"}, {Role: "learner"}, nil} {
		if mayEditUsers(u) {
			t.Errorf("%+v must not edit users", u)
		}
	}
}
