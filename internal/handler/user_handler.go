package handler

import (
	"errors"
	"html"
	"log"
	"time"
	"gofiber-baro/internal/domain"
	"gofiber-baro/internal/service/user"
	middleware "gofiber-baro/pkg/middleware"
	"gofiber-baro/pkg/utils"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type UserHandler struct {
	userService       *user.Service
	fertilizerService *user.FertilizerService
	db                interface{}
}

// ponytail: mirrors the PALETTES name list in react-genbaro/src/lib/plant-variants.ts — keep in sync
var validPlantPalettes = map[string]bool{
	"Forest": true, "Sunset": true, "Ocean": true, "Desert": true, "Rose": true,
	"Lavender": true, "Sunshine": true, "Mint": true, "Coral": true, "Autumn": true,
}

func NewUserHandler(userService *user.Service, fertilizerService *user.FertilizerService) *UserHandler {
	return &UserHandler{userService: userService, fertilizerService: fertilizerService}
}

func (h *UserHandler) LoginUser(c *fiber.Ctx) error {
	var loginData struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := c.BodyParser(&loginData); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if loginData.Email == "" || loginData.Password == "" {
		return utils.SendError(c, fiber.StatusBadRequest, "Email and password are required")
	}

	token, role, userId, err := h.authenticateUser(loginData.Email, loginData.Password)
	if err != nil {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid credentials")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Login successful", map[string]interface{}{
		"token":  token,
		"role":   role,
		"userId": userId,
	})
}

func (h *UserHandler) VerifyToken(c *fiber.Ctx) error {
	claims, ok := c.Locals("user").(*middleware.Claims)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Token is valid", map[string]string{
		"role":   claims.Role,
		"userId": claims.UserID,
	})
}

func (h *UserHandler) CreateReflection(c *fiber.Ctx) error {
	userID := c.Params("id")
	objectID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid user ID")
	}

	claims, ok := c.Locals("user").(*middleware.Claims)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}

	if claims.Role != "admin" && claims.UserID != userID {
		return utils.SendError(c, fiber.StatusForbidden, "You are not allowed to post reflection for this user")
	}

	var reflection domain.Reflection
	if err := c.BodyParser(&reflection); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid reflection data")
	}

	// Barometer Validation & Sanitization
	validZones := map[string]bool{
		"Comfort Zone":                           true,
		"Stretch Zone - Enjoying the Challenges": true,
		"Stretch Zone - Overwhelmed":             true,
		"Panic Zone":                             true,
		// Also allow lowercase variations as seen in the codebase
		"Stretch zone - Enjoying the challenges": true,
		"Stretch zone - Overwhelmed":             true,
	}

	if !validZones[reflection.ReflectionData.Barometer] {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid Barometer zone selected. Please choose from the provided options.")
	}

	// Sanitize all text inputs to prevent XSS
	reflection.ReflectionData.Barometer = html.EscapeString(reflection.ReflectionData.Barometer)

	for i, s := range reflection.ReflectionData.TechSessions.SessionName {
		reflection.ReflectionData.TechSessions.SessionName[i] = html.EscapeString(s)
	}
	reflection.ReflectionData.TechSessions.Happy = html.EscapeString(reflection.ReflectionData.TechSessions.Happy)
	reflection.ReflectionData.TechSessions.Improve = html.EscapeString(reflection.ReflectionData.TechSessions.Improve)

	for i, s := range reflection.ReflectionData.NonTechSessions.SessionName {
		reflection.ReflectionData.NonTechSessions.SessionName[i] = html.EscapeString(s)
	}
	reflection.ReflectionData.NonTechSessions.Happy = html.EscapeString(reflection.ReflectionData.NonTechSessions.Happy)
	reflection.ReflectionData.NonTechSessions.Improve = html.EscapeString(reflection.ReflectionData.NonTechSessions.Improve)

	reflection.UserID = objectID
	if reflection.Date.IsZero() {
		reflection.Date = utils.GetThailandTime()
	}
	reflection.Day = utils.GetThailandDate()

	createdReflection, err := h.createReflection(objectID, reflection)
	if err != nil {
		if err.Error() == "user has already created a reflection today" {
			return utils.SendError(c, fiber.StatusConflict, "You have already submitted a reflection today. Please try again tomorrow.")
		}
		log.Printf("CreateReflection error for user %s: %v", objectID.Hex(), err)
		return utils.SendError(c, fiber.StatusInternalServerError, "Error creating reflection")
	}

	return utils.SendResponse(c, fiber.StatusCreated, "Reflection successfully created", createdReflection)
}

