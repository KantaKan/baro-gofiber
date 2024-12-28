package utils

import (
	"log"

	"golang.org/x/crypto/bcrypt"
)

// HashPassword hashes the password using bcrypt.
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Error hashing password: %v", err)
		return "", err
	}
	return string(bytes), nil
}

// CheckPasswordHash compares the hashed password with the plaintext password.
func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
