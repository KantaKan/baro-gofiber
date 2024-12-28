package services

import (
	"context"
	"errors"
	"gofiber-baro/config"
	"gofiber-baro/models"
	"gofiber-baro/utils"
	"log"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var userCollection *mongo.Collection


func InitUserService() {
	if config.DB != nil {
		userCollection = config.DB.Collection("users")
	} else {
		log.Fatal("Failed to initialize user service: database connection is nil")
	}
}

// CreateUser handles user registration logic
func CreateUser(user models.User) (*models.User, error) {
	// Ensure DB connection is not nil
	if config.DB == nil {
		return nil, errors.New("MongoDB connection is not initialized")
	}

	// Check if user already exists
	filter := bson.M{"email": user.Email}
	var existingUser models.User
	err := userCollection.FindOne(context.Background(), filter).Decode(&existingUser)
	if err == nil {
		return nil, errors.New("user already exists")
	}

	// Hash the password
	hashedPassword, err := utils.HashPassword(user.Password)
	if err != nil {
		return nil, errors.New("failed to hash password")
	}
	user.Password = hashedPassword

	// Insert new user into the database
	result, err := userCollection.InsertOne(context.Background(), user)
	if err != nil {
		return nil, errors.New("failed to create user")
	}

	// Retrieve inserted user ID
	user.ID = result.InsertedID.(primitive.ObjectID)

	return &user, nil
}

// AuthenticateUser validates credentials and generates a JWT token
func AuthenticateUser(email, password string) (string, error) {
	// Ensure DB connection is not nil
	if config.DB == nil {
		return "", errors.New("MongoDB connection is not initialized")
	}

	// Find the user by email
	filter := bson.M{"email": email}
	var user models.User
	err := userCollection.FindOne(context.Background(), filter).Decode(&user)
	if err != nil {
		return "", errors.New("invalid email or password")
	}

	// Check the password
	if !utils.CheckPasswordHash(password, user.Password) {
		return "", errors.New("invalid email or password")
	}

	// Generate a JWT token
	token, err := utils.GenerateJWT(user.ID.Hex())
	if err != nil {
		return "", errors.New("failed to generate token")
	}

	return token, nil
}

// GetUserByID retrieves a user by their ID
func GetUserByID(userID string) (*models.User, error) {
	// Ensure DB connection is not nil
	if config.DB == nil {
		return nil, errors.New("MongoDB connection is not initialized")
	}

	// Convert userID to ObjectID
	objectID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, errors.New("invalid user ID")
	}

	// Find the user in the database
	filter := bson.M{"_id": objectID}
	var user models.User
	err = userCollection.FindOne(context.Background(), filter).Decode(&user)
	if err != nil {
		return nil, errors.New("user not found")
	}

	return &user, nil
}

// CreateReflection creates a reflection for a user and pushes it into the 'Reflections' array
func CreateReflection(userID primitive.ObjectID, reflection models.Reflection) (*models.Reflection, error) {
	// Ensure DB connection is not nil
	if config.DB == nil {
		return nil, errors.New("MongoDB connection is not initialized")
	}

	// Check if user exists
	filter := bson.M{"_id": userID}
	var user models.User
	err := userCollection.FindOne(context.Background(), filter).Decode(&user)
	if err != nil {
		return nil, errors.New("user not found")
	}

	// Initialize reflections array if not present
	if user.Reflections == nil {
		user.Reflections = []models.Reflection{} // Initialize as empty array
	}

	// Add new reflection to the user's reflections
	user.Reflections = append(user.Reflections, reflection)

	// Update user document with new reflection
	update := bson.M{
		"$set": bson.M{
			"reflections": user.Reflections,
		},
	}

	// Use upsert option to insert if no existing reflections field is found
	_, err = userCollection.UpdateOne(context.Background(), filter, update, options.Update().SetUpsert(true))
	if err != nil {
		return nil, errors.New("failed to update user reflections")
	}

	// Return the new reflection object
	return &reflection, nil
}

// GetReflections retrieves all reflections for a user
func GetReflections(userID primitive.ObjectID) ([]models.Reflection, error) {
	// Ensure DB connection is not nil
	if config.DB == nil {
		return nil, errors.New("MongoDB connection is not initialized")
	}

	// Find the user and retrieve reflections
	filter := bson.M{"_id": userID}
	var user models.User
	err := userCollection.FindOne(context.Background(), filter).Decode(&user)
	if err != nil {
		return nil, errors.New("user not found")
	}

	// Return the reflections
	return user.Reflections, nil
}
