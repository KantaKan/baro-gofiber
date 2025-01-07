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

func GetAllReflectionsController(c *fiber.Ctx) error {
    page := c.QueryInt("page", 1)
    if page < 1 {
        page = 1
    }

    limit := c.QueryInt("limit", 10)
    if limit < 1 {
        limit = 10
    }

    reflections, total, err := services.GetAllReflectionsWithUserInfo(page, limit)
    if err != nil {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
            "success": false,
            "message": "Failed to fetch reflections",
            "error":   err.Error(),
        })
    }

    return c.JSON(fiber.Map{
        "success": true,
        "data":    reflections,
        "total":   total,
    })
}