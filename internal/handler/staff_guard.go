package handler

import (
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
		c.Locals("staff", staff.Ctx{UserID: claims.UserID, Name: u.FirstName, Role: role, Cohorts: u.StaffCohorts, Groups: u.StaffGroups})
		return c.Next()
	}
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
