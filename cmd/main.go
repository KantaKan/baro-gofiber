package main

import (
	"gofiber-baro/config"
	"gofiber-baro/internal/service/attendance"
	"gofiber-baro/pkg/utils"
	"log"
	"os"
	"time"

	"github.com/gofiber/fiber/v2/middleware/helmet"
	"github.com/gofiber/fiber/v2/middleware/limiter"

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

	// Close check-in sessions: once a minute, mark learners who didn't check in.
	closer := attendance.NewSessionCloser(container.AttendanceRepo, container.LeaveRepo, container.UserService, container.CohortService, container.HolidayService)
	go func() {
		for range time.Tick(time.Minute) {
			if err := closer.CloseIfDue(utils.GetThailandTime()); err != nil {
				log.Printf("session closer: %v", err)
			}
		}
	}()

	// Behind a load balancer the client IP arrives in a header (e.g. X-Forwarded-For);
	// set PROXY_HEADER so the check-in network limit sees the learner's real IP.
	fiberCfg := fiber.Config{}
	if h := os.Getenv("PROXY_HEADER"); h != "" {
		fiberCfg.ProxyHeader = h
	}
	app := fiber.New(fiberCfg)

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
		ContentSecurityPolicy: "default-src 'self'; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; font-src 'self' https://fonts.gstatic.com; script-src 'self' 'unsafe-inline' 'unsafe-eval'; img-src 'self' data:",
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
