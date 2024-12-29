package main

import (
	"log"
	"os"

	"gofiber-baro/config"
	"gofiber-baro/routes"
	"gofiber-baro/services"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/joho/godotenv"
)

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

    // Configure CORS to allow localhost:5173
    app.Use(cors.New(cors.Config{
        AllowOrigins:     "http://localhost:5173", // Allow Vite development server
        AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS",
        AllowHeaders:     "Origin, Content-Type, Accept, Authorization",
        AllowCredentials: true,
        MaxAge:           3600, // Cache preflight response for 1 hour
    }))

    routes.SetupRoutes(app)

    port := os.Getenv("PORT")
    if port == "" {
        port = "3000"
    }

    log.Printf("🚀 Server is running on http://localhost:%s", port)
    log.Fatal(app.Listen(":" + port))
}
