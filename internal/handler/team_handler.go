package handler

import (
	"errors"
	"time"

	"gofiber-baro/internal/domain"
	"gofiber-baro/internal/service/staff"
	"gofiber-baro/pkg/utils"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type teamService interface {
	Members() ([]domain.User, error)
	Invites() ([]staff.Invite, error)
	Invite(actor primitive.ObjectID, email, role string, cohorts []int, groups []string) (string, *staff.Invite, error)
	Renew(id primitive.ObjectID) (string, *staff.Invite, error)
	CancelInvite(id primitive.ObjectID) error
	Lookup(token string) (*staff.Invite, bool, error)
	Accept(token, first, last, password string) error
	Update(actor, id primitive.ObjectID, role string, cohorts []int, groups []string) error
	Remove(actor, id primitive.ObjectID) error
}

// TeamHandler: Team & roles (Owner only, via StaffGuard) and the public invite pages.
type TeamHandler struct{ team teamService }

func NewTeamHandler(t teamService) *TeamHandler { return &TeamHandler{team: t} }

func teamErr(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, staff.ErrInviteInvalid), errors.Is(err, staff.ErrNotMember):
		return utils.SendError(c, fiber.StatusNotFound, err.Error())
	case errors.Is(err, staff.ErrWrongPassword):
		return utils.SendError(c, fiber.StatusUnauthorized, err.Error())
	case errors.Is(err, staff.ErrNotOwner):
		return utils.SendError(c, fiber.StatusForbidden, err.Error())
	case errors.Is(err, staff.ErrSelf), errors.Is(err, staff.ErrBadRole), errors.Is(err, staff.ErrBadEmail),
		errors.Is(err, staff.ErrWeakPassword), errors.Is(err, staff.ErrAlreadyStaff):
		return utils.SendError(c, fiber.StatusBadRequest, err.Error())
	}
	return utils.SendError(c, fiber.StatusBadRequest, err.Error())
}

func actorID(c *fiber.Ctx) (primitive.ObjectID, bool) {
	s, ok := StaffFrom(c)
	if !ok {
		return primitive.NilObjectID, false
	}
	id, err := primitive.ObjectIDFromHex(s.UserID)
	return id, err == nil
}

