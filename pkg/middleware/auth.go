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

// StaffStillActive, when set, is asked about every staff (role "admin") token: a removed
// team member's token stays valid for days, so it must not be enough on its own.
// It returns false when the account no longer has staff access.
var StaffStillActive func(userID string) (bool, error)

// Custom claims structure
type Claims struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
	Cohort int    `json:"cohort"`
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

	// The httpOnly session cookie is the normal way in; a Bearer header still works for
	// sessions from before the cookie (and for tools).
	tokenString, fromCookie := "", false
	if authHeader := c.Get("Authorization"); strings.HasPrefix(authHeader, "Bearer ") {
		tokenString = strings.TrimPrefix(authHeader, "Bearer ")
	} else if v := c.Cookies(SessionCookie); v != "" {
		tokenString, fromCookie = v, true
	}
	if tokenString == "" {
		return fiber.NewError(fiber.StatusUnauthorized, "Unauthorized: No token provided")
	}
	if fromCookie && !sameSiteWrite(c) {
		return fiber.NewError(fiber.StatusForbidden, "This request didn't come from Baro")
	}

	// Parse and validate the token
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		// Validate the signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secretKey), nil
	})
	if err != nil || !token.Valid {
		return fiber.NewError(fiber.StatusUnauthorized, "Unauthorized: Invalid token")
	}

	// Check token expiration
	if claims.ExpiresAt != nil && claims.ExpiresAt.Before(time.Now()) {
		return fiber.NewError(fiber.StatusUnauthorized, "Unauthorized: Token expired")
	}

	if claims.Role == "admin" && StaffStillActive != nil {
		active, err := StaffStillActive(claims.UserID)
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, "Could not check your access, try again")
		}
		if !active {
			return fiber.NewError(fiber.StatusForbidden, "Your staff access has ended")
		}
	}

	// Store the claims in the context for later use
	c.Locals("user", claims)
	c.Locals("userID", claims.UserID)
	c.Locals("token", tokenString)

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