func (h *UserHandler) GetUserReflections(c *fiber.Ctx) error {
	userID := c.Params("id")
	objectID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid user ID")
	}

	claims, ok := c.Locals("user").(*middleware.Claims)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}

	if claims.Role != "admin" && claims.UserID != userID {
		return utils.SendError(c, fiber.StatusForbidden, "You are not allowed to access this user's reflections")
	}

	reflections, err := h.getReflections(objectID)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error retrieving reflections")
	}

	return utils.SendResponse(c, fiber.StatusOK, "User reflections retrieved", reflections)
}

func (h *UserHandler) GetCohort(c *fiber.Ctx) error {
	cohort := c.Params("cohort")
	if cohort == "" {
		return utils.SendError(c, fiber.StatusBadRequest, "Cohort is required")
	}

	users, _, err := h.userService.GetAllUsers(0, "", "", "", "first_name", 1, 1, 500)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error fetching cohort")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Cohort retrieved", users)
}

func (h *UserHandler) GetUserByID(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return utils.SendError(c, fiber.StatusBadRequest, "User ID is required")
	}

	claims, ok := c.Locals("user").(*middleware.Claims)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}

	user, err := h.userService.GetUserByID(id)
	if err != nil {
		return utils.SendError(c, fiber.StatusNotFound, "User not found")
	}

	isAdmin := claims.Role == "admin"

	// Admin sees everything, user sees their own profile with limited data
	if isAdmin {
		user.Password = ""
		return utils.SendResponse(c, fiber.StatusOK, "User retrieved", user)
	}

	// Non-admin: can only see users from their own cohort
	if user.CohortNumber != claims.Cohort {
		return utils.SendError(c, fiber.StatusForbidden, "You can only view profiles within your own cohort")
	}

	// Return safe version without sensitive data
	return utils.SendResponse(c, fiber.StatusOK, "User retrieved", user.ToSafe())
}

func (h *UserHandler) GetUserProfile(c *fiber.Ctx) error {
	userID := c.Locals("userID")
	if userID == nil {
		return utils.SendError(c, fiber.StatusUnauthorized, "Unauthorized")
	}

	user, err := h.userService.GetUserByID(userID.(string))
	if err != nil {
		return utils.SendError(c, fiber.StatusNotFound, "User not found")
	}

	user.Password = ""

	return utils.SendResponse(c, fiber.StatusOK, "User profile retrieved", user)
}

func (h *UserHandler) UpdateUser(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return utils.SendError(c, fiber.StatusBadRequest, "User ID is required")
	}

	claims, ok := c.Locals("user").(*middleware.Claims)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}

	// Security: ONLY admins can use the general update endpoint
	if claims.Role != "admin" {
		return utils.SendError(c, fiber.StatusForbidden, "Only admins can update general user information. Learners should use the personal-details endpoint.")
	}

	var body map[string]interface{}
	if err := c.BodyParser(&body); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if err := h.userService.UpdateUser(id, body); err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error updating user")
	}

	return utils.SendResponse(c, fiber.StatusOK, "User updated successfully", nil)
}

func (h *UserHandler) UseFertilizerProtect(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return utils.SendError(c, fiber.StatusBadRequest, "User ID is required")
	}

	claims, ok := c.Locals("user").(*middleware.Claims)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}

	if claims.UserID != id {
		return utils.SendError(c, fiber.StatusForbidden, "You can only use your own fertilizer")
	}

	var body struct {
		Date string `json:"date"`
	}
	if err := c.BodyParser(&body); err != nil || body.Date == "" {
		return utils.SendError(c, fiber.StatusBadRequest, "A date is required")
	}

	userID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid user ID")
	}

	if err := h.fertilizerService.ProtectDate(userID, body.Date); err != nil {
		switch {
		case errors.Is(err, user.ErrInvalidProtectDate):
			return utils.SendError(c, fiber.StatusBadRequest, "That date can't be protected")
		case errors.Is(err, domain.ErrDateAlreadyProtected):
			return utils.SendError(c, fiber.StatusConflict, "That date is already protected")
		case errors.Is(err, domain.ErrInsufficientFertilizer):
			return utils.SendError(c, fiber.StatusConflict, "Not enough fertilizer")
		default:
			return utils.SendError(c, fiber.StatusInternalServerError, "Error using fertilizer")
		}
	}

	return utils.SendResponse(c, fiber.StatusOK, "Streak protected", nil)
}

