package routes

import (
	"gofiber-baro/controllers"
	middleware "gofiber-baro/middlewares"

	"github.com/gofiber/fiber/v2"
)

// SetupRoutes sets up all the routes for the application
// @title Barometer API
// @version 1.0
// @description This is the API documentation for the Barometer application
// @termsOfService http://swagger.io/terms/
// @contact.name API Support
// @contact.email support@example.com
// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html
// @host 127.0.0.1:3000
// @BasePath /
func SetupRoutes(app *fiber.App) {
	// Public routes
	// @Summary Register new user
	// @Description Register a new user in the system
	// @Tags auth
	// @Accept json
	// @Produce json
	// @Param user body models.User true "User registration details"
	// @Success 201 {object} models.User
	// @Failure 400 {object} utils.Response
	// @Failure 500 {object} utils.Response
	// @Router /register [post]
	app.Post("/register", controllers.RegisterUser)

	// @Summary Login user
	// @Description Authenticate user and return JWT token
	// @Tags auth
	// @Accept json
	// @Produce json
	// @Param credentials body LoginRequest true "Login credentials"
	// @Success 200 {object} TokenResponse
	// @Failure 400,401 {object} utils.Response
	// @Router /login [post]
	app.Post("/login", controllers.LoginUser)

	// @Summary Verify JWT token
	// @Description Verify if the JWT token is valid
	// @Tags auth
	// @Security ApiKeyAuth
	// @Accept json
	// @Produce json
	// @Success 200 {object} utils.Response
	// @Failure 401 {object} utils.Response
	// @Router /api/verify-token [get]
	app.Get("/api/verify-token", middleware.AuthMiddleware, controllers.VerifyToken)

	// Protected routes
	protected := app.Group("/users", middleware.AuthMiddleware)

	// @Summary Get user profile
	// @Description Get user profile by ID
	// @Tags users
	// @Security ApiKeyAuth
	// @Accept json
	// @Produce json
	// @Param id path string true "User ID"
	// @Success 200 {object} models.User
	// @Failure 401,404 {object} utils.Response
	// @Router /users/{id} [get]
	protected.Get("/:id", controllers.GetUserProfile)

	// @Summary Create reflection
	// @Description Create a new reflection for a user
	// @Tags reflections
	// @Security ApiKeyAuth
	// @Accept json
	// @Produce json
	// @Param id path string true "User ID"
	// @Param reflection body models.Reflection true "Reflection details"
	// @Success 201 {object} models.Reflection
	// @Failure 400,401,409 {object} utils.Response
	// @Router /users/{id}/reflections [post]
	protected.Post("/:id/reflections", controllers.CreateReflection)

	// @Summary Get user reflections
	// @Description Get all reflections for a user
	// @Tags reflections
	// @Security ApiKeyAuth
	// @Accept json
	// @Produce json
	// @Param id path string true "User ID"
	// @Success 200 {object} utils.Response{data=[]models.Reflection}
	// @Failure 401,404 {object} utils.Response
	// @Router /users/{id}/reflections [get]
	protected.Get("/:id/reflections", controllers.GetUserReflections)

	// Admin routes
	admin := app.Group("/admin", middleware.AuthMiddleware)

	// @Summary Get all users
	// @Description Get a list of all users
	// @Tags admin
	// @Security ApiKeyAuth
	// @Accept json
	// @Produce json
	// @Success 200 {array} models.User
	// @Failure 401,500 {object} utils.Response
	// @Router /admin/users [get]
	admin.Get("/users", controllers.GetAllUsers)

	// @Summary Get barometer data
	// @Description Get barometer statistics
	// @Tags admin
	// @Security ApiKeyAuth
	// @Accept json
	// @Produce json
	// @Param timeRange query string false "Time range (90d, 30d, 7d)" default(90d)
	// @Success 200 {array} services.BarometerData
	// @Failure 400,401,500 {object} utils.Response
	// @Router /admin/barometer [get]
	admin.Get("/barometer", controllers.GetBarometerData)

	// @Summary Get all reflections
	// @Description Get all reflections with pagination
	// @Tags admin
	// @Security ApiKeyAuth
	// @Accept json
	// @Produce json
	// @Param page query integer false "Page number" default(1) minimum(1)
	// @Param limit query integer false "Items per page" default(10) minimum(1)
	// @Success 200 {object} utils.Response{data=[]models.ReflectionWithUser}
	// @Failure 401,500 {object} utils.Response
	// @Router /admin/reflections [get]
	admin.Get("/reflections", controllers.GetAllReflectionsController)

	// @Summary Get user with reflections
	// @Description Get user details with their reflections
	// @Tags admin
	// @Security ApiKeyAuth
	// @Accept json
	// @Produce json
	// @Param id path string true "User ID"
	// @Success 200 {object} utils.Response{data=services.UserWithReflections}
	// @Failure 400,401,500 {object} utils.Response
	// @Router /admin/userreflections/{id} [get]
	admin.Get("/userreflections/:id", controllers.GetUserWithReflections)

	// @Summary Get chart data
	// @Description Get weekly reflection data for charts
	// @Tags admin
	// @Security ApiKeyAuth
	// @Accept json
	// @Produce json
	// @Success 200 {array} map[string]interface{}
	// @Failure 401,500 {object} utils.Response
	// @Router /admin/chart-data [get]
	admin.Get("/chart-data", controllers.GetChartData)

	// @Summary Get daily barometer data
	// @Description Get daily barometer statistics
	// @Tags admin
	// @Security ApiKeyAuth
	// @Accept json
	// @Produce json
	// @Success 200 {object} utils.Response{data=[]services.BarometerData}
	// @Failure 401,500 {object} utils.Response
	// @Router /admin/reflections/chartday [get]
	admin.Get("/reflections/chartday", controllers.GetBarometerData)
}
