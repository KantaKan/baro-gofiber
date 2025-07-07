package controllers

import (
	middleware "gofiber-baro/middlewares"
	"gofiber-baro/models"
	"gofiber-baro/services"
	"gofiber-baro/utils"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// RegisterUser handles user registration
// @Summary Register new user
// @Description Register a new user in the system
// @Tags auth
// @Accept json
// @Produce json
// @Param user body models.User true "User registration details"
// @Success 201 {object} utils.StandardResponse{data=models.User} "User successfully registered"
// @Failure 400 {object} utils.StandardResponse "Invalid request body"
// @Failure 500 {object} utils.StandardResponse "Error creating user"
// @Router /register [post]
func RegisterUser(c *fiber.Ctx) error {
	var user models.User

	// Parse JSON body into user struct
	if err := c.BodyParser(&user); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}

	// Validate required fields
	if user.Email == "" || user.Password == "" || user.FirstName == "" || user.LastName == "" {
		return utils.SendError(c, fiber.StatusBadRequest, "All fields are required")
	}

	// Ensure cohort number is set, default to 9 if not provided
	if user.CohortNumber == 0 {
		user.CohortNumber = 9 // Default to cohort 9 if not provided
	}

	// Call the service to create the user
	createdUser, err := services.CreateUser(user)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error creating user")
	}

	// Send successful response
	return utils.SendResponse(c, fiber.StatusCreated, "User successfully registered", createdUser)
}

// LoginUser handles user login
// @Summary Login user
// @Description Authenticate user and get JWT token
// @Tags auth
// @Accept json
// @Produce json
// @Param loginData body object{email=string,password=string} true "Login credentials"
// @Success 200 {object} utils.StandardResponse{data=object{token=string,role=string}} "Login successful"
// @Failure 400 {object} utils.StandardResponse "Invalid request body"
// @Failure 401 {object} utils.StandardResponse "Invalid credentials"
// @Router /login [post]
func LoginUser(c *fiber.Ctx) error {
	var loginData struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	// Parse the request body into the loginData struct
	if err := c.BodyParser(&loginData); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}

	// Validate required fields
	if loginData.Email == "" || loginData.Password == "" {
		return utils.SendError(c, fiber.StatusBadRequest, "Email and password are required")
	}

	// Authenticate the user (now returns token and role)
	token, role, err := services.AuthenticateUser(loginData.Email, loginData.Password)
	if err != nil {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid credentials")
	}

	// Send successful response with JWT token and role
	return utils.SendResponse(c, fiber.StatusOK, "Login successful", map[string]interface{}{
		"token": token,
		"role":  role,
	})
}

// VerifyToken verifies the validity of the token
// @Summary Verify JWT token
// @Description Verify if the provided JWT token is valid
// @Tags auth
// @Security BearerAuth
// @Produce json
// @Success 200 {object} utils.StandardResponse{data=object{role=string}} "Token is valid"
// @Failure 401 {object} utils.StandardResponse "Invalid token claims"
// @Router /api/verify-token [get]
func VerifyToken(c *fiber.Ctx) error {
	claims, ok := c.Locals("user").(*middleware.Claims)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Token is valid", map[string]string{
		"role": claims.Role,
	})
}

// GetUserProfile retrieves the user profile
// @Summary Get user profile
// @Description Get user profile by ID
// @Tags users
// @Security BearerAuth
// @Param id path string true "User ID"
// @Produce json
// @Success 200 {object} utils.StandardResponse{data=models.User} "User profile retrieved"
// @Failure 404 {object} utils.StandardResponse "User not found"
// @Router /users/{id} [get]
func GetUserProfile(c *fiber.Ctx) error {
	userID := c.Params("id") // Get user ID from route parameters

	// Extract claims from context
	claims, ok := c.Locals("user").(*middleware.Claims)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}

	// Only allow if user is admin or accessing their own profile
	if claims.Role != "admin" && claims.UserID.Hex() != userID {
		return utils.SendError(c, fiber.StatusForbidden, "You are not allowed to access this user's data")
	}

	// Call the service to get user by ID
	user, err := services.GetUserByID(userID)
	if err != nil {
		return utils.SendError(c, fiber.StatusNotFound, "User not found")
	}

	// Send successful response with user data
	return utils.SendResponse(c, fiber.StatusOK, "User profile retrieved", user)
}

// CreateReflection creates a new reflection
// @Summary Create reflection
// @Description Create a new reflection for a user
// @Tags reflections
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Param reflection body models.Reflection true "Reflection data"
// @Success 201 {object} utils.StandardResponse{data=models.Reflection} "Reflection created"
// @Failure 400 {object} utils.StandardResponse "Invalid request body"
// @Failure 409 {object} utils.StandardResponse "Already submitted reflection today"
// @Router /users/{id}/reflections [post]
func CreateReflection(c *fiber.Ctx) error {
	userID := c.Params("id") // Get user ID from route parameters
	objectID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid user ID")
	}

	// Extract claims from context
	claims, ok := c.Locals("user").(*middleware.Claims)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}

	// Only allow if user is admin or posting for themselves
	if claims.Role != "admin" && claims.UserID.Hex() != userID {
		return utils.SendError(c, fiber.StatusForbidden, "You are not allowed to post reflection for this user")
	}

	// Parse reflection data from the request body
	var reflection models.Reflection
	if err := c.BodyParser(&reflection); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid reflection data")
	}

	// Set the user_id in the reflection data
	reflection.UserID = objectID

	// Set the date to the current time if not provided
	if reflection.Date.IsZero() {
		reflection.Date = time.Now() // Set the current time to the Date field
	}

	// Set the Day field to the current date (this will replace the ID field)
	reflection.Day = reflection.Date.Format("2006-01-02")

	// Call service to create the reflection
	createdReflection, err := services.CreateReflection(objectID, reflection)
	if err != nil {
		if err.Error() == "user has already created a reflection today" {
			return utils.SendError(c, fiber.StatusConflict, "You have already submitted a reflection today. Please try again tomorrow.")
		}
		return utils.SendError(c, fiber.StatusInternalServerError, "Error creating reflection")
	}

	// Send successful response
	return utils.SendResponse(c, fiber.StatusCreated, "Reflection successfully created", createdReflection)
}

// GetUserReflections retrieves user reflections
// @Summary Get user reflections
// @Description Get all reflections for a specific user
// @Tags reflections
// @Security BearerAuth
// @Param id path string true "User ID"
// @Produce json
// @Success 200 {object} utils.StandardResponse{data=[]models.Reflection} "Reflections retrieved"
// @Failure 500 {object} utils.StandardResponse "Error retrieving reflections"
// @Router /users/{id}/reflections [get]
func GetUserReflections(c *fiber.Ctx) error {
	userID := c.Params("id") // Get user ID from route parameters
	objectID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid user ID")
	}

	// Extract claims from context
	claims, ok := c.Locals("user").(*middleware.Claims)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}

	// Only allow if user is admin or accessing their own reflections
	if claims.Role != "admin" && claims.UserID.Hex() != userID {
		return utils.SendError(c, fiber.StatusForbidden, "You are not allowed to access this user's reflections")
	}

	// Call service to get all reflections for the user
	reflections, err := services.GetReflections(objectID)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error retrieving reflections")
	}

	// Send successful response with reflections
	return utils.SendResponse(c, fiber.StatusOK, "User reflections retrieved", reflections)
}


