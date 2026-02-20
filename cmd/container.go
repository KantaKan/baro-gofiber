package main

import (
	"gofiber-baro/internal/domain"
	"gofiber-baro/internal/handler"
	"gofiber-baro/internal/repository"
	"gofiber-baro/internal/service/attendance"
	"gofiber-baro/internal/service/holiday"
	leaveService "gofiber-baro/internal/service/leave"
	reflectionService "gofiber-baro/internal/service/reflection"
	userService "gofiber-baro/internal/service/user"

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

	UserService                 *userService.Service
	BadgeService                *userService.BadgeService
	ReflectionService           *reflectionService.Service
	BarometerService            *reflectionService.BarometerService
	LeaveService                *leaveService.Service
	HolidayService              *holiday.Service
	AttendanceCodeService       *attendance.CodeService
	AttendanceSubmissionService *attendance.SubmissionService
	AttendanceStatsService      *attendance.StatsService
	AttendanceOverviewService   *attendance.OverviewService

	UserHandler       *handler.UserHandler
	AdminHandler      *handler.AdminHandler
	AttendanceHandler *handler.AttendanceHandler
	LeaveHandler      *handler.LeaveHandler
	HolidayHandler    *handler.HolidayHandler
	TalkBoardHandler  *handler.TalkBoardHandler
}

func NewContainer(db *mongo.Database) *Container {
	c := &Container{DB: db}

	c.initRepositories()
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
}

func (c *Container) initServices() {
	c.UserService = userService.NewService(c.UserRepo)
	c.BadgeService = userService.NewBadgeService(c.UserRepo)
	c.ReflectionService = reflectionService.NewService(c.DB)
	c.BarometerService = reflectionService.NewBarometerService(c.DB)
	c.LeaveService = leaveService.NewService(c.LeaveRepo, c.UserService)
	c.HolidayService = holiday.NewService(c.HolidayRepo, c.DB)

	c.AttendanceCodeService = attendance.NewCodeService(c.AttendanceCodeRepo, c.AttendanceRepo, c.UserService)
	c.AttendanceSubmissionService = attendance.NewSubmissionService(c.AttendanceRepo, c.UserService)
	c.AttendanceStatsService = attendance.NewStatsService(c.AttendanceRepo, c.UserService)
	c.AttendanceOverviewService = attendance.NewOverviewService(c.AttendanceRepo, c.AttendanceCodeRepo, c.UserService)
}

func (c *Container) initHandlers() {
	c.UserHandler = handler.NewUserHandler(c.UserService)
	c.AdminHandler = handler.NewAdminHandler(c.UserService, c.BadgeService, c.ReflectionService, c.BarometerService)
	c.AttendanceHandler = handler.NewAttendanceHandler(
		c.AttendanceCodeService,
		c.AttendanceSubmissionService,
		c.AttendanceStatsService,
		c.AttendanceOverviewService,
		c.UserService,
	)
	c.LeaveHandler = handler.NewLeaveHandler(c.LeaveService, c.UserService)
	c.HolidayHandler = handler.NewHolidayHandler(c.HolidayService)
	c.TalkBoardHandler = handler.NewTalkBoardHandler(c.TalkBoardRepo)
}
