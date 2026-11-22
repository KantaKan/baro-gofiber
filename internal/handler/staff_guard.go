package handler

import (
	"encoding/json"
	"errors"

	"gofiber-baro/internal/domain"
	"gofiber-baro/internal/service/staff"
	"gofiber-baro/pkg/middleware"
	"gofiber-baro/pkg/utils"

	"github.com/gofiber/fiber/v2"
)

type staffLookup interface {
	GetUserByID(id string) (*domain.User, error)
}

// StaffGuard enforces staff roles on /admin: it re-reads the user on every request, so a role
// change or removal applies at once, then checks the capability and the cohort the request names.
func StaffGuard(users staffLookup) fiber.Handler {
	return func(c *fiber.Ctx) error {
		claims, ok := c.Locals("user").(*middleware.Claims)
		if !ok {
			return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
		}
		u, err := users.GetUserByID(claims.UserID)
		if err != nil && !errors.Is(err, domain.ErrUserNotFound) {
			// A database hiccup is not a removal; don't tell people their access ended.
			return utils.SendError(c, fiber.StatusInternalServerError, "Could not check your access, try again")
		}
		if err != nil || u == nil {
			return utils.SendError(c, fiber.StatusForbidden, "Your staff access has ended")
		}
		role := staff.Effective(u.Role, u.StaffRole)
		if role == "" {
			return utils.SendError(c, fiber.StatusForbidden, "Your staff access has ended")
		}
		if !staff.Can(role, staff.RequiredCap(c.Method(), c.Path())) {
			return utils.SendError(c, fiber.StatusForbidden, "Your role can't do this")
		}
		if n, has := staff.CohortFromRequest(c.Path(), func(k string) string { return c.Query(k) }); has && !staff.InScope(role, u.StaffCohorts, n) {
			return utils.SendError(c, fiber.StatusForbidden, "You don't have access to this cohort")
		}
		if role != staff.Owner && !learnersInScope(c, users, role, u.StaffCohorts) {
			return utils.SendError(c, fiber.StatusForbidden, "You don't have access to this learner's cohort")
		}
		c.Locals("staff", staff.Ctx{UserID: claims.UserID, Name: u.FirstName, Role: role, Cohorts: u.StaffCohorts, Groups: u.StaffGroups})
		return c.Next()
	}
}

// learnersInScope: for non-owners, every learner a request names (in the path, or as user_id /
// user_ids in the JSON body) must be in one of their cohorts. Unknown ids pass; the handler 404s.
func learnersInScope(c *fiber.Ctx, users staffLookup, role string, cohorts []int) bool {
	ids := []string{}
	if id, ok := staff.LearnerFromPath(c.Path()); ok {
		ids = append(ids, id)
	}
	if m := c.Method(); m == "POST" || m == "PUT" || m == "PATCH" || m == "DELETE" {
		var b struct {
			UserID  string   `json:"user_id"`
			UserIDs []string `json:"user_ids"`
		}
		if json.Unmarshal(c.Body(), &b) == nil {
			if b.UserID != "" {
				ids = append(ids, b.UserID)
			}
			ids = append(ids, b.UserIDs...)
		}
	}
	for _, id := range ids {
		l, err := users.GetUserByID(id)
		if err != nil || l == nil {
			continue
		}
		if l.Role == "learner" && !staff.InScope(role, cohorts, l.CohortNumber) {
			return false
		}
	}
	return true
}

// StaffFrom returns the signed-in staff member set by StaffGuard.
func StaffFrom(c *fiber.Ctx) (staff.Ctx, bool) {
	s, ok := c.Locals("staff").(staff.Ctx)
	return s, ok
}

// StaffMe tells the admin UI who is signed in and what they may see.
// @Summary  Signed-in staff member  @Tags admin  @Security BearerAuth  @Router /admin/me [get]
func StaffMe(c *fiber.Ctx) error {
	s, ok := StaffFrom(c)
	if !ok {
		return utils.SendError(c, fiber.StatusForbidden, "Not staff")
	}
	cohorts, groups := s.Cohorts, s.Groups
	if cohorts == nil {
		cohorts = []int{}
	}
	if groups == nil {
		groups = []string{}
	}
	return utils.SendResponse(c, fiber.StatusOK, "ok", fiber.Map{"id": s.UserID, "first_name": s.Name, "staff_role": s.Role, "cohorts": cohorts, "groups": groups})
}
