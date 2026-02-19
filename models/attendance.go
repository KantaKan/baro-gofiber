package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type AttendanceSession string

const (
	SessionMorning   AttendanceSession = "morning"
	SessionAfternoon AttendanceSession = "afternoon"
)

type AttendanceStatus string

const (
	StatusPresent       AttendanceStatus = "present"
	StatusLate          AttendanceStatus = "late"
	StatusAbsent        AttendanceStatus = "absent"
	StatusLateExcused   AttendanceStatus = "late_excused"
	StatusAbsentExcused AttendanceStatus = "absent_excused"
)

type MarkedBy string

const (
	MarkedBySelf  MarkedBy = "self"
	MarkedByAdmin MarkedBy = "admin"
)

type AttendanceCode struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"_id"`
	Code         string             `bson:"code" json:"code"`
	CohortNumber int                `bson:"cohort_number" json:"cohort_number"`
	Session      AttendanceSession  `bson:"session" json:"session"`
	GeneratedAt  time.Time          `bson:"generated_at" json:"generated_at"`
	ExpiresAt    time.Time          `bson:"expires_at" json:"expires_at"`
	IsActive     bool               `bson:"is_active" json:"is_active"`
	GeneratedBy  string             `bson:"generated_by" json:"generated_by"`
}

type AttendanceRecord struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"_id"`
	UserID       primitive.ObjectID `bson:"user_id" json:"user_id"`
	JSDNumber    string             `bson:"jsd_number" json:"jsd_number"`
	FirstName    string             `bson:"first_name" json:"first_name"`
	LastName     string             `bson:"last_name" json:"last_name"`
	CohortNumber int                `bson:"cohort_number" json:"cohort_number"`
	Date         string             `bson:"date" json:"date"`
	Session      AttendanceSession  `bson:"session" json:"session"`
	Status       AttendanceStatus   `bson:"status" json:"status"`
	MarkedBy     MarkedBy           `bson:"marked_by" json:"marked_by"`
	MarkedByUser string             `bson:"marked_by_user,omitempty" json:"marked_by_user,omitempty"`
	SubmittedAt  time.Time          `bson:"submitted_at" json:"submitted_at"`
	Locked       bool               `bson:"locked" json:"locked"`
	IPAddress    string             `bson:"ip_address,omitempty" json:"ip_address,omitempty"`
	Deleted      bool               `bson:"deleted" json:"deleted"`
	DeletedAt    *time.Time         `bson:"deleted_at,omitempty" json:"deleted_at,omitempty"`
	DeletedBy    string             `bson:"deleted_by,omitempty" json:"deleted_by,omitempty"`
}

type AttendanceStats struct {
	UserID        primitive.ObjectID `json:"user_id"`
	JSDNumber     string             `json:"jsd_number"`
	FirstName     string             `json:"first_name"`
	LastName      string             `json:"last_name"`
	CohortNumber  int                `json:"cohort_number"`
	Present       int                `json:"present"`
	Late          int                `json:"late"`
	Absent        int                `json:"absent"`
	LateExcused   int                `json:"late_excused"`
	AbsentExcused int                `json:"absent_excused"`
	TotalDays     int                `json:"total_days"`
	WarningLevel  string             `json:"warning_level"`
}

type TodayAttendanceOverview struct {
	Session        AttendanceSession      `json:"session"`
	Code           string                 `json:"code,omitempty"`
	ExpiresAt      time.Time              `json:"expires_at,omitempty"`
	SubmittedCount int                    `json:"submitted_count"`
	Students       []StudentAttendanceRow `json:"students"`
}

type StudentAttendanceRow struct {
	UserID    primitive.ObjectID `json:"user_id"`
	JSDNumber string             `json:"jsd_number"`
	FirstName string             `json:"first_name"`
	LastName  string             `json:"last_name"`
	Morning   string             `json:"morning"`
	Afternoon string             `json:"afternoon"`
}
