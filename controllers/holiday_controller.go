package controllers

import (
	"gofiber-baro/models"
	"gofiber-baro/services"
	"gofiber-baro/utils"

	"github.com/gofiber/fiber/v2"
)

type CreateHolidayRequest struct {
	Name        string `json:"name"`
	StartDate   string `json:"start_date"`
	EndDate     string `json:"end_date"`
	Description string `json:"description"`
}

func CreateHoliday(c *fiber.Ctx) error {
	var req CreateHolidayRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if req.Name == "" || req.StartDate == "" || req.EndDate == "" {
		return utils.SendError(c, fiber.StatusBadRequest, "Name, start_date, and end_date are required")
	}

	userID, ok := c.Locals("userID").(string)
	if !ok || userID == "" {
		return utils.SendError(c, fiber.StatusUnauthorized, "User ID not found in context or invalid")
	}

	holiday, err := services.CreateHoliday(req.Name, req.StartDate, req.EndDate, req.Description, userID)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Failed to create holiday")
	}

	return utils.SendResponse(c, fiber.StatusCreated, "Holiday created successfully", holiday)
}

func GetHolidays(c *fiber.Ctx) error {
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	var holidays []models.Holiday
	var err error

	if startDate != "" && endDate != "" {
		holidays, err = services.GetHolidaysInRange(startDate, endDate)
	} else {
		holidays, err = services.GetHolidays()
	}

	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Failed to fetch holidays")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Holidays fetched successfully", holidays)
}

func DeleteHoliday(c *fiber.Ctx) error {
	holidayID := c.Params("id")
	if holidayID == "" {
		return utils.SendError(c, fiber.StatusBadRequest, "Holiday ID is required")
	}

	err := services.DeleteHoliday(holidayID)
	if err != nil {
		if err == services.ErrHolidayNotFound {
			return utils.SendError(c, fiber.StatusNotFound, "Holiday not found")
		}
		return utils.SendError(c, fiber.StatusInternalServerError, "Failed to delete holiday")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Holiday deleted successfully", nil)
}
