package main

import (
	"gofiber-baro/internal/handler"
	middleware "gofiber-baro/pkg/middleware"

	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
)

type Handlers struct {
	User         *handler.UserHandler
	Admin        *handler.AdminHandler
	Attendance   *handler.AttendanceHandler
	Leave        *handler.LeaveHandler
	Holiday      *handler.HolidayHandler
	TalkBoard    *handler.TalkBoardHandler
	Notification *handler.NotificationHandler
	Stamp        *handler.StampHandler
}

func setupRoutes(app *fiber.App, h Handlers) {
	app.Post("/login", h.User.LoginUser)
	app.Get("/api/verify-token", middleware.AuthMiddleware, h.User.VerifyToken)

	notifications := app.Group("/api/notifications", middleware.AuthMiddleware)
	notifications.Get("", h.Notification.GetActiveNotifications)
	notifications.Post("/:id/read", h.Notification.MarkAsRead)

	protected := app.Group("/users", middleware.AuthMiddleware)
	protected.Get("/", h.User.GetAllUsers)
	protected.Get("/genmate-garden", h.User.GetGenmateGarden)
	protected.Get("/:id", h.User.GetUserByID)
	protected.Put("/:id", h.User.UpdateUser)
	protected.Post("/:id/reflections", h.User.CreateReflection)
	protected.Get("/:id/reflections", h.User.GetUserReflections)
	protected.Put("/:id/personal-details", h.User.UpdatePersonalDetails)
	protected.Post("/:id/profile/comments", h.User.AddProfileComment)
	protected.Delete("/:id/profile/comments/:commentId", h.User.DeleteProfileComment)
	protected.Post("/:id/profile/reactions", h.User.AddProfileReaction)
	protected.Post("/:id/plant/reactions", h.User.AddPlantReaction)
	protected.Post("/:id/fertilizer/protect", h.User.UseFertilizerProtect)
	protected.Post("/:id/fertilizer/feed", h.User.UseFertilizerFeed)

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
	admin.Delete("/users/:id", h.Admin.DeleteUser)
	admin.Post("/badges/bulk", h.Admin.BulkAwardBadge)
	admin.Post("/users/:id/fertilizer", h.Admin.GrantFertilizer)
	admin.Post("/fertilizer/bulk", h.Admin.BulkGrantFertilizer)
	admin.Patch("/users/:id/plant", h.Admin.UpdatePlantOverride)
	admin.Post("/users/bulk-register", h.Admin.BulkRegisterUsers)
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
	admin.Get("/attendance/daily-stats", h.Attendance.GetDailyAttendanceStats)
	admin.Get("/attendance/student/:id", h.Attendance.GetStudentAttendanceHistory)
	admin.Post("/attendance/lock", h.Attendance.LockSession)
	admin.Post("/attendance/bulk", h.Attendance.BulkMarkAttendance)
	admin.Delete("/attendance/:id", h.Attendance.DeleteAttendanceRecord)
	admin.Get("/attendance/export/salesforce", h.Attendance.ExportToSalesforce)
	admin.Get("/attendance/export", h.Attendance.ExportAttendance)
	admin.Patch("/users/:id/salesforce-id", h.Attendance.UpdateSalesforceID)
	admin.Patch("/users/:id/attendance-status", h.Attendance.UpdateAttendanceStatus)

	admin.Post("/holidays", h.Holiday.CreateHoliday)
	admin.Get("/holidays", h.Holiday.GetHolidays)
	admin.Delete("/holidays/:id", h.Holiday.DeleteHoliday)

	admin.Post("/leave-requests", h.Leave.CreateLeaveRequestAdmin)
	admin.Get("/leave-requests", h.Leave.GetAllLeaveRequests)
	admin.Patch("/leave-requests/:id", h.Leave.UpdateLeaveRequestStatus)

	admin.Post("/notifications", h.Notification.CreateNotification)
	admin.Get("/notifications", h.Notification.GetAllNotifications)
	admin.Put("/notifications/:id", h.Notification.UpdateNotification)
	admin.Delete("/notifications/:id", h.Notification.DeleteNotification)

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
	board.Delete("/posts/:postId", h.TalkBoard.DeletePost)
	board.Post("/posts/:postId/comments", h.TalkBoard.AddComment)
	board.Delete("/posts/:postId/comments/:commentId", h.TalkBoard.DeleteComment)
	board.Post("/posts/:postId/reactions", h.TalkBoard.AddReactionToPost)
	board.Delete("/posts/:postId/reactions", h.TalkBoard.RemoveReactionFromPost)
	board.Post("/posts/:postId/comments/:commentId/reactions", h.TalkBoard.AddReactionToComment)

	stamps := app.Group("/stamps", middleware.AuthMiddleware)
	stamps.Post("/", h.Stamp.CreateStamp)

	cohorts := app.Group("/cohorts", middleware.AuthMiddleware)
	cohorts.Get("/", h.Stamp.ListCohorts)
	cohorts.Get("/:cohortNumber", h.Stamp.GetCohort)
	cohorts.Get("/:cohortNumber/stamps", h.Stamp.GetCohortStamps)

	admin.Put("/cohorts/:cohortNumber", h.Stamp.SetCohortLockAt)
	admin.Post("/cohorts/:cohortNumber/poster", h.Stamp.UploadPoster)
	admin.Delete("/cohorts/:cohortNumber/stamps", h.Stamp.ClearCohortStamps)
	admin.Delete("/cohorts/:cohortNumber/stamps/:stampId", h.Stamp.DeleteStamp)
}