func (h *UserHandler) UseFertilizerFeed(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return utils.SendError(c, fiber.StatusBadRequest, "User ID is required")
	}

	claims, ok := c.Locals("user").(*middleware.Claims)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}

	if claims.UserID != id {
		return utils.SendError(c, fiber.StatusForbidden, "You can only use your own fertilizer")
	}

	userID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid user ID")
	}

	var body struct {
		Quantity int `json:"quantity"`
	}
	_ = c.BodyParser(&body)
	if body.Quantity < 1 {
		body.Quantity = 1
	}

	if err := h.fertilizerService.Feed(userID, body.Quantity); err != nil {
		switch {
		case errors.Is(err, user.ErrInvalidFeedQuantity):
			return utils.SendError(c, fiber.StatusBadRequest, "Quantity must be at least 1")
		case errors.Is(err, domain.ErrInsufficientFertilizer):
			return utils.SendError(c, fiber.StatusConflict, "Not enough fertilizer")
		default:
			return utils.SendError(c, fiber.StatusInternalServerError, "Error using fertilizer")
		}
	}

	return utils.SendResponse(c, fiber.StatusOK, "Plant fed", nil)
}

func (h *UserHandler) UpdatePersonalDetails(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return utils.SendError(c, fiber.StatusBadRequest, "User ID is required")
	}

	claims, ok := c.Locals("user").(*middleware.Claims)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}

	// Security: Only allow user to update their own details
	if claims.UserID != id {
		return utils.SendError(c, fiber.StatusForbidden, "You can only update your own details")
	}

	var body struct {
		Bio             string             `json:"bio"`
		SocialLinks     domain.SocialLinks `json:"social_links"`
		PinnedBadgeIDs  []string           `json:"pinned_badge_ids"`
		SelectedPalette string             `json:"selected_palette"`
	}
	if err := c.BodyParser(&body); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if len(body.Bio) > 1000 {
		return utils.SendError(c, fiber.StatusBadRequest, "Bio must be under 1000 characters")
	}

	// ponytail: trusts the client's own tier check (getPlantVariant/getUnlockedPalettes in
	// plant-variants.ts) — only sanity-checks the name here since this is a cosmetic,
	// owner-only field. Port getEffectivePlantDays/TIER_THRESHOLDS to Go if this ladder
	// ever gates something non-cosmetic.
	if body.SelectedPalette != "" && !validPlantPalettes[body.SelectedPalette] {
		return utils.SendError(c, fiber.StatusBadRequest, "Unknown palette")
	}

	update := bson.M{
		"bio": body.Bio,
		"social_links": body.SocialLinks,
		"selected_palette": body.SelectedPalette,
	}

	pinnedOIDs := []primitive.ObjectID{}
	for _, pid := range body.PinnedBadgeIDs {
		oid, err := primitive.ObjectIDFromHex(pid)
		if err == nil {
			pinnedOIDs = append(pinnedOIDs, oid)
		}
	}
	update["pinned_badge_ids"] = pinnedOIDs

	if err := h.userService.UpdateUser(id, update); err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error updating personal details")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Personal details updated successfully", nil)
}

func (h *UserHandler) AwardBadge(c *fiber.Ctx) error {
	type RequestBody struct {
		UserID   string `json:"user_id"`
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

	if body.UserID == "" || body.Name == "" {
		return utils.SendError(c, fiber.StatusBadRequest, "User ID and badge name are required")
	}

	userID, err := primitive.ObjectIDFromHex(body.UserID)
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid user ID")
	}

	if err := h.userService.AwardBadge(userID, body.Type, body.Name, body.Emoji, body.ImageUrl, body.Color, body.Style); err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error awarding badge")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Badge awarded successfully", nil)
}

