package main

import (
	"gofiber-baro/internal/handler"
	middleware "gofiber-baro/pkg/middleware"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"time"
)

type Handlers struct {
	User       *handler.UserHandler
	Admin      *handler.AdminHandler
	Attendance *handler.AttendanceHandler
	Leave      *handler.LeaveHandler
	Holiday    *handler.HolidayHandler
	TalkBoard  *handler.TalkBoardHandler
}

func setupRoutes(app *fiber.App, h Handlers) {
	app.Post("/login", h.User.LoginUser)
	app.Get("/api/verify-token", middleware.AuthMiddleware, h.User.VerifyToken)

	protected := app.Group("/users", middleware.AuthMiddleware)
	protected.Get("/:id", h.User.GetUserProfile)
	protected.Post("/:id/reflections", h.User.CreateReflection)
	protected.Get("/:id/reflections", h.User.GetUserReflections)

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
	admin.Get("/users", h.Admin.GetAllUsers)
	admin.Get("/userreflections/:id", h.Admin.GetUserWithReflections)
	admin.Post("/users/:id/badges", h.Admin.AwardBadge)
	admin.Put("/users/:userId/reflections/:reflectionId/feedback", h.Admin.UpdateReflectionFeedback)
	admin.Get("/barometer", h.Admin.GetUserBarometerData)
	admin.Get("/reflections", h.Admin.GetAllReflections)
	admin.Get("/reflections/chartday", h.Admin.GetAllUsersBarometerData)
	admin.Get("/reflections/weekly", h.Admin.GetWeeklySummary)
	admin.Get("/emoji-zone-table", h.Admin.GetEmojiZoneTableData)

	admin.Post("/attendance/generate-code", h.Attendance.GenerateAttendanceCode)
	admin.Get("/attendance/active-code", h.Attendance.GetActiveAttendanceCode)
	admin.Get("/attendance/today", h.Attendance.GetTodayOverview)
	admin.Post("/attendance/manual", h.Attendance.ManualMarkAttendance)
	admin.Get("/attendance/logs", h.Attendance.GetAttendanceLogs)
	admin.Get("/attendance/stats", h.Attendance.GetAttendanceStats)
	admin.Get("/attendance/stats-by-days", h.Attendance.GetAttendanceStatsByDays)
	admin.Get("/attendance/daily-stats", h.Attendance.GetDailyAttendanceStats)
	admin.Get("/attendance/student/:id", h.Attendance.GetStudentAttendanceHistory)
	admin.Post("/attendance/lock", h.Attendance.LockSession)
	admin.Post("/attendance/bulk", h.Attendance.BulkMarkAttendance)
	admin.Delete("/attendance/:id", h.Attendance.DeleteAttendanceRecord)

	admin.Post("/holidays", h.Holiday.CreateHoliday)
	admin.Get("/holidays", h.Holiday.GetHolidays)
	admin.Delete("/holidays/:id", h.Holiday.DeleteHoliday)

	admin.Post("/leave-requests", h.Leave.CreateLeaveRequestAdmin)
	admin.Get("/leave-requests", h.Leave.GetAllLeaveRequests)
	admin.Patch("/leave-requests/:id", h.Leave.UpdateLeaveRequestStatus)

	student := app.Group("/attendance", middleware.AuthMiddleware)
	student.Post("/submit", h.Attendance.SubmitAttendance)
	student.Get("/my-status", h.Attendance.GetMyAttendanceStatus)
	student.Get("/my-history", h.Attendance.GetMyAttendanceHistory)
	student.Get("/my-daily-stats", h.Attendance.GetMyDailyStats)
	student.Get("/code", h.Attendance.GetActiveAttendanceCode)

	leave := app.Group("/leave-requests", middleware.AuthMiddleware)
	leave.Post("/", h.Leave.CreateLeaveRequest)
	leave.Get("/my", h.Leave.GetMyLeaveRequests)

	board := app.Group("/board", middleware.AuthMiddleware)
	board.Get("/posts", h.TalkBoard.GetPosts)
	board.Get("/posts/:postId", h.TalkBoard.GetPost)
	board.Post("/posts", h.TalkBoard.CreatePost)
	board.Post("/posts/:postId/comments", h.TalkBoard.AddComment)
	board.Post("/posts/:postId/reactions", h.TalkBoard.AddReactionToPost)
	board.Delete("/posts/:postId/reactions", h.TalkBoard.RemoveReactionFromPost)
	board.Post("/posts/:postId/comments/:commentId/reactions", h.TalkBoard.AddReactionToComment)
}
