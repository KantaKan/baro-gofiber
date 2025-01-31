package services

import (
	"context"
	"gofiber-baro/config"
	"gofiber-baro/models"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

func GetSpreadsheetData() ([]models.SpreadsheetData, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pipeline := []bson.M{
		{
			"$lookup": bson.M{
				"from":         "reflections",
				"localField":   "_id",
				"foreignField": "user_id",
				"as":           "reflections",
			},
		},
	}

	log.Printf("Executing aggregation pipeline: %+v", pipeline) // Log the pipeline

	cursor, err := config.DB.Collection("users").Aggregate(ctx, pipeline)
	if err != nil {
		log.Printf("Error during aggregation: %v", err)
		return nil, err
	}
	defer cursor.Close(ctx)

	var spreadsheetData []models.SpreadsheetData

	for cursor.Next(ctx) {
		var user models.User
		if err := cursor.Decode(&user); err != nil {
			log.Printf("Error decoding user: %v", err)
			continue
		}

		log.Printf("Decoded user: %+v", user) // Log the decoded user

		// Check if required fields in user are present
		if user.ID.IsZero() || user.JSDNumber == "" || user.FirstName == "" || user.LastName == "" ||
			user.Email == "" || user.CohortNumber == 0 || user.Password == "" || user.Role == "" {
			log.Printf("Skipping user due to missing data: %+v", user)
			continue // Skip this user if any required field is missing
		}

		// Create a row for each reflection
		for _, reflection := range user.Reflections {
			// Check if required fields in reflection are present
			if reflection.Day == "" || reflection.CreatedAt.IsZero() || 
				reflection.ReflectionData.TechSessions.Happy == "" {
				log.Printf("Skipping reflection due to missing data: %+v", reflection)
				continue // Skip this reflection if any required field is missing
			}

			data := models.SpreadsheetData{
				ID:             user.ID.Hex(),
				JSDNumber:      user.JSDNumber,
				FirstName:      user.FirstName,
				LastName:       user.LastName,
				Email:          user.Email,
				CohortNumber:   string(user.CohortNumber), // Convert int to string
				Password:       user.Password,
				Role:           user.Role,
				ReflectionDay:  reflection.Day,
				ReflectionDate: reflection.CreatedAt.Format("2006-01-02"),
				TechHappy:      reflection.ReflectionData.TechSessions.Happy,
				TechImprove:    reflection.ReflectionData.TechSessions.Improve,
				NonTechHappy:   reflection.ReflectionData.NonTechSessions.Happy,
				NonTechImprove: reflection.ReflectionData.NonTechSessions.Improve,
				Barometer:      reflection.ReflectionData.Barometer,
			}
			spreadsheetData = append(spreadsheetData, data)
		}
	}

	if len(spreadsheetData) == 0 {
		log.Println("No valid spreadsheet data found") // Log the absence of valid data
		return []models.SpreadsheetData{}, nil // Return an empty slice instead of nil
	}

	return spreadsheetData, nil
}