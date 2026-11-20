package handler

import (
	"gofiber-baro/internal/domain"
	"gofiber-baro/internal/service/cohort"
	"gofiber-baro/pkg/utils"

	"github.com/gofiber/fiber/v2"
)

type CohortHandler struct{ svc *cohort.Service }

func NewCohortHandler(svc *cohort.Service) *CohortHandler { return &CohortHandler{svc: svc} }

func (h *CohortHandler) GetSettings(c *fiber.Ctx) error {
	n, err := c.ParamsInt("cohort")
	if err != nil || n <= 0 {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid cohort")
	}
	s, err := h.svc.Get(n)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error fetching cohort settings")
	}
	return utils.SendResponse(c, fiber.StatusOK, "Cohort settings retrieved", s)
}

func (h *CohortHandler) UpdateSettings(c *fiber.Ctx) error {
	n, err := c.ParamsInt("cohort")
	if err != nil || n <= 0 {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid cohort")
	}
	var body domain.CohortSettings
	if err := c.BodyParser(&body); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}
	if err := h.svc.Update(n, body); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, err.Error())
	}
	s, _ := h.svc.Get(n)
	return utils.SendResponse(c, fiber.StatusOK, "Cohort settings updated", s)
}
