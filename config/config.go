package config

import (
	"context"
	"fmt"
	"log"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

var DB *mongo.Database
var AttendanceCodesCollection *mongo.Collection
var AttendanceRecordsCollection *mongo.Collection
var LeaveRequestsCollection *mongo.Collection
var HolidaysCollection *mongo.Collection

func InitializeDB(mongoURI, databaseName string) error {
	log.Printf("Debug - MongoDB URI: %s", mongoURI)

	if mongoURI == "" || databaseName == "" {
		return fmt.Errorf("MONGO_URI or DATABASE_NAME not provided")
	}

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
	HolidaysCollection = DB.Collection("holidays")

	// Create Indexes
	if err := createIndexes(context.Background()); err != nil {
		log.Printf("Warning: failed to create indexes: %v", err)
	}

	return nil
}

func createIndexes(ctx context.Context) error {
	// 1. Users Collection Indexes
	usersColl := DB.Collection("users")
	userIndexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "email", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{
				{Key: "cohort_number", Value: 1},
				{Key: "first_name", Value: 1},
			},
		},
		{
			Keys: bson.D{{Key: "jsd_number", Value: 1}},
		},
	}
	_, err := usersColl.Indexes().CreateMany(ctx, userIndexes)
	if err != nil {
		return err
	}

	// 2. Attendance Records Indexes
	attendanceIndexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "user_id", Value: 1},
				{Key: "date", Value: 1},
				{Key: "session", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{
				{Key: "cohort_number", Value: 1},
				{Key: "date", Value: 1},
			},
		},
	}
	_, err = AttendanceRecordsCollection.Indexes().CreateMany(ctx, attendanceIndexes)
	if err != nil {
		return err
	}

	// 3. Talk Board Indexes
	boardColl := DB.Collection("talk_board")
	boardIndexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "cohort", Value: 1},
				{Key: "createdAt", Value: -1},
			},
		},
		{
			Keys: bson.D{{Key: "user_id", Value: 1}},
		},
	}
	_, err = boardColl.Indexes().CreateMany(ctx, boardIndexes)
	if err != nil {
		return err
	}

	// 4. Attendance Codes (TTL Index for auto-deletion of expired codes)
	codeIndexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "expires_at", Value: 1}},
			Options: options.Index().SetExpireAfterSeconds(0),
		},
		{
			Keys: bson.D{
				{Key: "cohort_number", Value: 1},
				{Key: "session", Value: 1},
				{Key: "is_active", Value: 1},
			},
		},
	}
	_, err = AttendanceCodesCollection.Indexes().CreateMany(ctx, codeIndexes)
	if err != nil {
		return err
	}

	log.Println("Database indexes synchronized successfully")
	return nil
}
