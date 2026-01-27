package services

import (
	"context"
	"errors"
	"gofiber-baro/config"
	"gofiber-baro/models"
	"gofiber-baro/utils"
	"log"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"
)

var userCollection *mongo.Collection


func InitUserService() {
	if config.DB != nil {
		userCollection = config.DB.Collection("users")
	} else {
		log.Println("Warning: Failed to initialize user service: database connection is nil")
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

	// Set default role to "learner"
	user.Role = "learner"

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
func AuthenticateUser(email, password string) (string, string, string, error) {
	// Find the user in the database
	var user models.User
	err := config.DB.Collection("users").FindOne(context.Background(), bson.M{"email": email}).Decode(&user)
	if err != nil {
		log.Printf("Error retrieving user: %v", err)
		return "", "", "", errors.New("invalid credentials")
	}

	// Compare the provided password with the stored hashed password
	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if err != nil {
		log.Printf("Password comparison failed: %v", err)
		return "", "", "", errors.New("invalid credentials")
	}

	// Generate JWT token
	claims := jwt.MapClaims{
		"user_id": user.ID.Hex(),
		"role":  user.Role, // Add the user's role to the token
		"exp":   time.Now().Add(time.Hour * 24 * 30).Unix(), // Token expires in 30 days
	}

	// Fetch the JWT secret from environment variables
	jwtSecret := os.Getenv("JWT_SECRET_KEY")
	if jwtSecret == "" {
		return "", "", "", errors.New("missing JWT secret key")
	}

	// Create the token using the claims and sign it with HMAC
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Sign the token
	tokenString, err := token.SignedString([]byte(jwtSecret))
	if err != nil {
		log.Printf("Error signing token: %v", err)
		return "", "", "", errors.New("could not generate token")
	}

	// Return the generated token and the user's role
	return tokenString, user.Role, user.ID.Hex(), nil
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

	// Check if user has already created a reflection today
	today := time.Now().Truncate(24 * time.Hour)
	for _, r := range user.Reflections {
		reflectionDate := r.CreatedAt.Truncate(24 * time.Hour)
		if reflectionDate.Equal(today) {
			return nil, errors.New("user has already created a reflection today")
		}
	}

	// Set creation timestamp
	reflection.CreatedAt = time.Now()

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

// UserWithReflections struct to hold user data and reflections
type UserWithReflections struct {
    User        models.User          `json:"user"`
    Reflections []models.Reflection  `json:"reflections"`
}

func GetUserWithReflections(userID primitive.ObjectID) (*UserWithReflections, error) {
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

    // Return the user and reflections
    return &UserWithReflections{
        User:        user,
        Reflections: user.Reflections,
    }, nil
}
