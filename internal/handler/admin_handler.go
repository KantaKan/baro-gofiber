package handler

import (
	"gofiber-baro/internal/service/reflection"
	"gofiber-baro/internal/service/user"
	"gofiber-baro/pkg/utils"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type AdminHandler struct {
	userService       *user.Service
	badgeService      *user.BadgeService
	reflectionService *reflection.Service
	barometerService  *reflection.BarometerService
}

func NewAdminHandler(
	userService *user.Service,
	badgeService *user.BadgeService,
	reflectionService *reflection.Service,
	barometerService *reflection.BarometerService,
) *AdminHandler {
	return &AdminHandler{
		userService:       userService,
		badgeService:      badgeService,
		reflectionService: reflectionService,
		barometerService:  barometerService,
	}
}

func (h *AdminHandler) GetAllUsers(c *fiber.Ctx) error {
	cohort := c.QueryInt("cohort", 0)
	role := c.Query("role", "")
	email := c.Query("email", "")
	search := c.Query("search", "")
	sort := c.Query("sort", "first_name")
	sortDir := c.QueryInt("sortDir", 1)
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 50)

	users, total, err := h.userService.GetAllUsers(cohort, role, email, search, sort, sortDir, page, limit)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error fetching users")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Users retrieved", fiber.Map{
		"users": users,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

func (h *AdminHandler) GetUserByID(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return utils.SendError(c, fiber.StatusBadRequest, "User ID is required")
	}

	user, err := h.userService.GetUserByID(id)
	if err != nil {
		return utils.SendError(c, fiber.StatusNotFound, "User not found")
	}

	return utils.SendResponse(c, fiber.StatusOK, "User retrieved", user)
}

func (h *AdminHandler) GetUserWithReflections(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return utils.SendError(c, fiber.StatusBadRequest, "User ID is required")
	}

	user, err := h.userService.GetUserByID(id)
	if err != nil {
		return utils.SendError(c, fiber.StatusNotFound, "User not found")
	}

	reflections := user.Reflections

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "User data retrieved",
		"data": fiber.Map{
			"user":        user,
			"reflections": reflections,
		},
	})
}

func (h *AdminHandler) AwardBadge(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return utils.SendError(c, fiber.StatusBadRequest, "User ID is required")
	}

	type RequestBody struct {
		Type     string `json:"type"`
		Name     string `json:"name"`
		Emoji    string `json:"emoji"`
		ImageUrl string `json:"imageUrl"`
		Color    string `json:"color"`
		Style    string `json:"style"`
	}

	var body RequestBody
	if err := c.BodyParser(&body); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if body.Name == "" {
		return utils.SendError(c, fiber.StatusBadRequest, "Badge name is required")
	}

	userID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid user ID")
	}

	if err := h.badgeService.AwardBadge(userID, body.Type, body.Name, body.Emoji, body.ImageUrl, body.Color, body.Style); err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error awarding badge")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Badge awarded successfully", nil)
}

func (h *AdminHandler) GetAllReflections(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 20)

	reflections, total, err := h.reflectionService.GetAllReflectionsWithUserInfo(page, limit)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error fetching reflections")
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Reflections retrieved",
		"data":    reflections,
		"total":   total,
	})
}

func (h *AdminHandler) GetAllReflectionsWithUserInfo(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 20)

	reflections, total, err := h.reflectionService.GetAllReflectionsWithUserInfo(page, limit)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error fetching reflections")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Reflections retrieved", fiber.Map{
		"reflections": reflections,
		"total":       total,
		"page":        page,
		"limit":       limit,
	})
}

func (h *AdminHandler) GetUserBarometerData(c *fiber.Ctx) error {
	users, _, err := h.userService.GetAllUsers(0, "", "", "", "", 0, 1, 500)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error fetching users")
	}

	data, err := h.barometerService.GetUserBarometerData(users)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error fetching barometer data")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Barometer data retrieved", data)
}

func (h *AdminHandler) GetAllUsersBarometerData(c *fiber.Ctx) error {
	timeRange := c.Query("timeRange", "90d")
	cohort := c.QueryInt("cohort", 0)

	data, err := h.barometerService.GetAllUsersBarometerData(timeRange, cohort)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error fetching barometer data")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Barometer data retrieved", data)
}

func (h *AdminHandler) GetEmojiZoneTableData(c *fiber.Ctx) error {
	users, _, err := h.userService.GetAllUsers(0, "", "", "", "", 0, 1, 500)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error fetching users")
	}

	data, err := h.reflectionService.GetEmojiZoneTableData(users)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error fetching emoji zone data")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Emoji zone data retrieved", data)
}

func (h *AdminHandler) GetWeeklySummary(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 10)
	cohort := c.QueryInt("cohort", 0)

	summaries, total, err := h.barometerService.GetWeeklySummary(page, limit, cohort)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error fetching weekly summary")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Weekly summary retrieved", fiber.Map{
		"summaries": summaries,
		"total":     total,
		"page":      page,
		"limit":     limit,
	})
}

func (h *AdminHandler) UpdateReflectionFeedback(c *fiber.Ctx) error {
	type RequestBody struct {
		UserID       string `json:"user_id"`
		ReflectionID string `json:"reflection_id"`
		Feedback     string `json:"feedback"`
	}

	var body RequestBody
	if err := c.BodyParser(&body); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if body.UserID == "" || body.ReflectionID == "" {
		return utils.SendError(c, fiber.StatusBadRequest, "User ID and reflection ID are required")
	}

	userID, err := primitive.ObjectIDFromHex(body.UserID)
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid user ID")
	}

	reflectionID, err := primitive.ObjectIDFromHex(body.ReflectionID)
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid reflection ID")
	}

	if err := h.userService.UpdateReflectionFeedback(userID, reflectionID, body.Feedback); err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error updating feedback")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Feedback updated successfully", nil)
}
