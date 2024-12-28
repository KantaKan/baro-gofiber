package routes

import (
	"gofiber-baro/controllers"

	"github.com/gofiber/fiber/v2"
)

func SetupRoutes(app *fiber.App) {
	// Public routes
	app.Post("/register", controllers.RegisterUser)
	app.Post("/login", controllers.LoginUser)

	// Protected routes (add JWT middleware later)
	app.Get("/users/:id", controllers.GetUserProfile)

	// Add reflection routes
	app.Post("/users/:id/reflections", controllers.CreateReflection)  // Create reflection for a user
	app.Get("/users/:id/reflections", controllers.GetUserReflections)  // Get all reflections for a user
}
