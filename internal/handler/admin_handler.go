package handler

import (
	"gofiber-baro/internal/service/reflection"
	"gofiber-baro/internal/service/user"
	"gofiber-baro/pkg/middleware"
	"gofiber-baro/pkg/utils"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type AdminHandler struct {
	userService       *user.Service
	badgeService      *user.BadgeService
	fertilizerService *user.FertilizerService
	reflectionService *reflection.Service
	barometerService  *reflection.BarometerService
}

func NewAdminHandler(
	userService *user.Service,
	badgeService *user.BadgeService,
	fertilizerService *user.FertilizerService,
	reflectionService *reflection.Service,
	barometerService *reflection.BarometerService,
) *AdminHandler {
	return &AdminHandler{
		userService:       userService,
		badgeService:      badgeService,
		fertilizerService: fertilizerService,
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

func (h *AdminHandler) BulkAwardBadge(c *fiber.Ctx) error {
	type RequestBody struct {
		UserIDs []string `json:"userIds"`
		Type    string   `json:"type"`
		Name    string   `json:"name"`
		Emoji    string   `json:"emoji"`
		ImageUrl string   `json:"imageUrl"`
		Color    string   `json:"color"`
		Style    string   `json:"style"`
	}

	var body RequestBody
	if err := c.BodyParser(&body); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if len(body.UserIDs) == 0 {
		return utils.SendError(c, fiber.StatusBadRequest, "User IDs are required")
	}

	if body.Name == "" {
		return utils.SendError(c, fiber.StatusBadRequest, "Badge name is required")
	}

	successCount := 0
	failCount := 0

	for _, id := range body.UserIDs {
		userID, err := primitive.ObjectIDFromHex(id)
		if err != nil {
			failCount++
			continue
		}

		if err := h.badgeService.AwardBadge(userID, body.Type, body.Name, body.Emoji, body.ImageUrl, body.Color, body.Style); err != nil {
			failCount++
			continue
		}
		successCount++
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success":      true,
		"message":      "Bulk badge award completed",
		"data":         nil,
		"successCount": successCount,
		"failCount":    failCount,
	})
}

func (h *AdminHandler) GrantFertilizer(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return utils.SendError(c, fiber.StatusBadRequest, "User ID is required")
	}

	type RequestBody struct {
		Amount int    `json:"amount"`
		Note   string `json:"note"`
	}

	var body RequestBody
	if err := c.BodyParser(&body); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if body.Amount <= 0 {
		body.Amount = 1
	}

	userID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid user ID")
	}

	claims, _ := c.Locals("user").(*middleware.Claims)
	grantedBy := ""
	if claims != nil {
		grantedBy = claims.UserID
	}

	if err := h.fertilizerService.Grant(userID, body.Amount, body.Note, grantedBy); err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error granting fertilizer")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Fertilizer granted successfully", nil)
}

func (h *AdminHandler) BulkGrantFertilizer(c *fiber.Ctx) error {
	type RequestBody struct {
		UserIDs []string `json:"userIds"`
		Amount  int      `json:"amount"`
		Note    string   `json:"note"`
	}

	var body RequestBody
	if err := c.BodyParser(&body); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if len(body.UserIDs) == 0 {
		return utils.SendError(c, fiber.StatusBadRequest, "User IDs are required")
	}

	if body.Amount <= 0 {
		body.Amount = 1
	}

	claims, _ := c.Locals("user").(*middleware.Claims)
	grantedBy := ""
	if claims != nil {
		grantedBy = claims.UserID
	}

	successCount := 0
	failCount := 0

	for _, id := range body.UserIDs {
		userID, err := primitive.ObjectIDFromHex(id)
		if err != nil {
			failCount++
			continue
		}

		if err := h.fertilizerService.Grant(userID, body.Amount, body.Note, grantedBy); err != nil {
			failCount++
			continue
		}
		successCount++
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success":      true,
		"message":      "Bulk fertilizer grant completed",
		"data":         nil,
		"successCount": successCount,
		"failCount":    failCount,
	})
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

