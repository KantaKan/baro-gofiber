package routes

import (
	"gofiber-baro/controllers"
	middleware "gofiber-baro/middlewares"

	"github.com/gofiber/fiber/v2"
)

func SetupRoutes(app *fiber.App) {

	// Public routes
	app.Post("/register", controllers.RegisterUser)
	app.Post("/login", controllers.LoginUser)

	// Protected user routes with JWT authentication
	protected := app.Group("/users", middleware.AuthMiddleware)
	protected.Get("/:id", controllers.GetUserProfile)
	protected.Post("/:id/reflections", controllers.CreateReflection)
	protected.Get("/:id/reflections", controllers.GetUserReflections)

	// Admin routes - only accessible to admin users
	admin := app.Group("/admin",middleware.AuthMiddleware) // JWT + Admin role check
	admin.Get("/users", controllers.GetAllUsers)    
	admin.Get("/barometer",controllers.GetUserBarometerDataController)          // Admin can view all users
}
