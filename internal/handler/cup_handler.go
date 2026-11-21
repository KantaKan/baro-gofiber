package handler

import (
	"errors"

	"gofiber-baro/internal/domain"
	"gofiber-baro/internal/service/cup"
	"gofiber-baro/pkg/utils"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/mongo"
)

type cupUsers interface {
	GetUserByID(id string) (*domain.User, error)
}

// CupHandler serves the weekly GenMate Cup.
type CupHandler struct {
	svc   *cup.Service
	users cupUsers
}

func NewCupHandler(svc *cup.Service, users cupUsers) *CupHandler {
	return &CupHandler{svc: svc, users: users}
}

// Me returns this week's standings, my group, champion group, my medals and the wall of fame.
// @Summary      My GenMate Cup
// @Tags         cup
// @Produce      json
// @Success      200  {object}  cup.MyCup
// @Security     BearerAuth
// @Router       /cup/me [get]
func (h *CupHandler) Me(c *fiber.Ctx) error {
	id, _ := c.Locals("userID").(string)
	if id == "" {
		return utils.SendError(c, fiber.StatusUnauthorized, "Unauthorized")
	}
	u, err := h.users.GetUserByID(id)
	if err != nil || u == nil {
		return utils.SendError(c, fiber.StatusNotFound, "User not found")
	}
	out, err := h.svc.Me(*u, utils.GetThailandTime())
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error loading the Cup")
	}
	return utils.SendResponse(c, fiber.StatusOK, "Cup retrieved", out)
}

type awardBody struct {
	Cohort   int    `json:"cohort"`
	Group    string `json:"group"`
	Category string `json:"category"` // coding | soft | helping
	Points   int    `json:"points"`
	Reason   string `json:"reason"`
}

// Award gives a group points for this week.
// @Summary      Award GenMate Cup points
// @Tags         admin
// @Accept       json
// @Produce      json
// @Param        body  body  awardBody  true  "category: coding, soft or helping; points 1-100; reason up to 140 chars"
// @Success      201  {object}  domain.CupAward
// @Failure      400  {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/cup/awards [post]
func (h *CupHandler) Award(c *fiber.Ctx) error {
	var b awardBody
	if err := c.BodyParser(&b); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}
	admin, _ := c.Locals("userID").(string)
	a, err := h.svc.Award(admin, b.Cohort, b.Group, b.Category, b.Points, b.Reason, utils.GetThailandTime())
	if err != nil {
		if errors.Is(err, cup.ErrInvalidAward) {
			return utils.SendError(c, fiber.StatusBadRequest, err.Error())
		}
		return utils.SendError(c, fiber.StatusInternalServerError, "Error saving the award")
	}
	return utils.SendResponse(c, fiber.StatusCreated, "Points awarded", a)
}

// DeleteAward undoes a TA award (server "showing up" bonuses can't be undone here).
// @Summary      Undo a GenMate Cup award
// @Tags         admin
// @Param        id  path  string  true  "Award ID"
// @Success      200  {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/cup/awards/{id} [delete]
func (h *CupHandler) DeleteAward(c *fiber.Ctx) error {
	err := h.svc.Delete(c.Params("id"))
	switch {
	case errors.Is(err, cup.ErrInvalidAward):
		return utils.SendError(c, fiber.StatusBadRequest, err.Error())
	case errors.Is(err, mongo.ErrNoDocuments):
		return utils.SendError(c, fiber.StatusNotFound, "Award not found")
	case err != nil:
		return utils.SendError(c, fiber.StatusInternalServerError, "Error deleting the award")
	}
	return utils.SendResponse(c, fiber.StatusOK, "Award removed", nil)
}

// Standings returns a cohort's week (default: this week) with every award.
// @Summary      GenMate Cup standings
// @Tags         admin
// @Produce      json
// @Param        cohort  query  int     true   "Cohort number"
// @Param        week    query  string  false  "Monday YYYY-MM-DD"
// @Success      200  {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /admin/cup/standings [get]
func (h *CupHandler) Standings(c *fiber.Ctx) error {
	cohort := c.QueryInt("cohort")
	if cohort <= 0 {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid cohort")
	}
	week := c.Query("week", cup.CurrentWeek(utils.GetThailandTime()))
	s, awards, err := h.svc.Standings(cohort, week)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error loading standings")
	}
	return utils.SendResponse(c, fiber.StatusOK, "Standings retrieved", fiber.Map{"week": week, "standings": s, "awards": awards})
}