func (h *UserHandler) UpdateReflectionFeedback(c *fiber.Ctx) error {
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

func (h *UserHandler) GetGenmateGarden(c *fiber.Ctx) error {
	claims, ok := c.Locals("user").(*middleware.Claims)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}

	if claims.Cohort <= 0 {
		return utils.SendResponse(c, fiber.StatusOK, "Genmate garden retrieved", fiber.Map{
			"users": []interface{}{},
		})
	}

	me, err := h.userService.GetUserByID(claims.UserID)
	if err != nil || me.GenmateGroup == "" {
		return utils.SendResponse(c, fiber.StatusOK, "Genmate garden retrieved", fiber.Map{
			"users": []interface{}{},
		})
	}

	users, _, err := h.userService.GetAllUsers(claims.Cohort, "learner", "", "", "first_name", 1, 1, 1000)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error fetching genmate garden")
	}

	members := make([]fiber.Map, 0)
	for _, u := range users {
		if u.Deleted || u.GenmateGroup != me.GenmateGroup {
			continue
		}

		dates := make([]string, 0, len(u.Reflections))
		for _, r := range u.Reflections {
			day := r.Day
			if day == "" {
				day = r.Date.Format(time.RFC3339)
			}
			dates = append(dates, day)
		}

		protectedDates := make([]string, 0)
		for _, entry := range u.FertilizerLog {
			if entry.Kind == "protect" && entry.RelatedDate != "" {
				protectedDates = append(protectedDates, entry.RelatedDate)
			}
		}

		members = append(members, fiber.Map{
			"_id":              u.ID.Hex(),
			"first_name":       u.FirstName,
			"last_name":        u.LastName,
			"cohort_number":    u.CohortNumber,
			"genmate_group":    u.GenmateGroup,
			"reflection_dates": dates,
			"growth_points":    u.GrowthPoints,
			"protected_dates":  protectedDates,
			"plant_reactions":  u.PlantReactions,
			"selected_palette": u.SelectedPalette,
		})
	}

	return utils.SendResponse(c, fiber.StatusOK, "Genmate garden retrieved", fiber.Map{
		"users": members,
	})
}

func (h *UserHandler) GetAllUsers(c *fiber.Ctx) error {
	claims, ok := c.Locals("user").(*middleware.Claims)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}

	cohort := c.QueryInt("cohort", 0)
	if claims.Role != "admin" {
		// Enforce cohort restriction for learners: ignore any cohort query param and use their own
		cohort = claims.Cohort

		// Defensive: prevent returning all users if cohort is 0 or invalid
		if cohort <= 0 {
			return utils.SendResponse(c, fiber.StatusOK, "Users retrieved", fiber.Map{
				"users": []interface{}{},
				"total": 0,
				"page":  1,
				"limit": 50,
			})
		}
	}

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

	isAdmin := claims.Role == "admin"

	if isAdmin {
		for i := range users {
			users[i].Password = ""
		}
		return utils.SendResponse(c, fiber.StatusOK, "Users retrieved", fiber.Map{
			"users": users,
			"total": total,
			"page":  page,
			"limit": limit,
		})
	}

	safeUsers := make([]domain.UserSafe, len(users))
	for i, user := range users {
		safeUsers[i] = user.ToSafe()
	}

	return utils.SendResponse(c, fiber.StatusOK, "Users retrieved", fiber.Map{
		"users": safeUsers,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

func (h *UserHandler) AddProfileComment(c *fiber.Ctx) error {
	userID := c.Params("id")
	targetOID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid user ID")
	}

	claims, ok := c.Locals("user").(*middleware.Claims)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}

	commenterOID, err := primitive.ObjectIDFromHex(claims.UserID)
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid commenter ID")
	}

	var body struct {
		Content  string `json:"content"`
		ParentID string `json:"parentId,omitempty"`
	}
	if err := c.BodyParser(&body); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if body.Content == "" {
		return utils.SendError(c, fiber.StatusBadRequest, "Content is required")
	}

	// Fetch commenter's actual data to prevent impersonation
	commenter, err := h.userService.GetUserByID(claims.UserID)
	if err != nil {
		return utils.SendError(c, fiber.StatusNotFound, "Commenter user not found")
	}

	if err := h.userService.AddProfileComment(targetOID, commenterOID, commenter.ZoomName, commenter.CohortNumber, body.Content, body.ParentID); err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error adding comment")
	}

	return utils.SendResponse(c, fiber.StatusCreated, "Comment added successfully", nil)
}

