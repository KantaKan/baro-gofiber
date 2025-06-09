package controllers

import (
	"gofiber-baro/services"
	"gofiber-baro/utils"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// GetReflectionsByWeek retrieves reflections by week and sorts by Panic Zone first

// GetAllUsers retrieves all users
// @Summary Get all users
// @Description Get a list of all users (Admin only)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Success 200 {object} utils.StandardResponse{data=[]models.User} "Users retrieved"
// @Failure 403 {object} utils.StandardResponse "Access denied"
// @Router /admin/users [get]
func GetAllUsers(c *fiber.Ctx) error {
	cohort := c.QueryInt("cohort", 0)
	role := c.Query("role", "")
	email := c.Query("email", "")
	search := c.Query("search", "")
	sort := c.Query("sort", "") // e.g., "first_name", "email", "created_at"
	sortDir := c.QueryInt("sortDir", 1) // 1 for ascending, -1 for descending
	page := c.QueryInt("page", 1)
	if page < 1 {
		page = 1
	}
	limit := c.QueryInt("limit", 40)
	if limit < 1 {
		limit = 40
	}
	if limit > 100 {
		limit = 100
	}

	users, total, err := services.GetAllUsers(cohort, role, email, search, sort, sortDir, page, limit)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error retrieving users")
	}

	return utils.SendResponse(c, fiber.StatusOK, "All users retrieved", fiber.Map{
		"users": users,
		"total": total,
		"page": page,
		"limit": limit,
	})
}

// GetUserBarometerDataController retrieves barometer statistics
// @Summary Get barometer statistics
// @Description Get statistics about user barometer data (Admin only)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Success 200 {object} utils.StandardResponse "Barometer statistics retrieved"
// @Failure 500 {object} utils.StandardResponse "Error fetching data"
// @Router /admin/barometer [get]
func GetUserBarometerDataController(c *fiber.Ctx) error {
	// Call service to get the barometer data (zone counts)
	zoneCounts, err := services.GetUserBarometerData()
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error fetching barometer data")
	}

	// Send successful response with the zone counts
	return utils.SendResponse(c, fiber.StatusOK, "Barometer zone counts for the day", zoneCounts)
}

// GetAllReflectionsController retrieves all reflections
// @Summary Get all reflections
// @Description Get all user reflections with pagination (Admin only)
// @Tags admin
// @Security BearerAuth
// @Param page query integer false "Page number" default(1)
// @Param limit query integer false "Items per page" default(10)
// @Produce json
// @Success 200 {object} object{success=boolean,data=[]models.ReflectionWithUser,total=integer} "Reflections retrieved"
// @Failure 500 {object} object{success=boolean,message=string,error=string} "Failed to fetch reflections"
// @Router /admin/reflections [get]
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

// GetChartData retrieves chart data
// @Summary Get chart data
// @Description Get data for charts (Admin only)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Success 200 {array} object{day=string,DailyReflection=integer} "Chart data retrieved"
// @Failure 500 {object} object{error=string} "Failed to fetch chart data"
// @Router /admin/chart-data [get]
func GetChartData(c *fiber.Ctx) error {
	chartData, err := services.GetChartData()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch chart data"})
	}

	return c.JSON(chartData)
}

// GetBarometerData retrieves daily barometer data
// @Summary Get daily barometer data
// @Description Get daily barometer statistics (Admin only)
// @Tags admin
// @Security BearerAuth
// @Param timeRange query string false "Time range (90d, 30d, 7d)" default(90d)
// @Produce json
// @Success 200 {array} models.BarometerData "Barometer data retrieved"
// @Failure 400 {object} object{error=string} "Invalid time range"
// @Failure 500 {object} object{error=string} "Failed to fetch data"
// @Router /admin/reflections/chartday [get]
func GetBarometerData(c *fiber.Ctx) error {
	timeRange := c.Query("timeRange", "90d")
	// Validate timeRange
	if timeRange != "90d" && timeRange != "30d" && timeRange != "7d" {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid timeRange. Must be one of: 90d, 30d, 7d",
		})
	}

	chartData, err := services.GetAllUsersBarometerData(timeRange)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to fetch barometer data",
		})
	}

	return c.JSON(chartData)
}

// GetUserWithReflections retrieves user with their reflections
// @Summary Get user with reflections
// @Description Get a specific user with all their reflections (Admin only)
// @Tags admin
// @Security BearerAuth
// @Param id path string true "User ID"
// @Produce json
// @Success 200 {object} utils.StandardResponse{data=services.UserWithReflections} "User and reflections retrieved"
// @Failure 400 {object} utils.StandardResponse "Invalid user ID"
// @Failure 500 {object} utils.StandardResponse "Error retrieving data"
// @Router /admin/userreflections/{id} [get]
func GetUserWithReflections(c *fiber.Ctx) error {
	userID := c.Params("id") // Get user ID from route parameters
	objectID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid user ID")
	}

	// Call service to get user and reflections
	userWithReflections, err := services.GetUserWithReflections(objectID)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error retrieving user and reflections")
	}

	// Send successful response with user and reflections
	return utils.SendResponse(c, fiber.StatusOK, "User and reflections retrieved", userWithReflections)
}