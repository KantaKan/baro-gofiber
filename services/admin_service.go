package services

import (
	"context"
	"errors"
	"gofiber-baro/config"
	"gofiber-baro/models"
	"log"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
)

// GetAllUsers fetches all users in the database
func GetAllUsers() ([]models.User, error) {
	if config.DB == nil {
		return nil, errors.New("MongoDB connection is not initialized")
	}

	// Fetch all users from the database
	cursor, err := config.DB.Collection("users").Find(context.Background(), bson.M{})
	if err != nil {
		log.Printf("Error fetching users: %v", err)
		return nil, errors.New("Error fetching users")
	}
	defer cursor.Close(context.Background())

	var users []models.User
	for cursor.Next(context.Background()) {
		var user models.User
		if err := cursor.Decode(&user); err != nil {
			log.Printf("Error decoding user: %v", err)
			continue
		}
		users = append(users, user)
	}

	if err := cursor.Err(); err != nil {
		log.Printf("Cursor error: %v", err)
		return nil, errors.New("Error processing cursor data")
	}

	return users, nil
}

// GetUserBarometerData fetches and transforms the user reflection data into the 4 zone counts for the given day
func GetUserBarometerData() (map[string]int, error) {
	users, err := GetAllUsers()
	if err != nil {
		return nil, err
	}

	// Initialize counters for each zone
	zoneCounts := map[string]int{
		"Comfort Zone":                     0,
		"Stretch Zone - Enjoying the Challenges": 0,
		"Stretch Zone - Overwhelmed":       0,
		"Panic Zone":                       0,
	}

	for _, user := range users {
		// Iterate over each reflection for the user
		for _, reflection := range user.Reflections {
			// Get the barometer zone from the reflection data
			barometer := reflection.ReflectionData.Barometer

			// Normalize the barometer string to lowercase for case-insensitive comparison
			switch strings.ToLower(barometer) {
			case "comfort zone":
				zoneCounts["Comfort Zone"]++
			case "stretch zone- enjoying the challenges":
				zoneCounts["Stretch Zone - Enjoying the Challenges"]++
			case "stretch zone - overwhelmed":
				zoneCounts["Stretch Zone - Overwhelmed"]++
			case "panic zone":
				zoneCounts["Panic Zone"]++
			}
		}
	}

	// Return the counts of users in each zone for the current date
	return zoneCounts, nil
}
