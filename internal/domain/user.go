package domain

import (
	"encoding/json"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Badge struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	Type      string             `bson:"type" json:"type"`
	Name      string             `bson:"name" json:"name"`
	Emoji     string             `bson:"emoji" json:"emoji"`
	ImageUrl  string             `bson:"imageUrl,omitempty" json:"imageUrl,omitempty"`
	Color     string             `bson:"color,omitempty" json:"color,omitempty"`
	Style     string             `bson:"style,omitempty" json:"style,omitempty"`
	AwardedAt time.Time          `bson:"awardedAt" json:"awardedAt"`
}

type Reflection struct {
	ID             primitive.ObjectID `bson:"_id,omitempty" json:"_id"`
	Day            string             `bson:"day" json:"day"`
	UserID         primitive.ObjectID `bson:"user_id" json:"user_id"`
	Date           time.Time          `bson:"date" json:"date"`
	CreatedAt      time.Time          `bson:"createdAt" json:"createdAt"`
	ReflectionData ReflectionContent  `bson:"reflection" json:"reflection"`
	AdminFeedback  string             `bson:"admin_feedback,omitempty" json:"admin_feedback,omitempty"`
}

func (r Reflection) MarshalJSON() ([]byte, error) {
	type Alias Reflection
	thailandLoc, _ := time.LoadLocation("Asia/Bangkok")
	return json.Marshal(&struct {
		Date      string `json:"date"`
		CreatedAt string `json:"createdAt"`
		*Alias
	}{
		Date:      r.Date.In(thailandLoc).Format(time.RFC3339),
		CreatedAt: r.CreatedAt.In(thailandLoc).Format(time.RFC3339),
		Alias:     (*Alias)(&r),
	})
}

type ReflectionContent struct {
	TechSessions    SessionDetails `bson:"tech_sessions" json:"tech_sessions"`
	NonTechSessions SessionDetails `bson:"non_tech_sessions" json:"non_tech_sessions"`
	Barometer       string         `bson:"barometer" json:"barometer"`
}

type SessionDetails struct {
	SessionName []string `bson:"session_name" json:"session_name"`
	Happy       string   `bson:"happy" json:"happy"`
	Improve     string   `bson:"improve" json:"improve"`
}

type User struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"_id"`
	JSDNumber    string             `bson:"jsd_number" json:"jsd_number"`
	FirstName    string             `bson:"first_name" json:"first_name"`
	LastName     string             `bson:"last_name" json:"last_name"`
	Email        string             `bson:"email" json:"email"`
	CohortNumber int                `bson:"cohort_number" json:"cohort_number"`
	Reflections  []Reflection       `bson:"reflections" json:"reflections"`
	Password     string             `bson:"password,omitempty" json:"password,omitempty"`
	Role         string             `bson:"role" json:"role"`
	ProjectGroup string             `bson:"project_group" json:"project_group"`
	GenmateGroup string             `bson:"genmate_group" json:"genmate_group"`
	ZoomName     string             `bson:"zoom_name" json:"zoom_name"`
	Badges       []Badge            `bson:"badges,omitempty" json:"badges,omitempty"`
}

type ReflectionWithUser struct {
	ID         primitive.ObjectID `json:"id" bson:"_id"`
	FirstName  string             `bson:"first_name"`
	LastName   string             `bson:"last_name"`
	JsdNumber  string             `bson:"jsd_number"`
	Date       time.Time          `bson:"date"`
	Reflection ReflectionContent  `bson:"reflection"`
}

type EmojiZoneEntry struct {
	Date string `json:"date"`
	Zone string `json:"zone"`
}

type EmojiZoneTableData struct {
	ZoomName string           `json:"zoomname"`
	Entries  []EmojiZoneEntry `json:"entries"`
}

type WeeklySummary struct {
	WeekStartDate       string        `json:"week_start_date"`
	WeekEndDate         string        `json:"week_end_date"`
	StressedStudents    []StudentInfo `json:"stressed_students"`
	OverwhelmedStudents []StudentInfo `json:"overwhelmed_students"`
}

type StudentInfo struct {
	UserID    string    `bson:"user_id" json:"user_id"`
	FirstName string    `bson:"first_name" json:"first_name"`
	LastName  string    `bson:"last_name" json:"last_name"`
	ZoomName  string    `bson:"zoom_name" json:"zoom_name"`
	JsdNumber string    `bson:"jsd_number" json:"jsd_number"`
	Barometer string    `bson:"barometer" json:"barometer"`
	Date      time.Time `bson:"date" json:"date"`
}

type UserFilter struct {
	Cohort int
	Role   string
	Email  string
	Search string
}

type UserRepository interface {
	FindByID(ctx interface{}, id primitive.ObjectID) (*User, error)
	FindByEmail(ctx interface{}, email string) (*User, error)
	FindAll(ctx interface{}, filter UserFilter, opts interface{}) ([]User, int, error)
	Update(ctx interface{}, id primitive.ObjectID, update interface{}) error
	AddBadge(ctx interface{}, userID primitive.ObjectID, badge Badge) error
	UpdateReflectionFeedback(ctx interface{}, userID, reflectionID primitive.ObjectID, feedback string) error
	CreateReflection(ctx interface{}, userID primitive.ObjectID, reflection Reflection) error
}