type memberJSON struct {
	ID        string    `json:"id"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	Email     string    `json:"email"`
	StaffRole string    `json:"staff_role"`
	Cohorts   []int     `json:"cohorts"`
	Groups    []string  `json:"groups"`
	CreatedAt time.Time `json:"created_at"`
}

// List: members and pending invites.
// @Summary  Team members and invites  @Tags admin  @Security BearerAuth  @Router /admin/team [get]
func (h *TeamHandler) List(c *fiber.Ctx) error {
	ms, err := h.team.Members()
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Couldn't load the team")
	}
	invs, err := h.team.Invites()
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Couldn't load invites")
	}
	members := make([]memberJSON, 0, len(ms))
	for _, u := range ms {
		cohorts, groups := u.StaffCohorts, u.StaffGroups
		if cohorts == nil {
			cohorts = []int{}
		}
		if groups == nil {
			groups = []string{}
		}
		members = append(members, memberJSON{u.ID.Hex(), u.FirstName, u.LastName, u.Email, staff.Effective(u.Role, u.StaffRole), cohorts, groups, u.ID.Timestamp()})
	}
	if invs == nil {
		invs = []staff.Invite{}
	}
	return utils.SendResponse(c, fiber.StatusOK, "ok", fiber.Map{"members": members, "invites": invs})
}

type inviteBody struct {
	Email   string   `json:"email"`
	Role    string   `json:"role"`
	Cohorts []int    `json:"cohorts"`
	Groups  []string `json:"groups"`
}

// Invite creates an invite link (returned once).
// @Summary  Invite a team member  @Tags admin  @Security BearerAuth  @Router /admin/team/invites [post]
func (h *TeamHandler) Invite(c *fiber.Ctx) error {
	actor, ok := actorID(c)
	if !ok {
		return utils.SendError(c, fiber.StatusForbidden, "Not staff")
	}
	var b inviteBody
	if err := c.BodyParser(&b); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}
	tok, inv, err := h.team.Invite(actor, b.Email, b.Role, b.Cohorts, b.Groups)
	if err != nil {
		return teamErr(c, err)
	}
	return utils.SendResponse(c, fiber.StatusCreated, "Invite created", fiber.Map{"id": inv.ID.Hex(), "token": tok, "expires_at": inv.ExpiresAt})
}

func pathID(c *fiber.Ctx) (primitive.ObjectID, bool) {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	return id, err == nil
}

// Renew re-issues an invite link.
// @Summary  New invite link  @Tags admin  @Security BearerAuth  @Router /admin/team/invites/{id}/renew [post]
func (h *TeamHandler) Renew(c *fiber.Ctx) error {
	id, ok := pathID(c)
	if !ok {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid invite")
	}
	tok, inv, err := h.team.Renew(id)
	if err != nil {
		return teamErr(c, err)
	}
	return utils.SendResponse(c, fiber.StatusOK, "New link", fiber.Map{"token": tok, "expires_at": inv.ExpiresAt})
}

// CancelInvite deletes a pending invite.
// @Summary  Cancel invite  @Tags admin  @Security BearerAuth  @Router /admin/team/invites/{id} [delete]
func (h *TeamHandler) CancelInvite(c *fiber.Ctx) error {
	id, ok := pathID(c)
	if !ok {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid invite")
	}
	if err := h.team.CancelInvite(id); err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Couldn't cancel")
	}
	return utils.SendResponse(c, fiber.StatusOK, "Invite cancelled", nil)
}

// Update changes a member's role and scope.
// @Summary  Change role  @Tags admin  @Security BearerAuth  @Router /admin/team/{id} [patch]
func (h *TeamHandler) Update(c *fiber.Ctx) error {
	actor, ok := actorID(c)
	id, ok2 := pathID(c)
	if !ok || !ok2 {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid member")
	}
	var b inviteBody
	if err := c.BodyParser(&b); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}
	if err := h.team.Update(actor, id, b.Role, b.Cohorts, b.Groups); err != nil {
		return teamErr(c, err)
	}
	return utils.SendResponse(c, fiber.StatusOK, "Role updated", nil)
}

// Remove ends a member's access.
// @Summary  Remove member  @Tags admin  @Security BearerAuth  @Router /admin/team/{id} [delete]
func (h *TeamHandler) Remove(c *fiber.Ctx) error {
	actor, ok := actorID(c)
	id, ok2 := pathID(c)
	if !ok || !ok2 {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid member")
	}
	if err := h.team.Remove(actor, id); err != nil {
		return teamErr(c, err)
	}
	return utils.SendResponse(c, fiber.StatusOK, "Removed", nil)
}

// Lookup (public): what an invite link is for.
// @Summary  Read an invite  @Tags auth  @Router /invites/{token} [get]
func (h *TeamHandler) Lookup(c *fiber.Ctx) error {
	inv, existing, err := h.team.Lookup(c.Params("token"))
	if err != nil {
		return teamErr(c, err)
	}
	return utils.SendResponse(c, fiber.StatusOK, "ok", fiber.Map{"email": inv.Email, "role": inv.Role, "cohorts": inv.Cohorts, "existing": existing, "expires_at": inv.ExpiresAt})
}

// Accept (public): create or upgrade the account.
// @Summary  Accept an invite  @Tags auth  @Router /invites/{token}/accept [post]
func (h *TeamHandler) Accept(c *fiber.Ctx) error {
	var b struct {
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Password  string `json:"password"`
	}
	if err := c.BodyParser(&b); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}
	if err := h.team.Accept(c.Params("token"), b.FirstName, b.LastName, b.Password); err != nil {
		return teamErr(c, err)
	}
	return utils.SendResponse(c, fiber.StatusOK, "Welcome to the team", nil)
}
