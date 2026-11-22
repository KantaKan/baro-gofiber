package handler

import (
	"gofiber-baro/internal/domain"
	"gofiber-baro/internal/service/staff"
	"gofiber-baro/pkg/utils"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
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
		// cohort 0 would mean "every cohort" to the repo
		return a.ownCohort, a.ownCohort > 0
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

// boardPostAccess: may this actor read (or, with write, reply/react on) a post in postCohort?
// Learners: their own cohort only. Staff: the cohort must be in scope; writing also needs the
// care capability, so a read-only Viewer can look but not take part.
func boardPostAccess(a boardActor, postCohort int, write bool) bool {
	if a.role == "" {
		return a.ownCohort > 0 && postCohort == a.ownCohort
	}
	if !staff.InScope(a.role, a.cohorts, postCohort) {
		return false
	}
	return !write || staff.Can(a.role, staff.CapCare)
}

// boardName is how a person shows on the board: their Zoom name, else first name (staff often have no Zoom name).
func boardName(u *domain.User) string {
	if u.ZoomName != "" {
		return u.ZoomName
	}
	return u.FirstName
}

// boardUsers is the one thing the board needs from the user service.
type boardUsers interface {
	GetUserByID(id string) (*domain.User, error)
}

// postFor loads the post named by :postId and checks the caller may read it (or write to it).
// When it returns ok=false it has already sent the error response; the handler just returns nil.
func (h *TalkBoardHandler) postFor(c *fiber.Ctx, write bool) (*domain.Post, boardActor, bool) {
	oid, err := primitive.ObjectIDFromHex(c.Params("postId"))
	if err != nil {
		_ = utils.SendError(c, fiber.StatusBadRequest, "Invalid post ID")
		return nil, boardActor{}, false
	}
	a, ok := h.actor(c)
	if !ok {
		_ = utils.SendError(c, fiber.StatusUnauthorized, "User data not found")
		return nil, a, false
	}
	post, err := h.repo.FindByID(c.Context(), oid)
	if err != nil || post == nil {
		_ = utils.SendError(c, fiber.StatusNotFound, "Post not found")
		return nil, a, false
	}
	if !boardPostAccess(a, post.Cohort, false) {
		_ = utils.SendError(c, fiber.StatusForbidden, "You don't have access to this cohort's board")
		return nil, a, false
	}
	if write && !boardPostAccess(a, post.Cohort, true) {
		_ = utils.SendError(c, fiber.StatusForbidden, "Your role can read this board but not post or react")
		return nil, a, false
	}
	return post, a, true
}
