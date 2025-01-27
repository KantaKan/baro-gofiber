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
	CreatedAt  time.Time      `bson:"createdAt" json:"createdAt"` // Added for daily reflection limit
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

type ReflectionWithUser struct {
	ID         primitive.ObjectID `json:"id" bson:"_id"`
    FirstName  string    `bson:"first_name"`
    LastName   string    `bson:"last_name"`
    JsdNumber  string    `bson:"jsd_number"`
    Date       time.Time `bson:"date"`
    Reflection struct {
        TechSessions struct {
            SessionName []string `bson:"session_name"`
            Happy       string   `bson:"happy"`
            Improve     string   `bson:"improve"`
        } `bson:"tech_sessions"`
        NonTechSessions struct {
            SessionName []string `bson:"session_name"`
            Happy       string   `bson:"happy"`
            Improve     string   `bson:"improve"`
        } `bson:"non_tech_sessions"`
        Barometer string `bson:"barometer"`
    } `bson:"reflection"`
}

// BarometerData represents daily barometer statistics
type BarometerData struct {
    Date                             string `json:"date"`
    ComfortZone                      int    `json:"Comfort Zone"`
    PanicZone                        int    `json:"Panic Zone"`
    StretchZoneEnjoyingTheChallenges int    `json:"Stretch Zone - Enjoying the Challenges"`
    StretchZoneOverwhelmed           int    `json:"Stretch Zone - Overwhelmed"`
}


