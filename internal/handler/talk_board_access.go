package handler

import (
	"gofiber-baro/internal/service/staff"

	"github.com/gofiber/fiber/v2"
)

// boardActor is who is using the Talk Board: a learner (role "") or a staff member.
type boardActor struct {
	role      string // staff role from the database; "" for learners
	cohorts   []int  // staff cohorts (owners see every cohort)
	ownCohort int    // the user's own cohort_number
}

// boardCohortForRead: learners always read their own cohort; staff read a cohort in their scope.
// Owners asking for no cohort get every cohort (old pages); other staff get their first cohort.
func boardCohortForRead(a boardActor, requested int) (int, bool) {
	if a.role == "" {
		return a.ownCohort, true
	}
	if requested == 0 {
		if a.role == staff.Owner {
			return 0, true
		}
		if len(a.cohorts) > 0 {
			return a.cohorts[0], true
		}
		return 0, false
	}
	if !staff.InScope(a.role, a.cohorts, requested) {
		return 0, false
	}
	return requested, true
}

// boardCohortForPost: learners post to their own cohort; staff who may care for learners post to a
// cohort in their scope (the one sent, else their own).
func boardCohortForPost(a boardActor, requested int) (int, bool) {
	if a.role == "" {
		return a.ownCohort, true
	}
	if !staff.Can(a.role, staff.CapCare) {
		return 0, false
	}
	target := requested
	if target == 0 {
		target = a.ownCohort
	}
	if target <= 0 || !staff.InScope(a.role, a.cohorts, target) {
		return 0, false
	}
	return target, true
}

// boardMayModerate: staff with the care capability, in the post's cohort.
func boardMayModerate(a boardActor, postCohort int) bool {
	return a.role != "" && staff.Can(a.role, staff.CapCare) && staff.InScope(a.role, a.cohorts, postCohort)
}

// actor loads the caller from the database (the JWT role alone can't tell a Viewer from an Owner).
func (h *TalkBoardHandler) actor(c *fiber.Ctx) (boardActor, bool) {
	id, ok := c.Locals("userID").(string)
	if !ok {
		return boardActor{}, false
	}
	u, err := h.userService.GetUserByID(id)
	if err != nil || u == nil {
		return boardActor{}, false
	}
	return boardActor{role: staff.Effective(u.Role, u.StaffRole), cohorts: u.StaffCohorts, ownCohort: u.CohortNumber}, true
}
