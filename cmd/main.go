package main

import (
	"fmt"
	"gofiber-baro/config"
	"gofiber-baro/routes"
	"gofiber-baro/services"
	"log"
	"os"
	"time"

	"github.com/gofiber/fiber/v2/middleware/helmet"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/swagger"

	_ "gofiber-baro/docs" // import swagger docs

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/joho/godotenv"
)

// @title Barometer API
// @version 1.0
// @description This is the API documentation for the Barometer application
// @termsOfService http://swagger.io/terms/
// @contact.name API Support
// @contact.email support@example.com
// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html
// @host 127.0.0.1:3000
// @BasePath /
// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name Authorization
// @description Enter your Bearer token in the format: Bearer <token>
// @schemes http https
func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}

	mongoURI := os.Getenv("MONGO_URI")
	databaseName := os.Getenv("DATABASE_NAME")

	if mongoURI == "" || databaseName == "" {
		log.Fatal("MONGO_URI or DATABASE_NAME not set in environment variables")
	}

	if err := config.InitializeDB(mongoURI, databaseName); err != nil {
		log.Fatal("Failed to initialize database:", err)
	}
	log.Println("Successfully connected to MongoDB")

	services.InitUserService()

	app := fiber.New()
	app.Use(helmet.New(helmet.Config{
		ContentSecurityPolicy: "default-src 'self'; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; font-src 'self' https://fonts.gstatic.com; script-src 'self' 'unsafe-inline' 'unsafe-eval'; img-src 'self' data:;",
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
	// Configure CORS to allow localhost:5173
	app.Use(cors.New(cors.Config{
		AllowOrigins:     "*", // Allow Vite development server
		AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization",
		AllowCredentials: false,
		MaxAge:           3600, // Cache preflight response for 1 hour
	}))

	// Debug log for Swagger docs
	fmt.Println("Setting up Swagger handler...")
	
	// Swagger route with custom config
	app.Get("/swagger/*", swagger.New(swagger.Config{
		URL:         "http://127.0.0.1:3000/swagger/doc.json",
		DeepLinking: true,
	}))

	// Setup routes
	fmt.Println("Setting up routes...")
	routes.SetupRoutes(app)

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	fmt.Printf("Server starting on port %s...\n", port)
	log.Fatal(app.Listen(":" + port))
}
