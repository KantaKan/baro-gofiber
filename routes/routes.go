package routes

import (
	"gofiber-baro/controllers"
	middleware "gofiber-baro/middlewares"
	"gofiber-baro/utils"
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt"
)

func SetupRoutes(app *fiber.App) {

	app.Get("/test-token", func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.JSON(fiber.Map{
				"error": "No Authorization header",
			})
		}
		
		parts := strings.Split(authHeader, " ")
		tokenString := parts[1]
		
		// Try to validate the token
		userID, err := utils.ValidateJWT(tokenString)
		if err != nil {
			return c.JSON(fiber.Map{
				"error": err.Error(),
				"token_start": tokenString[:10],
			})
		}
		
		return c.JSON(fiber.Map{
			"valid": true,
			"user_id": userID,
		})
	})
	app.Get("/debug-token", func(c *fiber.Ctx) error {
		token := c.Get("Authorization")
		if token == "" {
			return c.JSON(fiber.Map{"error": "no token provided"})
		}
		
		// Remove "Bearer " if present
		token = strings.TrimPrefix(token, "Bearer ")
		
		// Split the token
		parts := strings.Split(token, ".")
		if len(parts) != 3 {
			return c.JSON(fiber.Map{"error": "invalid token format"})
		}
		
		// Decode the claims part (middle part)
		claimBytes, err := jwt.DecodeSegment(parts[1])
		if err != nil {
			return c.JSON(fiber.Map{"error": err.Error()})
		}
		
		return c.JSON(fiber.Map{
			"token_parts": len(parts),
			"header": parts[0],
			"claims": string(claimBytes),
			"signature": parts[2],
		})
	})
	app.Get("/debug/jwt", func(c *fiber.Ctx) error {
		key := os.Getenv("JWT_SECRET_KEY")
		token := c.Get("Authorization")
		
		return c.JSON(fiber.Map{
			"key_length": len(key),
			"key_preview": key[:4] + "...", // first 4 chars
			"token": token,
		})
	})
	// Public routes
	app.Post("/register", controllers.RegisterUser)
	app.Post("/login", controllers.LoginUser)

	// Protected user routes with JWT authentication
	protected := app.Group("/users",middleware.AuthMiddleware)
	protected.Get("/:id", controllers.GetUserProfile)
	protected.Post("/:id/reflections", controllers.CreateReflection)
	protected.Get("/:id/reflections", controllers.GetUserReflections)

	// Admin routes - only accessible to admin users
	admin := app.Group("/admin", middleware.AuthMiddleware, middleware.CheckAdminRole) // JWT + Admin role check
	admin.Get("/users", controllers.GetAllUsers)              // Admin can view all users
	admin.Get("/users/:id/reflections", controllers.GetUserReflections) // Admin can view specific user reflections
	admin.Get("/users/panic", controllers.GetPanicUsers) 
	admin.Get("/users/reflections/weekly", controllers.GetReflectionsByWeek)     // Get panic users
}
