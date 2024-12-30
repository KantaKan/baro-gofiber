package controllers

import (
	"gofiber-baro/models"
	"gofiber-baro/services"
	"gofiber-baro/utils"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// RegisterUser handles user registration
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

	// Authenticate the user (you need to modify this function to check credentials and return role)
	token, err := services.AuthenticateUser(loginData.Email, loginData.Password)
	if err != nil {
		return utils.SendError(c, fiber.StatusUnauthorized, "eeeInvalid credentials")
	}

	// Send successful response with JWT token
	return utils.SendResponse(c, fiber.StatusOK, "Login successful", map[string]string{
		"token": token,
	})
}

// GetUserProfile retrieves the user profile by ID
func GetUserProfile(c *fiber.Ctx) error {
	userID := c.Params("id") // Get user ID from route parameters

	// Call the service to get user by ID
	user, err := services.GetUserByID(userID)
	if err != nil {
		return utils.SendError(c, fiber.StatusNotFound, "User not found")
	}

	// Send successful response with user data
	return utils.SendResponse(c, fiber.StatusOK, "User profile retrieved", user)
}

// CreateReflection handles the creation of a new reflection for a user
func CreateReflection(c *fiber.Ctx) error {
	userID := c.Params("id") // Get user ID from route parameters
	objectID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid user ID")
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
	createdReflection, err := services.CreateReflection(objectID, reflection) // Pass both userID and reflection
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error creating reflection")
	}

	// Send successful response
	return utils.SendResponse(c, fiber.StatusCreated, "Reflection successfully created", createdReflection)
}

// GetUserReflections retrieves all reflections for a user
func GetUserReflections(c *fiber.Ctx) error {
	userID := c.Params("id") // Get user ID from route parameters
	objectID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid user ID")
	}

	// Call service to get all reflections for the user
	reflections, err := services.GetReflections(objectID)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error retrieving reflections")
	}

	// Send successful response with reflections
	return utils.SendResponse(c, fiber.StatusOK, "User reflections retrieved", reflections)
}
