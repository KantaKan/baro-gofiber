package services

import (
	"context"
	"errors"
	"gofiber-baro/config"
	"gofiber-baro/models"
	"log"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// GetReflectionsByWeek retrieves all reflections for users, grouped by week and sorted by Panic Zone
func GetReflectionsByWeek() ([]models.WeeklyReflection, error) {
	// Ensure DB connection is not nil
	if config.DB == nil {
		return nil, errors.New("MongoDB connection is not initialized")
	}

	// Define start of the week (Monday) and end of the week (Sunday)
	now := time.Now()
	startOfWeek := time.Date(now.Year(), now.Month(), now.Day()-int(now.Weekday())+1, 0, 0, 0, 0, now.Location())
	endOfWeek := startOfWeek.AddDate(0, 0, 7)

	// Aggregation Pipeline to group by week and sort by Panic Zone first
	pipeline := mongo.Pipeline{
		// Match reflections that fall within the current week
		{
			{"$match", bson.M{
				"reflections.date": bson.M{
					"$gte": startOfWeek,
					"$lt":  endOfWeek,
				},
			}},
		},
		// Unwind reflections array to handle individual reflections
		{
			{"$unwind", "$reflections"},
		},
		// Match reflections where barometer is Panic Zone or Stretch Zone - Overwhelmed
		{
			{"$match", bson.M{
				"reflections.reflection.barometer": bson.M{
					"$in": []string{
						"Comfort Zone", 
						"Stretch Zone - Enjoying the Challenges",
						"Stretch Zone - Overwhelmed",
						"Panic Zone",
					},
				},
			}},
		},
		// Sort reflections based on barometer, with Panic Zone first
		{
			{"$sort", bson.M{
				"reflections.reflection.barometer": 1, // Sort by barometer value
			}},
		},
		// Group reflections by week, format them into weekly entries
		{
			{"$group", bson.M{
				"_id": bson.M{
					"week":   bson.M{"$week": "$reflections.date"}, // Group by the week number
					"year":   bson.M{"$year": "$reflections.date"}, // Group by year
				},
				"users": bson.M{"$push": "$reflections"}, // Push individual reflections to an array
			}},
		},
		// Sort by week and year for correct order
		{
			{"$sort", bson.M{
				"_id.year":  1,
				"_id.week":  1, // Ensure the weeks are sorted correctly
			}},
		},
	}

	// Perform aggregation query
	cursor, err := config.DB.Collection("users").Aggregate(context.Background(), pipeline)
	if err != nil {
		log.Printf("Error fetching reflections by week: %v", err)
		return nil, errors.New("Error fetching reflections by week")
	}
	defer cursor.Close(context.Background())

	// Parse the result
	var weeklyReflections []models.WeeklyReflection
	for cursor.Next(context.Background()) {
		var weekData models.WeeklyReflection
		if err := cursor.Decode(&weekData); err != nil {
			log.Printf("Error decoding weekly reflection data: %v", err)
			continue
		}
		weeklyReflections = append(weeklyReflections, weekData)
	}

	if err := cursor.Err(); err != nil {
		log.Printf("Cursor error: %v", err)
		return nil, errors.New("Error processing cursor data")
	}

	return weeklyReflections, nil
}

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
func GetUsersByBarometer(barometer string) ([]models.User, error) {
	if config.DB == nil {
		return nil, errors.New("MongoDB connection is not initialized")
	}

	// Define start of the week (Monday) and end of the week (Sunday)
	now := time.Now()
	startOfWeek := time.Date(now.Year(), now.Month(), now.Day()-int(now.Weekday())+1, 0, 0, 0, 0, now.Location())
	endOfWeek := startOfWeek.AddDate(0, 0, 7)

	// Aggregation Pipeline to filter by barometer and week
	pipeline := mongo.Pipeline{
		// Match reflections that fall within the current week and match the specified barometer
		{
			{"$match", bson.M{
				"reflections.date": bson.M{
					"$gte": startOfWeek,
					"$lt":  endOfWeek,
				},
				"reflections.reflection.barometer": barometer, // Filter by the barometer value
			}},
		},
		// Unwind reflections array to handle individual reflections
		{
			{"$unwind", "$reflections"},
		},
		// Group users and their reflections
		{
			{"$group", bson.M{
				"_id":    "$_id",       // Group by user ID
				"reflections": bson.M{"$push": "$reflections"},
			}},
		},
	}

	// Perform aggregation query
	cursor, err := config.DB.Collection("users").Aggregate(context.Background(), pipeline)
	if err != nil {
		log.Printf("Error fetching users with barometer '%s': %v", barometer, err)
		return nil, errors.New("Error fetching users with specified barometer")
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
func GenerateJWT(user models.User) (string, error) {
	// Load JWT_SECRET_KEY from environment variables
	jwtSecret := os.Getenv("JWT_SECRET_KEY")
	if jwtSecret == "" {
		return "", errors.New("JWT_SECRET_KEY not set in environment variables")
	}

	// Create JWT claims
	claims := jwt.MapClaims{
		"id":   user.ID,
		"role": user.Role, // Use the user's role in the token
		"exp":  time.Now().Add(time.Hour * 72).Unix(),
	}

	// Create the JWT token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Sign the token with the secret key
	tokenString, err := token.SignedString([]byte(jwtSecret))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}