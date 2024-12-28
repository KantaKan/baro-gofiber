package controllers

import (
	"gofiber-baro/services"
	"gofiber-baro/utils"

	"github.com/gofiber/fiber/v2"
)

// GetReflectionsByWeek retrieves reflections by week and sorts by Panic Zone first
func GetReflectionsByWeek(c *fiber.Ctx) error {
	// Call service to fetch reflections by week
	weeklyReflections, err := services.GetReflectionsByWeek()
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error retrieving reflections")
	}

	// Send successful response with weekly reflections
	return utils.SendResponse(c, fiber.StatusOK, "Reflections by week retrieved", weeklyReflections)
}

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


func GetPanicUsers(c *fiber.Ctx) error {
	// Call the service layer to fetch users with Panic Zone reflections
	panicUsers, err := services.GetUsersByBarometer("Panic Zone")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Return the list of Panic Zone users
	return c.JSON(panicUsers)
}