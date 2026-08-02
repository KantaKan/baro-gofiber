package config

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
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
	log.Println("Connecting to MongoDB...")

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
	// The (user_id, date, session) uniqueness must only apply to LIVE records so that a
	// learner can re-check-in after an admin clears (soft-deletes) their record. We use a
	// partial unique index. A legacy full-unique index from an older deployment is
	// incompatible with the partial filter expression, so drop it by name first if present.
	// NOTE: dropping/recreating an index never touches document data.
	legacyUniqueName := "user_id_1_date_1_session_1"
	_, _ = AttendanceRecordsCollection.Indexes().DropOne(ctx, legacyUniqueName)

	cleanupAttendanceDuplicates(ctx)

	attendanceIndexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "user_id", Value: 1},
				{Key: "date", Value: 1},
				{Key: "session", Value: 1},
			},
			Options: options.Index().
				SetUnique(true).
				SetPartialFilterExpression(bson.M{"deleted": false}), // ponytail: $ne unsupported in partial index
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

	// 5. Stamps Indexes
	stampsColl := DB.Collection("stamps")
	stampIndexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "cohortNumber", Value: 1},
				{Key: "createdAt", Value: 1},
			},
		},
	}
	_, err = stampsColl.Indexes().CreateMany(ctx, stampIndexes)
	if err != nil {
		return err
	}

	// 6. Cohorts Indexes
	cohortsColl := DB.Collection("cohorts")
	cohortIndexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "cohort_number", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "lock_at", Value: 1}},
		},
	}
	_, err = cohortsColl.Indexes().CreateMany(ctx, cohortIndexes)
	if err != nil {
		return err
	}

	log.Println("Database indexes synchronized successfully")
	return nil
}

func cleanupAttendanceDuplicates(ctx context.Context) {
	pipeline := mongo.Pipeline{
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: bson.D{
				{Key: "user_id", Value: "$user_id"},
				{Key: "date", Value: "$date"},
				{Key: "session", Value: "$session"},
			}},
			{Key: "count", Value: bson.D{{Key: "$sum", Value: 1}}},
			{Key: "ids", Value: bson.D{{Key: "$push", Value: "$_id"}}},
		}}},
		{{Key: "$match", Value: bson.D{{Key: "count", Value: bson.D{{Key: "$gt", Value: 1}}}}}},
	}
	cursor, err := AttendanceRecordsCollection.Aggregate(ctx, pipeline)
	if err != nil {
		log.Printf("Warning: failed to query attendance duplicates: %v", err)
		return
	}
	defer cursor.Close(ctx)

	type dupGroup struct {
		ID struct {
			UserID  primitive.ObjectID `bson:"user_id"`
			Date    string             `bson:"date"`
			Session string             `bson:"session"`
		} `bson:"_id"`
		Count int                  `bson:"count"`
		IDs   []primitive.ObjectID `bson:"ids"`
	}

	var groups []dupGroup
	if err := cursor.All(ctx, &groups); err != nil {
		log.Printf("Warning: failed to decode attendance duplicates: %v", err)
		return
	}

	now := time.Now()
	for _, g := range groups {
		// keep the first one (oldest _id), soft-delete the rest
		for _, id := range g.IDs[1:] {
			_, err := AttendanceRecordsCollection.UpdateOne(ctx,
				bson.M{"_id": id},
				bson.M{"$set": bson.M{"deleted": true, "deleted_at": now, "deleted_by": "system"}},
			)
			if err != nil {
				log.Printf("Warning: failed to clean duplicate attendance %s: %v", id.Hex(), err)
			} else {
				log.Printf("Cleaned duplicate attendance: user=%s date=%s session=%s", g.ID.UserID.Hex(), g.ID.Date, g.ID.Session)
			}
		}
	}

	if len(groups) > 0 {
		log.Printf("Cleaned %d duplicate attendance groups", len(groups))
	}
}
