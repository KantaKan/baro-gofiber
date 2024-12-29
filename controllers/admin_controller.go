package controllers

import (
	"gofiber-baro/services"
	"gofiber-baro/utils"

	"github.com/gofiber/fiber/v2"
)

// GetReflectionsByWeek retrieves reflections by week and sorts by Panic Zone first

// GetAllUsers retrieves all users
func GetAllUsers(c *fiber.Ctx) error {
	// Call service to fetch all users
	users, err := services.GetAllUsers()
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error retrieving users")
	}

	// Send successful response with all users
	return utils.SendResponse(c, fiber.StatusOK, "All users retrieved", users)
}

func GetUserBarometerDataController(c *fiber.Ctx) error {
	// Call service to get the barometer data (zone counts)
	zoneCounts, err := services.GetUserBarometerData()
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error fetching barometer data")
	}

	// Send successful response with the zone counts
	return utils.SendResponse(c, fiber.StatusOK, "Barometer zone counts for the day", zoneCounts)
}
