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
	History      *handler.HistoryHandler
	Cosmetic     *handler.CosmeticHandler
	GiftBox      *handler.GiftBoxHandler
	Character    *handler.BaroCharacterHandler
	Showcase     *handler.ShowcaseHandler
	GodEvent     *handler.GodEventHandler
	StartupStory *handler.StartupStoryHandler
	Audit        fiber.Handler
}

func setupRoutes(app *fiber.App, h Handlers) {
	app.Use(h.Audit)

	loginLimiter := limiter.New(limiter.Config{
		Max:        10,
		Expiration: 1 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "Too many login attempts, please try again later",
			})
		},
	})
	app.Post("/login", loginLimiter, h.User.LoginUser)
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
	protected.Post("/:id/reflection-rewards/reconcile", h.User.ReconcileReflectionMilestones)
	protected.Put("/:id/personal-details", h.User.UpdatePersonalDetails)
	protected.Post("/:id/profile/comments", h.User.AddProfileComment)
	protected.Delete("/:id/profile/comments/:commentId", h.User.DeleteProfileComment)
	protected.Post("/:id/profile/reactions", h.User.AddProfileReaction)
	protected.Post("/:id/plant/reactions", h.User.AddPlantReaction)
	protected.Get("/:id/care-energy", h.User.GetCareEnergy)
	protected.Post("/:id/care-energy/protect", h.User.UseCareEnergyProtect)
	protected.Post("/:id/care-energy/feed", h.User.UseCareEnergyFeed)
	protected.Post("/:id/care-energy/gift", h.User.GiftCareEnergy)
	protected.Post("/:id/care-energy/rescue", h.User.RescueCareEnergy)
	protected.Post("/:id/care-energy/character-care", h.User.CareForCharacter)

	app.Get("/holidays", middleware.AuthMiddleware, h.Holiday.GetHolidays)

	cosmetics := app.Group("/plant-cosmetics", middleware.AuthMiddleware)
	cosmetics.Get("/catalog", h.Cosmetic.GetCatalog)
	cosmetics.Get("/collection", h.Cosmetic.GetCollection)
	cosmetics.Put("/equipment/:slot", h.Cosmetic.EquipCosmetic)
	cosmetics.Delete("/equipment/:slot", h.Cosmetic.UnequipCosmetic)

	characterCosmetics := app.Group("/character-cosmetics", middleware.AuthMiddleware)
	characterCosmetics.Get("/catalog", h.Cosmetic.GetCharacterCatalog)
	characterCosmetics.Get("/collection", h.Cosmetic.GetCharacterCollection)
	characterCosmetics.Put("/equipment/:slot", h.Cosmetic.EquipCharacterCosmetic)
	characterCosmetics.Delete("/equipment/:slot", h.Cosmetic.UnequipCharacterCosmetic)

	giftBoxes := app.Group("/gift-boxes", middleware.AuthMiddleware)
	giftBoxes.Get("", h.GiftBox.List)
	giftBoxes.Get("/recipients", h.GiftBox.SearchRecipients)
	giftBoxes.Get("/:id/odds", h.GiftBox.Odds)
	giftBoxes.Post("/:id/open", h.GiftBox.Open)
	giftBoxes.Post("/:id/transfer", h.GiftBox.Transfer)

	characters := app.Group("/baro-characters", middleware.AuthMiddleware)
	characters.Get("", h.Character.Collection)
	characters.Get("/growth", h.Character.Growth)
	characters.Get("/selection", h.Character.Selection)
	characters.Put("/equipped", h.Character.Equip)
	characters.Put("/pinned", h.Character.Pin)
	characters.Post("/reveal", h.Character.RevealStarter)

	lawn := app.Group("/showcase-lawn", middleware.AuthMiddleware)
	lawn.Get("", h.Showcase.List)
	lawn.Get("/me", h.Showcase.Mine)
	lawn.Put("/me", h.Showcase.Save)
	lawn.Delete("/me", h.Showcase.Remove)
	lawn.Put("/me/mood", h.Showcase.SetMood)
	lawn.Put("/me/emote", h.Showcase.SetEmote)
	lawn.Post("/:ownerId/reactions", h.Showcase.React)

	app.Get("/god-events", middleware.AuthMiddleware, h.GodEvent.List)

	startup := app.Group("/startup-story", middleware.AuthMiddleware)
	startup.Get("", h.StartupStory.Overview)
	startup.Get("/leaderboard", h.StartupStory.Leaderboard)
	startup.Put("/opt-out", h.StartupStory.OptOut)
	startup.Post("/runs", h.StartupStory.StartRun)
	startup.Post("/runs/active/founder", h.StartupStory.PickFounder)
	startup.Post("/runs/active/projects", h.StartupStory.StartProject)
	startup.Post("/runs/active/ship", h.StartupStory.Ship)
	startup.Post("/runs/active/item", h.StartupStory.PickItem)
	startup.Post("/runs/active/abandon", h.StartupStory.Abandon)
	startup.Post("/runs/active/ipo-choice", h.StartupStory.IPOChoice)
	startup.Post("/runs/active/hire", h.StartupStory.Hire)
	startup.Delete("/runs/active/staff/:id", h.StartupStory.Dismiss)

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
	admin.Post("/users/:id/care-energy", h.Admin.GrantCareEnergy)
	admin.Post("/care-energy/bulk", h.Admin.BulkGrantCareEnergy)
	admin.Patch("/users/:id/plant", h.Admin.UpdatePlantOverride)
	admin.Get("/users/:id/cosmetics", h.Cosmetic.GetAdminCollection)
	admin.Post("/users/:id/cosmetics/:cosmeticId", h.Cosmetic.GrantCosmetic)
	admin.Get("/users/:id/character-cosmetics", h.Cosmetic.GetAdminCharacterCollection)
	admin.Post("/users/:id/character-cosmetics/:cosmeticId", h.Cosmetic.GrantCharacterCosmetic)
	admin.Get("/users/:id/baro-characters", h.Character.AdminCollection)
	admin.Get("/users/:id/baro-characters/selection", h.Character.AdminSelection)
	admin.Post("/users/:id/baro-characters", h.Character.AdminGrant)
	admin.Put("/users/:id/baro-characters/equipped", h.Character.AdminEquip)
	admin.Put("/users/:id/baro-characters/pinned", h.Character.AdminPin)
	admin.Delete("/users/:id/cosmetics/:cosmeticId", h.Cosmetic.RevokeCosmetic)
	admin.Post("/users/:id/gift-boxes", h.GiftBox.Grant)
	admin.Post("/cohorts/:cohortNumber/gift-boxes", h.GiftBox.GrantCohort)
	admin.Put("/showcase-lawn/:ownerId/moderation", h.Showcase.Moderate)
	admin.Post("/god-events", handler.GodEventCastLimiter(), h.GodEvent.Cast)
	admin.Get("/cohorts/:cohortNumber/gift-boxes/recipients", h.GiftBox.PreviewAudience)
	admin.Post("/users/bulk-register", h.Admin.BulkRegisterUsers)
	admin.Put("/users/:userId/reflections/:reflectionId/feedback", h.Admin.UpdateReflectionFeedback)
	admin.Get("/barometer", h.Admin.GetUserBarometerData)
	admin.Get("/reflections", h.Admin.GetAllReflections)
	admin.Get("/reflections/chartday", h.Admin.GetAllUsersBarometerData)
	admin.Get("/reflections/weekly", h.Admin.GetWeeklySummary)
	admin.Get("/emoji-zone-table", h.Admin.GetEmojiZoneTableData)
	admin.Get("/history", h.History.GetHistory)

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
	cohorts.Get("/:cohortNumber/garden", h.User.GetCohortGarden)

	admin.Put("/cohorts/:cohortNumber", h.Stamp.SetCohortLockAt)
	admin.Post("/cohorts/:cohortNumber/poster", h.Stamp.UploadPoster)
	admin.Delete("/cohorts/:cohortNumber/stamps", h.Stamp.ClearCohortStamps)
	admin.Delete("/cohorts/:cohortNumber/stamps/:stampId", h.Stamp.DeleteStamp)
}
