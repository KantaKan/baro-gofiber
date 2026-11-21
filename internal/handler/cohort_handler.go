package handler

import (
	"gofiber-baro/internal/domain"
	"gofiber-baro/internal/service/cohort"
	"gofiber-baro/pkg/utils"

	"github.com/gofiber/fiber/v2"
)

type CohortHandler struct{ svc *cohort.Service }

func NewCohortHandler(svc *cohort.Service) *CohortHandler { return &CohortHandler{svc: svc} }

// GetSettings returns a cohort's check-in rules and allowed networks.
// @Summary      Get cohort check-in settings
// @Tags         admin
// @Produce      json
// @Param        cohort  path  int  true  "Cohort number"
// @Success      200  {object}  domain.CohortSettings
// @Security     BearerAuth
// @Router       /admin/cohorts/{cohort}/settings [get]
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

// UpdateSettings replaces a cohort's check-in rules and allowed networks.
// @Summary      Update cohort check-in settings
// @Tags         admin
// @Accept       json
// @Produce      json
// @Param        cohort  path  int                    true  "Cohort number"
// @Param        body    body  domain.CohortSettings  true  "Rules and allowed networks (empty list = any network)"
// @Success      200  {object}  domain.CohortSettings
// @Failure      400  {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/cohorts/{cohort}/settings [put]
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
