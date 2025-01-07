package services

import (
	"context"
	"errors"
	"gofiber-baro/config"
	"gofiber-baro/models"
	"log"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// GetAllUsers fetches all users in the database.
func GetAllUsers() ([]models.User, error) {
	if config.DB == nil {
		return nil, errors.New("MongoDB connection is not initialized")
	}

	// Fetch all users from the database.
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

// GetAllReflections fetches all reflections from all users in the database.
func GetAllReflections() ([]models.Reflection, error) {
	if config.DB == nil {
		return nil, errors.New("MongoDB connection is not initialized")
	}

	// Fetch all users to extract their reflections.
	users, err := GetAllUsers()
	if err != nil {
		log.Printf("Error fetching users: %v", err)
		return nil, errors.New("Error fetching users for reflections")
	}

	var reflections []models.Reflection
	for _, user := range users {
		// Append all reflections from the user.
		reflections = append(reflections, user.Reflections...)
	}

	return reflections, nil
}

// GetUserBarometerData fetches and transforms user reflection data into the 4 zone counts.
func GetUserBarometerData() (map[string]int, error) {
	users, err := GetAllUsers()
	if err != nil {
		return nil, err
	}

	// Initialize counters for each zone.
	zoneCounts := map[string]int{
		"Comfort Zone":                          0,
		"Stretch Zone - Enjoying the Challenges": 0,
		"Stretch Zone - Overwhelmed":            0,
		"Panic Zone":                            0,
	}

	for _, user := range users {
		// Iterate over each reflection for the user.
		for _, reflection := range user.Reflections {
			// Get the barometer zone from the reflection data.
			barometer := reflection.ReflectionData.Barometer

			// Normalize the barometer string to lowercase for case-insensitive comparison.
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

	return zoneCounts, nil
}

func GetAllReflectionsWithUserInfo(page int, limit int) ([]models.ReflectionWithUser, int, error) {
    offset := (page - 1) * limit

    pipeline := []bson.M{
        {
            "$unwind": "$reflections",
        },
        {
            "$project": bson.M{
                "first_name":  "$first_name",
                "last_name":   "$last_name",
                "jsd_number":  "$jsd_number",
                "date":        "$reflections.date",
                "reflection":  "$reflections.reflection",
            },
        },
        {
            "$sort": bson.M{
                "date": -1,
            },
        },
        {
            "$skip": offset,
        },
        {
            "$limit": limit,
        },
    }

    log.Printf("Executing pipeline: %+v", pipeline)

    // Execute the aggregation pipeline
    cursor, err := config.DB.Collection("users").Aggregate(context.Background(), pipeline, options.Aggregate())
    if err != nil {
        log.Printf("Error executing aggregation: %v", err)
        return nil, 0, errors.New("error fetching reflections with user info")
    }
    defer cursor.Close(context.Background())

    var reflectionsWithUser []models.ReflectionWithUser
    if err := cursor.All(context.Background(), &reflectionsWithUser); err != nil {
        log.Printf("Error decoding reflections: %v", err)
        return nil, 0, errors.New("error processing reflection data")
    }

    log.Printf("Reflections with user info: %+v", reflectionsWithUser)

    // Get the total count of reflections
    countPipeline := []bson.M{
        {
            "$unwind": "$reflections",
        },
        {
            "$count": "total",
        },
    }

    log.Printf("Executing count pipeline: %+v", countPipeline)

    countCursor, err := config.DB.Collection("users").Aggregate(context.Background(), countPipeline, options.Aggregate())
    if err != nil {
        log.Printf("Error executing count aggregation: %v", err)
        return nil, 0, errors.New("error fetching total count of reflections")
    }
    defer countCursor.Close(context.Background())

    var countResult []bson.M
    if err := countCursor.All(context.Background(), &countResult); err != nil {
        log.Printf("Error decoding count result: %v", err)
        return nil, 0, errors.New("error processing count data")
    }

    total := 0
    if len(countResult) > 0 {
        total = int(countResult[0]["total"].(int32))
    }

    log.Printf("Total reflections count: %d", total)

    return reflectionsWithUser, total, nil
}