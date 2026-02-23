package middleware

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
	"github.com/joho/godotenv"
)

// Custom claims structure
type Claims struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// Load environment variables
func init() {
	if err := godotenv.Load(); err != nil {
		fmt.Println("Error loading .env file")
	}
}

// AuthMiddleware handles JWT authentication
func AuthMiddleware(c *fiber.Ctx) error {
	// Get the secret key from environment variables
	secretKey := os.Getenv("JWT_SECRET_KEY")
	if secretKey == "" {
		return fiber.NewError(fiber.StatusInternalServerError, "Secret key not configured")
	}

	// Extract token from the Authorization header
	authHeader := c.Get("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		return fiber.NewError(fiber.StatusUnauthorized, "Unauthorized: No token provided")
	}
	tokenString := strings.TrimPrefix(authHeader, "Bearer ")

	// Parse and validate the token
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(secretKey), nil
	})
	if err != nil || !token.Valid {
		return fiber.NewError(fiber.StatusUnauthorized, "Unauthorized: Invalid token")
	}

	// Check token expiration
	if claims.ExpiresAt != nil && claims.ExpiresAt.Before(time.Now()) {
		return fiber.NewError(fiber.StatusUnauthorized, "Unauthorized: Token expired")
	}

	// Store the claims in the context for later use
	c.Locals("user", claims)
	c.Locals("userID", claims.UserID)

	return c.Next()
}

// CheckAdminRole middleware to validate that the user is an admin
func CheckAdminRole(c *fiber.Ctx) error {
	claims, ok := c.Locals("user").(*Claims)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "Invalid token claims")
	}

	if claims.Role != "admin" {
		return fiber.NewError(fiber.StatusForbidden, "Access denied: admin role required")
	}

	return c.Next()
}
