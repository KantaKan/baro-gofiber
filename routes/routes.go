package routes

import (
	"gofiber-baro/controllers"
	middleware "gofiber-baro/middlewares"

	"github.com/gofiber/fiber/v2"
)

func SetupRoutes(app *fiber.App) {

	// Public routes
	// app.Post("/register", controllers.RegisterUser)
	app.Post("/login", controllers.LoginUser)
	// Verify token route
	app.Get("/api/verify-token", middleware.AuthMiddleware, controllers.VerifyToken)
	
	// Protected user routes with JWT authentication
	protected := app.Group("/users", middleware.AuthMiddleware)
	
	protected.Get("/:id", controllers.GetUserProfile)
	protected.Post("/:id/reflections", controllers.CreateReflection)
	protected.Get("/:id/reflections", controllers.GetUserReflections)
	
	
	// Admin routes - only accessible to admin users
	admin := app.Group("/admin", middleware.AuthMiddleware, middleware.CheckAdminRole)
	admin.Get("/userreflections/:id", controllers.GetUserWithReflections) // New route
	admin.Get("/users", controllers.GetAllUsers)    
	admin.Get("/barometer",controllers.GetUserBarometerDataController)          // Admin can view all users
	admin.Get("/reflections", controllers.GetAllReflectionsController) // Admin can view all reflections
	admin.Get("/chart-data", controllers.GetChartData)
	admin.Get("/reflections/chartday", controllers.GetBarometerData)
	admin.Get("/reflections/weekly", controllers.GetWeeklySummary)
	
	// New route for the emoji zone table API
	admin.Get("/emoji-zone-table", controllers.GetEmojiZoneTableDataController)

	// Talk Board routes
	board := app.Group("/board", middleware.AuthMiddleware)
	board.Get("/posts", controllers.GetPosts)
	board.Get("/posts/:postId", controllers.GetPost) // Add this line
	board.Post("/posts", controllers.CreatePost)
	board.Post("/posts/:postId/comments", controllers.AddComment)
	board.Post("/posts/:postId/reactions", controllers.AddReactionToPost)
	board.Delete("/posts/:postId/reactions", controllers.RemoveReactionFromPost)
	board.Post("/posts/:postId/comments/:commentId/reactions", controllers.AddReactionToComment)
}