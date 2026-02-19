package config

import (
	"context"
	"fmt"
	"log" // Add this import
	"strings"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

var DB *mongo.Database
var AttendanceCodesCollection *mongo.Collection
var AttendanceRecordsCollection *mongo.Collection
var LeaveRequestsCollection *mongo.Collection

func InitializeDB(mongoURI, databaseName string) error {
	// Add debug logging
	log.Printf("Debug - MongoDB URI: %s", mongoURI)

	if mongoURI == "" || databaseName == "" {
		return fmt.Errorf("MONGO_URI or DATABASE_NAME not provided")
	}

	// Try trimming any whitespace or quotes that might have been picked up
	mongoURI = strings.TrimSpace(mongoURI)
	mongoURI = strings.Trim(mongoURI, "\"'")

	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI(mongoURI))
	if err != nil {
		return fmt.Errorf("failed to connect to MongoDB: %v", err)
	}

	if err := client.Ping(context.Background(), readpref.Primary()); err != nil {
		return fmt.Errorf("failed to ping MongoDB: %v", err)
	}

	DB = client.Database(databaseName)

	AttendanceCodesCollection = DB.Collection("attendance_codes")
	AttendanceRecordsCollection = DB.Collection("attendance_records")
	LeaveRequestsCollection = DB.Collection("leave_requests")

	return nil
}
