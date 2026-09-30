package main

import (
	"log"

	"gofiber-baro/internal/domain"
	"gofiber-baro/internal/handler"
	"gofiber-baro/internal/repository"
	"gofiber-baro/internal/service/achievement"
	"gofiber-baro/internal/service/attendance"
	"gofiber-baro/internal/service/character"
	"gofiber-baro/internal/service/giftbox"
	"gofiber-baro/internal/service/godevent"
	"gofiber-baro/internal/service/holiday"
	leaveService "gofiber-baro/internal/service/leave"
	"gofiber-baro/internal/service/milestone"
	notificationService "gofiber-baro/internal/service/notification"
	reflectionService "gofiber-baro/internal/service/reflection"
	"gofiber-baro/internal/service/reward"
	"gofiber-baro/internal/service/showcase"
	userService "gofiber-baro/internal/service/user"
	"gofiber-baro/internal/storage"
	"gofiber-baro/pkg/middleware"

	"github.com/gofiber/fiber/v2"
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
	AuditLogRepo       domain.AuditLogRepository
	GiftBoxRepo        *repository.GiftBoxRepository
	CharacterRepo      *repository.BaroCharacterRepository
	ShowcaseRepo       *repository.ShowcaseRepository
	GodEventRepo       *repository.GodEventRepository

	StampStorage storage.Storage

	UserService                 *userService.Service
	BadgeService                *userService.BadgeService
	CareEnergyService           *userService.CareEnergyService
	CosmeticService             *userService.CosmeticService
	ReflectionService           *reflectionService.Service
	BarometerService            *reflectionService.BarometerService
	LeaveService                *leaveService.Service
	HolidayService              *holiday.Service
	NotificationService         *notificationService.Service
	RewardService               *reward.Service
	GiftBoxService              *giftbox.Service
	CharacterService            *character.Service
	CharacterGrowthService      *character.GrowthService
	CharacterOwnershipService   *character.OwnershipService
	CharacterEggService         *character.EggService
	ShowcaseService             *showcase.Service
	GodEventService             *godevent.Service
	MilestoneService            *milestone.Service
	AchievementService          *achievement.Service
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
	HistoryHandler      *handler.HistoryHandler
	CosmeticHandler     *handler.CosmeticHandler
	GiftBoxHandler      *handler.GiftBoxHandler
	CharacterHandler    *handler.BaroCharacterHandler
	ShowcaseHandler     *handler.ShowcaseHandler
	GodEventHandler     *handler.GodEventHandler

	AuditMiddleware fiber.Handler
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
	c.AuditLogRepo = repository.NewAuditLogRepository(c.DB)
	c.GiftBoxRepo = repository.NewGiftBoxRepository(c.DB)
	c.CharacterRepo = repository.NewBaroCharacterRepository(c.DB)
	c.ShowcaseRepo = repository.NewShowcaseRepository(c.DB)
	c.GodEventRepo = repository.NewGodEventRepository(c.DB)
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
	c.CareEnergyService = userService.NewCareEnergyService(c.UserRepo, c.HolidayService)
	c.NotificationService = notificationService.NewService(c.NotificationRepo)
	c.CosmeticService = userService.NewCosmeticService(c.UserRepo, c.NotificationService)
	rewardCatalog := append(c.CosmeticService.Catalog(), c.CosmeticService.CharacterCatalog()...)
	c.RewardService = reward.NewService(c.GiftBoxRepo, reward.NewSelector(reward.SystemRandom{}), rewardCatalog)
	c.CharacterService = character.NewService(c.CharacterRepo, character.SecurePicker{})
	c.CharacterGrowthService = character.NewGrowthService(c.UserRepo, c.HolidayService)
	c.CharacterOwnershipService = character.NewOwnershipService(c.CharacterRepo, character.SecurePicker{})
	c.CharacterEggService = character.NewEggService(c.CharacterRepo, character.SecurePicker{})
	c.GiftBoxService = giftbox.NewService(c.GiftBoxRepo, c.RewardService, c.CharacterEggService)
	c.ShowcaseService = showcase.NewService(c.ShowcaseRepo)
	c.GodEventService = godevent.NewService(c.GodEventRepo)
	c.MilestoneService = milestone.NewService(c.UserRepo, c.GiftBoxRepo, c.HolidayService)
	c.AchievementService = achievement.NewService(c.UserRepo, c.GiftBoxRepo, c.HolidayService)

	c.AttendanceCodeService = attendance.NewCodeService(c.AttendanceCodeRepo, c.AttendanceRepo, c.UserService)
	c.AttendanceSubmissionService = attendance.NewSubmissionService(c.AttendanceRepo, c.UserService)
	c.AttendanceStatsService = attendance.NewStatsService(c.AttendanceRepo, c.UserService)
	c.AttendanceOverviewService = attendance.NewOverviewService(c.AttendanceRepo, c.AttendanceCodeRepo, c.UserService)
	c.AttendanceExportService = attendance.NewExportService(c.AttendanceRepo, c.UserService)
}

func (c *Container) initHandlers() {
	c.UserHandler = handler.NewUserHandler(c.UserService, c.CareEnergyService, c.MilestoneService, c.AchievementService)
	c.AdminHandler = handler.NewAdminHandler(c.UserService, c.BadgeService, c.CareEnergyService, c.ReflectionService, c.BarometerService)
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
	c.HistoryHandler = handler.NewHistoryHandler(c.AuditLogRepo, c.UserRepo)
	c.CosmeticHandler = handler.NewCosmeticHandler(c.CosmeticService)
	c.GiftBoxHandler = handler.NewGiftBoxHandler(c.GiftBoxService)
	c.CharacterHandler = handler.NewBaroCharacterHandler(c.CharacterService, c.CharacterGrowthService, c.CharacterOwnershipService)
	c.ShowcaseHandler = handler.NewShowcaseHandler(c.ShowcaseService)
	c.GodEventHandler = handler.NewGodEventHandler(c.GodEventService)

	c.AuditMiddleware = middleware.AuditMiddleware(c.AuditLogRepo)
}
