package main

import (
	"gofiber-baro/config"
	"gofiber-baro/routes"
	"gofiber-baro/services"
	"log"
	"os"
	"time"

	"github.com/gofiber/fiber/v2/middleware/helmet"
	"github.com/gofiber/fiber/v2/middleware/limiter"

	"gofiber-baro/controllers"
	_ "gofiber-baro/docs" // This will be generated

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
	// Load environment variables
	if os.Getenv("ENVIRONMENT") != "production" {
		// Only try to load .env file in non-production environment
		if err := godotenv.Load(); err != nil {
			log.Println("No .env file found, using environment variables")
		}
	}

	// Verify that required environment variables are set
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
		port = "3000" // default port
	}

	if err := config.InitializeDB(mongoURI, dbName); err != nil {
		log.Fatal("Failed to initialize database:", err)
	}
	log.Println("Successfully connected to MongoDB")

	services.InitUserService()

	app := fiber.New()
	app.Use(helmet.New(helmet.Config{
		ContentSecurityPolicy: "default-src 'self'; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; font-src 'self' https://fonts.gstatic.com; script-src 'self' 'unsafe-inline' 'unsafe-eval'; img-src 'self' data:",
		XSSProtection:        "1; mode=block",
		ContentTypeNosniff:   "nosniff",
		ReferrerPolicy:       "no-referrer",
	}))

	app.Use(limiter.New(limiter.Config{
		Max:        100,              // Max number of requests
		Expiration: 1 * time.Minute,  // Per minute
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP() // Rate limit by IP
		},
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "Too many requests",
			})
		},
	}))

	// Add health check route before CORS middleware
	app.Get("/health", func(c *fiber.Ctx) error {
		log.Println("Health check route called")
		return c.SendString("OK ")
	})
	app.Get("/spreadsheet-data", controllers.GetSpreadsheetData)
	// Get CORS allowed origins from environment variable, default to localhost if not set


	// Configure CORS
	app.Use(cors.New(cors.Config{
		AllowOrigins:     "https://generation-barometer.vercel.app/,http://localhost:5173",        // Use environment variable
		AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization",
		AllowCredentials: true,
		MaxAge:           3600, // Cache preflight response for 1 hour
	}))

	routes.SetupRoutes(app)

	// Add Swagger documentation route
	app.Get("/swagger/*", swagger.HandlerDefault)

	log.Printf("🚀 Server is running on http://localhost:%s", port)
	log.Printf("Environment: %s", os.Getenv("ENVIRONMENT"))
	log.Printf("MongoDB URI: %s", mongoURI[:10]+"...") // Only log the beginning for security
	log.Printf("Database Name: %s", dbName)
	log.Printf("Port: %s", port)
	log.Fatal(app.Listen(":" + port))
}
