package middleware

import (
	"errors"
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt"
)

// AuthenticateJWT is middleware to verify the JWT token
func AuthenticateJWT(c *fiber.Ctx) error {
	// Get JWT token from Authorization header
	authHeader := c.Get("Authorization")
	if authHeader == "" {
		return fiber.NewError(fiber.StatusUnauthorized, "Authorization header is required")
	}

	// Extract the token from the Authorization header (Bearer <token>)
	tokenString := strings.Split(authHeader, " ")[1]

	// Load JWT_SECRET_KEY from environment variables
	jwtSecret := os.Getenv("JWT_SECRET_KEY")
	if jwtSecret == "" {
		return fiber.NewError(fiber.StatusInternalServerError, "JWT_SECRET_KEY not set in environment variables")
	}

	// Parse and validate the token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Ensure the token's signing method is HMAC with SHA256
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("invalid signing method")
		}
		return []byte(jwtSecret), nil
	})
	if err != nil || !token.Valid {
		return fiber.NewError(fiber.StatusUnauthorized, "Invalid or expired token")
	}

	// Extract claims
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || claims["role"] == nil {
		return fiber.NewError(fiber.StatusUnauthorized, "Invalid token claims")
	}

	// Check if the user has admin role
	role := claims["role"].(string)
	if role != "admin" {
		return fiber.NewError(fiber.StatusForbidden, "Access denied")
	}

	// Proceed to the next handler if authentication and role check are successful
	return c.Next()
}

func CheckAdminRole(c *fiber.Ctx) error {
	// Retrieve the JWT token
	authHeader := c.Get("Authorization")
	if authHeader == "" {
		return fiber.NewError(fiber.StatusUnauthorized, "Missing token")
	}

	tokenString := strings.Split(authHeader, "Bearer ")[1]
	if tokenString == "" {
		return fiber.NewError(fiber.StatusUnauthorized, "Invalid token format")
	}

	// Parse the JWT token
	secretKey := os.Getenv("JWT_SECRET_KEY")
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Check the signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fiber.NewError(fiber.StatusUnauthorized, "Invalid signing method")
		}
		return []byte(secretKey), nil
	})

	if err != nil || !token.Valid {
		return fiber.NewError(fiber.StatusUnauthorized, "Invalid token")
	}

	// Check if the user has an admin role
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || claims["role"] != "admin" {
		return fiber.NewError(fiber.StatusForbidden, "Not authorized")
	}

	// Continue to the next middleware/handler if admin
	return c.Next()
}