// ponytail: no CSV upload, no individual validation per field beyond required
func (h *AdminHandler) BulkRegisterUsers(c *fiber.Ctx) error {
	type UserEntry struct {
		FirstName    string `json:"first_name"`
		LastName     string `json:"last_name"`
		Email        string `json:"email"`
		JSDNumber    string `json:"jsd_number"`
		Password     string `json:"password"`
		ProjectGroup string `json:"project_group"`
		GenmateGroup string `json:"genmate_group"`
		ZoomName     string `json:"zoom_name"`
	}
	type RequestBody struct {
		CohortNumber int         `json:"cohort_number"`
		Password     string      `json:"password"`
		Users        []UserEntry `json:"users"`
	}

	var body RequestBody
	if err := c.BodyParser(&body); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if body.CohortNumber == 0 {
		return utils.SendError(c, fiber.StatusBadRequest, "cohort_number is required")
	}
	if len(body.Users) == 0 {
		return utils.SendError(c, fiber.StatusBadRequest, "at least one user is required")
	}
	if len(body.Users) > 200 {
		return utils.SendError(c, fiber.StatusBadRequest, "maximum 200 users per request")
	}

	inputs := make([]user.BulkUserInput, len(body.Users))
	for i, u := range body.Users {
		inputs[i] = user.BulkUserInput{
			FirstName:    u.FirstName,
			LastName:     u.LastName,
			Email:        u.Email,
			JSDNumber:    u.JSDNumber,
			Password:     u.Password,
			ProjectGroup: u.ProjectGroup,
			GenmateGroup: u.GenmateGroup,
			ZoomName:     u.ZoomName,
		}
	}

	results, err := h.userService.BulkCreateUsers(inputs, body.CohortNumber, body.Password)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Bulk registration failed")
	}

	successCount := 0
	failCount := 0
	for _, r := range results {
		if r.Status == "created" {
			successCount++
		} else {
			failCount++
		}
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success":      true,
		"message":      "Bulk registration completed",
		"data":         results,
		"successCount": successCount,
		"failCount":    failCount,
	})
}

func (h *AdminHandler) DeleteUser(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return utils.SendError(c, fiber.StatusBadRequest, "User ID is required")
	}

	if err := h.userService.SoftDeleteUser(id); err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error deleting user")
	}

	return utils.SendResponse(c, fiber.StatusOK, "User deleted successfully", nil)
}

// UpdatePlantOverride lets an admin override a learner's full plant look —
// palette plus species/pot/leaf/flower/stem — or clear any of them back to
// the hash-derived default by sending an empty string.
// PATCH /admin/users/:id/plant
func (h *AdminHandler) UpdatePlantOverride(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return utils.SendError(c, fiber.StatusBadRequest, "User ID is required")
	}

	type RequestBody struct {
		Palette string `json:"palette"`
		Species string `json:"species"`
		Pot     string `json:"pot"`
		Leaf    string `json:"leaf"`
		Flower  string `json:"flower"`
		Stem    string `json:"stem"`
	}

	var body RequestBody
	if err := c.BodyParser(&body); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if body.Palette != "" && !validPlantPalettes[body.Palette] {
		return utils.SendError(c, fiber.StatusBadRequest, "Unknown palette")
	}
	if body.Species != "" && !validPlantSpecies[body.Species] {
		return utils.SendError(c, fiber.StatusBadRequest, "Unknown species")
	}
	if body.Pot != "" && !validPlantPots[body.Pot] {
		return utils.SendError(c, fiber.StatusBadRequest, "Unknown pot style")
	}
	if body.Leaf != "" && !validPlantLeaves[body.Leaf] {
		return utils.SendError(c, fiber.StatusBadRequest, "Unknown leaf style")
	}
	if body.Flower != "" && !validPlantFlowers[body.Flower] {
		return utils.SendError(c, fiber.StatusBadRequest, "Unknown flower type")
	}
	if body.Stem != "" && !validPlantStems[body.Stem] {
		return utils.SendError(c, fiber.StatusBadRequest, "Unknown stem style")
	}

	update := map[string]interface{}{
		"selected_palette": body.Palette,
		"selected_species": body.Species,
		"selected_pot":     body.Pot,
		"selected_leaf":    body.Leaf,
		"selected_flower":  body.Flower,
		"selected_stem":    body.Stem,
	}

	if err := h.userService.UpdateUser(id, update); err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error updating plant override")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Plant updated", nil)
}
