package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Reflection struct for user reflections, where Day is used instead of ID
type Reflection struct {
	Day        string         `bson:"day" json:"day"` // Custom field for day, could be a date or unique identifier
	UserID     primitive.ObjectID `bson:"user_id" json:"user_id"` // Reference to the user
	Date       time.Time      `bson:"date" json:"date"`
	ReflectionData ReflectionContent `bson:"reflection" json:"reflection"` // Renamed for clarity
}

// ReflectionContent contains session data for the reflection
type ReflectionContent struct {
	TechSessions    SessionDetails `bson:"tech_sessions" json:"tech_sessions"`
	NonTechSessions SessionDetails `bson:"non_tech_sessions" json:"non_tech_sessions"`
	Barometer       string         `bson:"barometer" json:"barometer"`
}

// SessionDetails represents session details like "Happy" and "Improve" for each session
type SessionDetails struct {
	SessionName []string `bson:"session_name" json:"session_name"`
	Happy       string   `bson:"happy" json:"happy"`
	Improve     string   `bson:"improve" json:"improve"`
}

// User struct with reference to reflections
type User struct {
	ID            primitive.ObjectID `bson:"_id,omitempty" json:"_id"`        // MongoDB generated ID
	JSDNumber     string             `bson:"jsd_number" json:"jsd_number"`    // JSD Number เลขที่นักเรียน
	FirstName     string             `bson:"first_name" json:"first_name"`
	LastName      string             `bson:"last_name" json:"last_name"`
	Email         string             `bson:"email" json:"email"`
	CohortNumber  int                `bson:"cohort_number" json:"cohort_number"`
	Reflections   []Reflection       `bson:"reflections" json:"reflections"`  // This is where the reflections should be
	Password      string             `bson:"password,omitempty" json:"password,omitempty"`
	Role          string             `bson:"role" json:"role"`                // Add role field (admin/user)
}

type ReflectionWithUser struct {
	ID          primitive.ObjectID `json:"_id" bson:"_id"`
	UserID      primitive.ObjectID `json:"user_id" bson:"user_id"`
	FirstName   string            `json:"first_name" bson:"first_name"`
	LastName    string            `json:"last_name" bson:"last_name"`
	JSDNumber   string            `json:"jsd_number" bson:"jsd_number"`
	Date        time.Time         `json:"date" bson:"date"`
	Reflection  ReflectionData    `json:"reflection" bson:"reflection"`
}

// Existing ReflectionData structure
type ReflectionData struct {
	Barometer       string         `json:"barometer" bson:"barometer"`
	TechSessions    SessionData    `json:"tech_sessions" bson:"tech_sessions"`
	NonTechSessions SessionData    `json:"non_tech_sessions" bson:"non_tech_sessions"`
}

type SessionData struct {
	SessionName []string `json:"session_name" bson:"session_name"`
	Happy      string   `json:"happy" bson:"happy"`
	Improve    string   `json:"improve" bson:"improve"`
}