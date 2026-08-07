package main

import (
	"log"

	"gofiber-baro/internal/domain"
	"gofiber-baro/internal/handler"
	"gofiber-baro/internal/repository"
	"gofiber-baro/internal/service/attendance"
	"gofiber-baro/internal/service/holiday"
	leaveService "gofiber-baro/internal/service/leave"
	notificationService "gofiber-baro/internal/service/notification"
	reflectionService "gofiber-baro/internal/service/reflection"
	userService "gofiber-baro/internal/service/user"
	"gofiber-baro/internal/storage"

	"go.mongodb.org/mongo-driver/mongo"
)

type Container struct {
	DB *mongo.Database

	UserRepo           domain.UserRepository
	AttendanceRepo     domain.AttendanceRepository
	AttendanceCodeRepo domain.AttendanceCodeRepository
	LeaveRepo          domain.LeaveRequestRepository
	HolidayRepo        domain.HolidayRepository
	TalkBoardRepo      domain.TalkBoardRepository
	NotificationRepo   domain.NotificationRepository
	StampRepo          domain.StampRepository
	CohortRepo         domain.CohortRepository

	StampStorage storage.Storage

	UserService                 *userService.Service
	BadgeService                *userService.BadgeService
	FertilizerService           *userService.FertilizerService
	ReflectionService           *reflectionService.Service
	BarometerService            *reflectionService.BarometerService
	LeaveService                *leaveService.Service
	HolidayService              *holiday.Service
	NotificationService         *notificationService.Service
	AttendanceCodeService       *attendance.CodeService
	AttendanceSubmissionService *attendance.SubmissionService
	AttendanceStatsService      *attendance.StatsService
	AttendanceOverviewService   *attendance.OverviewService
	AttendanceExportService     *attendance.ExportService

	UserHandler         *handler.UserHandler
	AdminHandler        *handler.AdminHandler
	AttendanceHandler   *handler.AttendanceHandler
	LeaveHandler        *handler.LeaveHandler
	HolidayHandler      *handler.HolidayHandler
	TalkBoardHandler    *handler.TalkBoardHandler
	NotificationHandler *handler.NotificationHandler
	StampHandler        *handler.StampHandler
}

func NewContainer(db *mongo.Database) *Container {
	c := &Container{DB: db}

	c.initRepositories()
	c.initStorage()
	c.initServices()
	c.initHandlers()

	return c
}

func (c *Container) initRepositories() {
	c.UserRepo = repository.NewUserRepository(c.DB)
	c.AttendanceRepo = repository.NewAttendanceRepository(c.DB)
	c.AttendanceCodeRepo = repository.NewAttendanceCodeRepository(c.DB)
	c.LeaveRepo = repository.NewLeaveRequestRepository(c.DB)
	c.HolidayRepo = repository.NewHolidayRepository(c.DB)
	c.TalkBoardRepo = repository.NewTalkBoardRepository(c.DB)
	c.NotificationRepo = repository.NewNotificationRepository(c.DB)
	c.StampRepo = repository.NewStampRepository(c.DB)
	c.CohortRepo = repository.NewCohortRepository(c.DB)
}

func (c *Container) initStorage() {
	s, err := storage.NewSupabaseStorage()
	if err != nil {
		log.Printf("WARNING: supabase storage not configured: %v", err)
		s = nil
	}
	c.StampStorage = s
}

func (c *Container) initServices() {
	c.UserService = userService.NewService(c.UserRepo)
	c.BadgeService = userService.NewBadgeService(c.UserRepo)
	c.ReflectionService = reflectionService.NewService(c.DB)
	c.BarometerService = reflectionService.NewBarometerService(c.DB)
	c.LeaveService = leaveService.NewService(c.LeaveRepo, c.UserService)
	c.HolidayService = holiday.NewService(c.HolidayRepo, c.DB)
	c.FertilizerService = userService.NewFertilizerService(c.UserRepo, c.HolidayService)
	c.NotificationService = notificationService.NewService(c.NotificationRepo)

	c.AttendanceCodeService = attendance.NewCodeService(c.AttendanceCodeRepo, c.AttendanceRepo, c.UserService)
	c.AttendanceSubmissionService = attendance.NewSubmissionService(c.AttendanceRepo, c.UserService)
	c.AttendanceStatsService = attendance.NewStatsService(c.AttendanceRepo, c.UserService)
	c.AttendanceOverviewService = attendance.NewOverviewService(c.AttendanceRepo, c.AttendanceCodeRepo, c.UserService)
	c.AttendanceExportService = attendance.NewExportService(c.AttendanceRepo, c.UserService)
}

func (c *Container) initHandlers() {
	c.UserHandler = handler.NewUserHandler(c.UserService, c.FertilizerService)
	c.AdminHandler = handler.NewAdminHandler(c.UserService, c.BadgeService, c.FertilizerService, c.ReflectionService, c.BarometerService)
	c.AttendanceHandler = handler.NewAttendanceHandler(
		c.AttendanceCodeService,
		c.AttendanceSubmissionService,
		c.AttendanceStatsService,
		c.AttendanceOverviewService,
		c.AttendanceExportService,
		c.UserService,
	)
	c.LeaveHandler = handler.NewLeaveHandler(c.LeaveService, c.UserService)
	c.HolidayHandler = handler.NewHolidayHandler(c.HolidayService)
	c.TalkBoardHandler = handler.NewTalkBoardHandler(c.TalkBoardRepo, c.UserService)
	c.NotificationHandler = handler.NewNotificationHandler(c.NotificationService)
	c.StampHandler = handler.NewStampHandler(c.StampRepo, c.CohortRepo, c.UserService, c.StampStorage)
}
