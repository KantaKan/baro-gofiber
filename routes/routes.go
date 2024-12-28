package routes

import (
	"gofiber-baro/controllers"
	"gofiber-baro/middlewares"

	"github.com/gofiber/fiber/v2"
)

func SetupRoutes(app *fiber.App) {
	// Public routes
	app.Post("/register", controllers.RegisterUser)
	app.Post("/login", controllers.LoginUser)

	// Protected user routes with JWT authentication
	protected := app.Group("/users", middleware.AuthenticateJWT)
	protected.Get("/:id", controllers.GetUserProfile)
	protected.Post("/:id/reflections", controllers.CreateReflection)
	protected.Get("/:id/reflections", controllers.GetUserReflections)

	// Admin routes - only accessible to admin users
	admin := app.Group("/admin", middleware.AuthenticateJWT, middleware.CheckAdminRole) // JWT + Admin role check
	admin.Get("/users", controllers.GetAllUsers)              // Admin can view all users
	admin.Get("/users/:id/reflections", controllers.GetUserReflections) // Admin can view specific user reflections
	admin.Get("/users/panic", controllers.GetPanicUsers)      // Get panic users
}
