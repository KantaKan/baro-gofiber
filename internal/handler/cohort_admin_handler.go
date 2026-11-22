package handler

import (
	"context"
	"errors"
	"regexp"
	"time"

	"gofiber-baro/internal/domain"
	"gofiber-baro/internal/service/cohort"
	"gofiber-baro/pkg/utils"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type cohortCreator interface {
	Create(cohort int, startDate string, courseDays int) error
}

type groupStore interface {
	GetUserByID(id string) (*domain.User, error)
	SetGenmateGroup(ctx context.Context, id primitive.ObjectID, group string) error
}

// CohortAdminHandler creates cohorts and moves learners between GenMate groups.
type CohortAdminHandler struct {
	cohorts cohortCreator
	users   groupStore
}

func NewCohortAdminHandler(c cohortCreator, u groupStore) *CohortAdminHandler {
	return &CohortAdminHandler{cohorts: c, users: u}
}

var groupLetter = regexp.MustCompile(`^[A-Z]?$`)

// MoveGroups applies all moves, or none if any is invalid.
// @Summary  Move learners between GenMate groups  @Tags admin  @Security BearerAuth  @Router /admin/cohorts/{cohort}/groups [put]
func (h *CohortAdminHandler) MoveGroups(c *fiber.Ctx) error {
	n, err := c.ParamsInt("cohort")
	if err != nil || n <= 0 {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid cohort")
	}
	var b struct {
		Moves []struct {
			UserID string `json:"user_id"`
			Group  string `json:"group"`
		} `json:"moves"`
	}
	if err := c.BodyParser(&b); err != nil || len(b.Moves) == 0 || len(b.Moves) > 500 {
		return utils.SendError(c, fiber.StatusBadRequest, "moves are required")
	}
	ids := make([]primitive.ObjectID, len(b.Moves))
	for i, m := range b.Moves {
		id, e := primitive.ObjectIDFromHex(m.UserID)
		if e != nil || !groupLetter.MatchString(m.Group) {
			return utils.SendError(c, fiber.StatusBadRequest, "Each move needs a learner and a group letter A–Z")
		}
		u, e := h.users.GetUserByID(m.UserID)
		if e != nil || u == nil || u.CohortNumber != n || u.Role != "learner" {
			return utils.SendError(c, fiber.StatusBadRequest, "Every learner must be in this cohort")
		}
		ids[i] = id
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	for i, m := range b.Moves {
		if e := h.users.SetGenmateGroup(ctx, ids[i], m.Group); e != nil {
			return utils.SendError(c, fiber.StatusInternalServerError, "Error saving groups")
		}
	}
	return utils.SendResponse(c, fiber.StatusOK, "Groups updated", fiber.Map{"moved": len(b.Moves)})
}

// Create starts a new cohort (default check-in rules).
// @Summary  Create a cohort  @Tags admin  @Security BearerAuth  @Router /admin/cohorts [post]
func (h *CohortAdminHandler) Create(c *fiber.Ctx) error {
	var b struct {
		CohortNumber int    `json:"cohort_number"`
		StartDate    string `json:"start_date"`
		CourseDays   int    `json:"course_days"`
	}
	if err := c.BodyParser(&b); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}
	if err := h.cohorts.Create(b.CohortNumber, b.StartDate, b.CourseDays); err != nil {
		if errors.Is(err, cohort.ErrCohortExists) {
			return utils.SendError(c, fiber.StatusConflict, err.Error())
		}
		return utils.SendError(c, fiber.StatusBadRequest, err.Error())
	}
	return utils.SendResponse(c, fiber.StatusCreated, "Cohort created", fiber.Map{"cohort_number": b.CohortNumber})
}
