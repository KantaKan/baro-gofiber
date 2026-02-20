package handler

import (
	"errors"
	"gofiber-baro/internal/domain"
	"gofiber-baro/internal/service/user"
	middleware "gofiber-baro/pkg/middleware"
	"gofiber-baro/pkg/utils"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type UserHandler struct {
	userService *user.Service
	db          interface{}
}

func NewUserHandler(userService *user.Service) *UserHandler {
	return &UserHandler{userService: userService}
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

	user, err := h.userService.GetUserByID(id)
	if err != nil {
		return utils.SendError(c, fiber.StatusNotFound, "User not found")
	}

	return utils.SendResponse(c, fiber.StatusOK, "User retrieved", user)
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

	var body map[string]interface{}
	if err := c.BodyParser(&body); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if err := h.userService.UpdateUser(id, body); err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error updating user")
	}

	return utils.SendResponse(c, fiber.StatusOK, "User updated successfully", nil)
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

func (h *UserHandler) GetAllUsers(c *fiber.Ctx) error {
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

	token, err := utils.GenerateJWT(user.ID, user.Role, "")
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
