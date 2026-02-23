package handler

import (
	"gofiber-baro/internal/service/holiday"
	"gofiber-baro/pkg/utils"

	"github.com/gofiber/fiber/v2"
)

type HolidayHandler struct {
	holidayService *holiday.Service
}

func NewHolidayHandler(holidayService *holiday.Service) *HolidayHandler {
	return &HolidayHandler{holidayService: holidayService}
}

func (h *HolidayHandler) GetHolidays(c *fiber.Ctx) error {
	holidays, err := h.holidayService.GetHolidays()
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error fetching holidays")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Holidays retrieved", holidays)
}

func (h *HolidayHandler) CreateHoliday(c *fiber.Ctx) error {
	type RequestBody struct {
		Name        string `json:"name"`
		StartDate   string `json:"start_date"`
		EndDate     string `json:"end_date"`
		Description string `json:"description"`
	}

	var body RequestBody
	if err := c.BodyParser(&body); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if body.Name == "" || body.StartDate == "" || body.EndDate == "" {
		return utils.SendError(c, fiber.StatusBadRequest, "Name, start date, and end date are required")
	}

	adminID := c.Locals("userID")
	createdBy := ""
	if id, ok := adminID.(string); ok {
		createdBy = id
	}

	holiday, err := h.holidayService.CreateHoliday(body.Name, body.StartDate, body.EndDate, body.Description, createdBy)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error creating holiday")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Holiday created", holiday)
}

func (h *HolidayHandler) DeleteHoliday(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return utils.SendError(c, fiber.StatusBadRequest, "Holiday ID is required")
	}

	if err := h.holidayService.DeleteHoliday(id); err != nil {
		if err == holiday.ErrHolidayNotFound {
			return utils.SendError(c, fiber.StatusNotFound, "Holiday not found")
		}
		return utils.SendError(c, fiber.StatusInternalServerError, "Error deleting holiday")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Holiday deleted", nil)
}
