package controllers

import (
	"gofiber-baro/services"
	"gofiber-baro/utils"

	"github.com/gofiber/fiber/v2"
)



// GetSpreadsheetData retrieves data formatted for spreadsheet export
// @Summary Get spreadsheet data
// @Description Get all user and reflection data formatted for spreadsheet export (Admin only)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Success 200 {array} SpreadsheetData "Spreadsheet data retrieved"
// @Failure 500 {object} utils.StandardResponse "Error retrieving data"
// @Router /api/spreadsheet-data [get]
func GetSpreadsheetData(c *fiber.Ctx) error {
	data, err := services.GetSpreadsheetData()
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error retrieving spreadsheet data")
	}
	return c.JSON(data)
}