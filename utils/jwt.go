package utils

import (
	"log"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

var jwtKey = []byte(os.Getenv("JWT_SECRET_KEY"))

// Claims defines the structure of the JWT claims
type Claims struct {
	UserID string `json:"user_id"`
	jwt.RegisteredClaims
}

// GenerateJWT generates a new JWT token for the given user ID.
func GenerateJWT(userID string) (string, error) {
	expirationTime := time.Now().Add(24 * time.Hour) // Set token expiration time

	claims := &Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime), // Use the value directly
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtKey)
	if err != nil {
		log.Printf("Error generating token: %v", err)
		return "", err
	}
	return tokenString, nil
}

// ValidateJWT validates the JWT token and returns the user ID.
func ValidateJWT(tokenString string) (string, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return jwtKey, nil
	})
	if err != nil {
		log.Printf("Error validating token: %v", err)
		return "", err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return "", err
	}

	return claims.UserID, nil
}
