package main

import (
	"context"
	"errors"
	"gofiber-baro/config"
	"gofiber-baro/internal/domain"
	"gofiber-baro/internal/jobs"
	"gofiber-baro/internal/service/attendance"
	"gofiber-baro/internal/service/cup"
	"gofiber-baro/pkg/middleware"
	"gofiber-baro/pkg/utils"
	"log"
	"net/url"
	"os"
	"time"

	"github.com/gofiber/fiber/v2/middleware/helmet"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/recover"

	_ "gofiber-baro/docs"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/swagger"
	"github.com/joho/godotenv"
)

// @title Generation Barometer API
// @version 1.0
// @description API Server for Generation Barometer Application
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.email generationth@generation.org

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:3000
// @BasePath /
// @schemes http https

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Enter your bearer token in the format **Bearer <token>**

func main() {
	if os.Getenv("ENVIRONMENT") != "production" {
		if err := godotenv.Load(); err != nil {
			log.Println("No .env file found, using environment variables")
		}
	}

	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		log.Fatal("MONGO_URI environment variable is not set")
	}

	dbName := os.Getenv("DATABASE_NAME")
	if dbName == "" {
		log.Fatal("DATABASE_NAME environment variable is not set")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	if err := config.InitializeDB(mongoURI, dbName); err != nil {
		log.Fatal("Failed to initialize database:", err)
	}
	log.Println("Successfully connected to MongoDB")

	container := NewContainer(config.DB)
	// A removed team member's token stays valid for days; check staff tokens against the database.
	middleware.StaffStillActive = func(id string) (bool, error) {
		u, err := container.UserService.GetUserByID(id)
		if errors.Is(err, domain.ErrUserNotFound) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		return u.Role == "admin", nil
	}
	indexCtx, cancelIndexes := context.WithTimeout(context.Background(), 10*time.Second)
	if err := container.CharacterRepo.EnsureIndexes(indexCtx); err != nil {
		cancelIndexes()
		log.Fatal("Failed to ensure Baro character indexes:", err)
	}
	cancelIndexes()
	indexCtx, cancelIndexes = context.WithTimeout(context.Background(), 10*time.Second)
	if err := container.GiftBoxRepo.EnsureIndexes(indexCtx); err != nil {
		cancelIndexes()
		log.Fatal("Failed to ensure gift box indexes:", err)
	}
	cancelIndexes()
	indexCtx, cancelIndexes = context.WithTimeout(context.Background(), 10*time.Second)
	if err := container.CareRepo.EnsureIndexes(indexCtx); err != nil {
		log.Fatal("Failed to ensure care indexes:", err)
	}
	if err := container.CalendarRepo.EnsureIndexes(indexCtx); err != nil {
		log.Fatal("Failed to ensure calendar indexes:", err)
	}
	if err := container.ReportRepo.EnsureIndexes(indexCtx); err != nil {
		log.Fatal("Failed to ensure weekly report indexes:", err)
	}
	if err := container.SlackRepo.EnsureIndexes(indexCtx); err != nil {
		log.Fatal("Failed to ensure Slack log indexes:", err)
	}
	if err := container.StaffRepo.EnsureIndexes(indexCtx); err != nil {
		log.Fatal("Failed to ensure staff invite indexes:", err)
	}
	if err := container.CupRepo.EnsureIndexes(indexCtx); err != nil {
		log.Fatal("Failed to ensure GenMate Cup indexes:", err)
	}
	if err := container.GodEventRepo.EnsureIndexes(indexCtx); err != nil {
		cancelIndexes()
		log.Fatal("Failed to ensure GOD event indexes:", err)
	}
	cancelIndexes()
	indexCtx, cancelIndexes = context.WithTimeout(context.Background(), 10*time.Second)
	if err := container.StartupStoryRepo.EnsureIndexes(indexCtx); err != nil {
		cancelIndexes()
		log.Fatal("Failed to ensure Startup Story indexes:", err)
	}
	cancelIndexes()

	go jobs.RunCohortLockJob(context.Background(), config.DB, time.Hour)
	go jobs.RunWeeklyReportJob(context.Background(), reportJob{slack: container.SlackService, rep: container.ReportService}, time.Minute)

	// Close check-in sessions: once a minute, mark learners who didn't check in.
	closer := attendance.NewSessionCloser(container.AttendanceRepo, container.LeaveRepo, container.UserService, container.CohortService, container.HolidayService)
	cupRunner := cup.NewRunner(container.CupService, container.CupRepo, container.AttendanceRepo, container.CohortService, container.GiftBoxService,
		func(t time.Time) (bool, error) { return attendance.IsClassDay(t, container.HolidayService) })
	go func() {
		for range time.Tick(time.Minute) {
			now := utils.GetThailandTime()
			if err := closer.CloseIfDue(now); err != nil {
				log.Printf("session closer: %v", err)
			}
			// After the closer, so the showing-up bonus sees every learner's record.
			if err := cupRunner.RunDue(now); err != nil {
				log.Printf("cup runner: %v", err)
			}
		}
	}()

	// Behind a load balancer the client IP arrives in a header (e.g. X-Forwarded-For);
	// set PROXY_HEADER so the check-in network limit sees the learner's real IP.
	app := fiber.New(fiberConfig(os.Getenv("PROXY_HEADER")))

	app.Use(recover.New())

	allowedOrigins := os.Getenv("CORS_ALLOWED_ORIGINS")
	if allowedOrigins == "" {
		allowedOrigins = "http://localhost:5173,https://generation-barometer.vercel.app"
	}

	app.Use(cors.New(cors.Config{
		AllowOrigins:     allowedOrigins,
		AllowMethods:     "GET,POST,PUT,PATCH,DELETE,OPTIONS",
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization, X-Requested-With",
		AllowCredentials: true,
		MaxAge:           3600,
	}))

	app.Options("/*", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusNoContent)
	})
	app.Use(helmet.New(helmet.Config{
		ContentSecurityPolicy: buildCSP(),
		XSSProtection:         "1; mode=block",
		ContentTypeNosniff:    "nosniff",
		ReferrerPolicy:        "no-referrer",
	}))

	app.Use(limiter.New(limiter.Config{
		Max:        300,
		Expiration: 1 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			authHeader := c.Get("Authorization")
			if authHeader != "" {
				return authHeader
			}
			return c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "Too many requests",
			})
		},
	}))

	app.Get("/health", func(c *fiber.Ctx) error {
		log.Println("Health check route called")
		return c.SendString("OK ")
	})

	handlers := Handlers{
		User:         container.UserHandler,
		Admin:        container.AdminHandler,
		Attendance:   container.AttendanceHandler,
		Leave:        container.LeaveHandler,
		Holiday:      container.HolidayHandler,
		TalkBoard:    container.TalkBoardHandler,
		Notification: container.NotificationHandler,
		Cohort:       container.CohortHandler,
		Cup:          container.CupHandler,
		AdminViews:   container.AdminViewsHandler,
		AdminMark:    container.AdminMarkHandler,
		Care:         container.CareHandler,
		Wellbeing:    container.WellbeingHandler,
		CohortAdmin:  container.CohortAdminHandler,
		Activity:     container.ActivityHandler,
		StaffGuard:   container.StaffGuard,
		Team:         container.TeamHandler,
		Slack:        container.SlackHandler,
		Report:       container.ReportHandler,
		Calendar:     container.CalendarHandler,
		CohortList:   container.CohortListHandler,
		Stamp:        container.StampHandler,
		History:      container.HistoryHandler,
		Cosmetic:     container.CosmeticHandler,
		GiftBox:      container.GiftBoxHandler,
		Character:    container.CharacterHandler,
		Showcase:     container.ShowcaseHandler,
		GodEvent:     container.GodEventHandler,
		StartupStory: container.StartupStoryHandler,
		Audit:        container.AuditMiddleware,
	}

	setupRoutes(app, handlers)

	app.Get("/swagger/*", swagger.HandlerDefault)

	log.Printf("🚀 Server is running on http://localhost:%s", port)
	log.Printf("Environment: %s", os.Getenv("ENVIRONMENT"))
	log.Printf("MongoDB URI: %s", mongoURI[:10]+"...")
	log.Printf("Database Name: %s", dbName)
	log.Printf("Port: %s", port)
	log.Fatal(app.Listen(":" + port))
}

// fiberConfig reads the client IP from proxyHeader when set. EnableIPValidation makes
// c.IP() return the first valid address of a list like "203.0.113.5, 10.0.0.2".
func fiberConfig(proxyHeader string) fiber.Config {
	cfg := fiber.Config{}
	if proxyHeader != "" {
		cfg.ProxyHeader = proxyHeader
		cfg.EnableIPValidation = true
	}
	return cfg

}

func buildCSP() string {
	csp := "default-src 'self'; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; font-src 'self' https://fonts.gstatic.com; script-src 'self' 'unsafe-inline' 'unsafe-eval'; img-src 'self' data:"

	publicBase := os.Getenv("SUPABASE_STORAGE_PUBLIC_URL")
	if publicBase != "" {
		u, err := url.Parse(publicBase)
		if err == nil && u.Host != "" {
			csp += " https://" + u.Host
		}
	}

	return csp
}
