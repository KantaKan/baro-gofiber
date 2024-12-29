package utils

import (
	"errors"
	"log"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Claims defines the structure of the JWT claims
type Claims struct {
	UserID string `json:"user_id"`
	jwt.RegisteredClaims
}

// getJWTKey ensures the secret key is loaded at runtime
func getJWTKey() []byte {
	key := os.Getenv("JWT_SECRET_KEY")
	if key == "" {
		log.Fatal("JWT_SECRET_KEY environment variable is not set")
	}
	return []byte(key)
}

// GenerateJWT generates a new JWT token for the given user ID.
func GenerateJWT(userID primitive.ObjectID, role string, secretKey string) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID.Hex(),
		"role":    role,
		"exp":     time.Now().Add(24 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secretKey))
}

// ValidateJWT validates the JWT token and returns the user ID.
func ValidateJWT(tokenString string) (string, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return getJWTKey(), nil
	})
	if err != nil {
		log.Printf("Error parsing token: %v", err)
		return "", err
	}

	claims, ok := token.Claims.(*Claims)
	if ok && token.Valid {
		return claims.UserID, nil
	}

	return "", errors.New("invalid token claims")
}
