package routes

import (
	"gofiber-baro/controllers"
	middleware "gofiber-baro/middlewares"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
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

	// Admin routes - only accessible to admin users with higher rate limit (300 req/min)
	adminLimiter := limiter.New(limiter.Config{
		Max:        300,
		Expiration: 1 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "Too many requests",
			})
		},
	})
	admin := app.Group("/admin", middleware.AuthMiddleware, middleware.CheckAdminRole, adminLimiter)
	admin.Get("/userreflections/:id", controllers.GetUserWithReflections)                                // New route
	admin.Post("/users/:id/badges", controllers.AwardBadgeToUser)                                        // New route to award badges
	admin.Put("/users/:userId/reflections/:reflectionId/feedback", controllers.UpdateReflectionFeedback) // New route to update reflection feedback
	admin.Get("/users", controllers.GetAllUsers)
	admin.Get("/barometer", controllers.GetUserBarometerDataController) // Admin can view all users
	admin.Get("/reflections", controllers.GetAllReflectionsController)  // Admin can view all reflections
	admin.Get("/chart-data", controllers.GetChartData)
	admin.Get("/reflections/chartday", controllers.GetBarometerData)
	admin.Get("/reflections/weekly", controllers.GetWeeklySummary)

	// New route for the emoji zone table API
	admin.Get("/emoji-zone-table", controllers.GetEmojiZoneTableDataController)

	// Attendance routes
	admin.Post("/attendance/generate-code", controllers.GenerateAttendanceCode)
	admin.Get("/attendance/active-code", controllers.GetActiveAttendanceCode)
	admin.Get("/attendance/today", controllers.GetTodayOverview)
	admin.Post("/attendance/manual", controllers.ManualMarkAttendance)
	admin.Get("/attendance/logs", controllers.GetAttendanceLogs)
	admin.Get("/attendance/stats", controllers.GetAttendanceStats)
	admin.Get("/attendance/stats-by-days", controllers.GetAttendanceStatsByDays)
	admin.Get("/attendance/daily-stats", controllers.GetDailyAttendanceStats)
	admin.Get("/attendance/student/:id", controllers.GetStudentAttendanceHistory)
	admin.Post("/attendance/lock", controllers.LockSession)
	admin.Post("/attendance/bulk", controllers.BulkMarkAttendance)
	admin.Delete("/attendance/:id", controllers.DeleteAttendanceRecord)

	// Student attendance routes
	student := app.Group("/attendance", middleware.AuthMiddleware)
	student.Post("/submit", controllers.SubmitAttendance)
	student.Get("/my-status", controllers.GetMyAttendanceStatus)
	student.Get("/my-history", controllers.GetMyAttendanceHistory)
	student.Get("/my-daily-stats", controllers.GetMyDailyStats)
	student.Get("/code", controllers.GetActiveAttendanceCode)

	// Leave request routes - student
	leave := app.Group("/leave-requests", middleware.AuthMiddleware)
	leave.Post("/", controllers.CreateLeaveRequest)
	leave.Get("/my", controllers.GetMyLeaveRequests)

	// Leave request routes - admin
	admin.Post("/leave-requests", controllers.AdminCreateLeaveRequest)
	admin.Get("/leave-requests", controllers.GetAllLeaveRequests)
	admin.Patch("/leave-requests/:id", controllers.UpdateLeaveRequestStatus)

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
