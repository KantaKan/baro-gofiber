package services

import (
	"context"
	"gofiber-baro/config"
	"gofiber-baro/models"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
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
				"as":          "reflections",
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
		var user struct {
			ID           primitive.ObjectID `bson:"_id"`
			JSDNumber    string            `bson:"jsdNumber"`
			FirstName    string            `bson:"firstName"`
			LastName     string            `bson:"lastName"`
			Email        string            `bson:"email"`
			CohortNumber string            `bson:"cohortNumber"`
			Password     string            `bson:"password"`
			Role         string            `bson:"role"`
			Reflections  []models.Reflection `bson:"reflections"`
		}

		if err := cursor.Decode(&user); err != nil {
			log.Printf("Error decoding user: %v", err)
			continue
		}

		log.Printf("Decoded user: %+v", user) // Log the decoded user

		// Initialize default values for the spreadsheet data
		defaultData := models.SpreadsheetData{
			ID:             user.ID.Hex(),
			JSDNumber:      user.JSDNumber,
			FirstName:      user.FirstName,
			LastName:       user.LastName,
			Email:          user.Email,
			CohortNumber:   user.CohortNumber,
			Password:       user.Password,
			Role:          user.Role,
			ReflectionDay: "N/A", // Default value if no reflection
			ReflectionDate: "N/A", // Default value if no reflection
			TechHappy:     "N/A", // Default value if no reflection
			TechImprove:   "N/A", // Default value if no reflection
			NonTechHappy:  "N/A", // Default value if no reflection
			NonTechImprove: "N/A", // Default value if no reflection
			Barometer:     "N/A", // Default value if no reflection
		}

		// Check if reflections are present
		if len(user.Reflections) == 0 {
			log.Println("No reflections found for user:", user.ID.Hex())
			// Add the default data to the spreadsheetData
			spreadsheetData = append(spreadsheetData, defaultData)
			continue
		}

		// Create a row for each reflection
		for _, reflection := range user.Reflections {
			data := models.SpreadsheetData{
				ID:             user.ID.Hex(),
				JSDNumber:      user.JSDNumber,
				FirstName:      user.FirstName,
				LastName:       user.LastName,
				Email:          user.Email,
				CohortNumber:   user.CohortNumber,
				Password:       user.Password,
				Role:          user.Role,
				ReflectionDay: reflection.Day,
				ReflectionDate: reflection.CreatedAt.Format("2006-01-02"),
				TechHappy:     reflection.ReflectionData.TechSessions.Happy,
				TechImprove:   reflection.ReflectionData.TechSessions.Improve,
				NonTechHappy:  reflection.ReflectionData.NonTechSessions.Happy,
				NonTechImprove: reflection.ReflectionData.NonTechSessions.Improve,
				Barometer:     reflection.ReflectionData.Barometer,
			}
			spreadsheetData = append(spreadsheetData, data)
		}
	}

	if len(spreadsheetData) == 0 {
		log.Println("No spreadsheet data found")
	}

	return spreadsheetData, nil
}