func (h *UserHandler) DeleteProfileComment(c *fiber.Ctx) error {
	userID := c.Params("id")
	commentID := c.Params("commentId")

	targetOID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid user ID")
	}

	commentOID, err := primitive.ObjectIDFromHex(commentID)
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid comment ID")
	}

	claims, ok := c.Locals("user").(*middleware.Claims)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}

	// Security: Only admin can delete any comment, or user can delete comment on their own profile?
	// Usually admin only for moderation, or the user who wrote the comment.
	// But the user specifically asked for admin UI.
	if claims.Role != "admin" {
		return utils.SendError(c, fiber.StatusForbidden, "Only admins can delete profile comments")
	}

	if err := h.userService.DeleteProfileComment(targetOID, commentOID); err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error deleting profile comment")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Comment deleted successfully", nil)
}

func (h *UserHandler) AddProfileReaction(c *fiber.Ctx) error {
	userID := c.Params("id")
	targetOID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid user ID")
	}

	claims, ok := c.Locals("user").(*middleware.Claims)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}

	reactorOID, err := primitive.ObjectIDFromHex(claims.UserID)
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid reactor ID")
	}

	var body struct {
		Type  string `json:"type"`
		Value string `json:"value"`
	}
	if err := c.BodyParser(&body); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if body.Value == "" {
		return utils.SendError(c, fiber.StatusBadRequest, "Reaction value is required")
	}

	if body.Type == "" {
		body.Type = "emoji"
	}

	if err := h.userService.AddProfileReaction(targetOID, reactorOID, body.Type, body.Value); err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error adding reaction")
	}

	return utils.SendResponse(c, fiber.StatusCreated, "Reaction added successfully", nil)
}

func (h *UserHandler) AddPlantReaction(c *fiber.Ctx) error {
	userID := c.Params("id")
	targetOID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid user ID")
	}

	claims, ok := c.Locals("user").(*middleware.Claims)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}

	reactorOID, err := primitive.ObjectIDFromHex(claims.UserID)
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid reactor ID")
	}

	if claims.Role != "admin" {
		me, err := h.userService.GetUserByID(claims.UserID)
		if err != nil {
			return utils.SendError(c, fiber.StatusForbidden, "You can only cheer your genmates' plants")
		}
		target, err := h.userService.GetUserByID(userID)
		if err != nil {
			return utils.SendError(c, fiber.StatusNotFound, "User not found")
		}
		if me.GenmateGroup == "" || me.GenmateGroup != target.GenmateGroup {
			return utils.SendError(c, fiber.StatusForbidden, "You can only cheer your genmates' plants")
		}
	}

	var body struct {
		Type  string `json:"type"`
		Value string `json:"value"`
	}
	if err := c.BodyParser(&body); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if body.Value == "" {
		return utils.SendError(c, fiber.StatusBadRequest, "Reaction value is required")
	}

	if body.Type == "" {
		body.Type = "emoji"
	}

	if err := h.userService.AddPlantReaction(targetOID, reactorOID, body.Type, body.Value); err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error adding reaction")
	}

	return utils.SendResponse(c, fiber.StatusCreated, "Reaction added successfully", nil)
}

func (h *UserHandler) GetDomainUserByID(id string) (*domain.User, error) {
	return h.userService.GetUserByID(id)
}

func (h *UserHandler) authenticateUser(email, password string) (string, string, string, error) {
	user, err := h.userService.GetUserByEmail(email)
	if err != nil {
		return "", "", "", errors.New("invalid credentials")
	}

	if !utils.CheckPasswordHash(password, user.Password) {
		return "", "", "", errors.New("invalid credentials")
	}

	token, err := utils.GenerateJWT(user.ID, user.Role, user.CohortNumber, "")
	if err != nil {
		return "", "", "", errors.New("could not generate token")
	}

	return token, user.Role, user.ID.Hex(), nil
}

func (h *UserHandler) createReflection(userID primitive.ObjectID, reflection domain.Reflection) (*domain.Reflection, error) {
	return h.userService.CreateReflection(userID, reflection)
}

func (h *UserHandler) getReflections(userID primitive.ObjectID) ([]domain.Reflection, error) {
	return h.userService.GetReflections(userID)
}
