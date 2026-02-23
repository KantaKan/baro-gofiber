package domain

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
	PresentDays   int                `json:"present_days"`
	AbsentDays    int                `json:"absent_days"`
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
	UserID            primitive.ObjectID `json:"user_id"`
	JSDNumber         string             `json:"jsd_number"`
	FirstName         string             `json:"first_name"`
	LastName          string             `json:"last_name"`
	Morning           string             `json:"morning"`
	Afternoon         string             `json:"afternoon"`
	MorningRecordID   string             `json:"morning_record_id,omitempty"`
	AfternoonRecordID string             `json:"afternoon_record_id,omitempty"`
}

type AttendanceRecordFilter struct {
	Cohort     int
	Date       string
	Session    AttendanceSession
	UserID     primitive.ObjectID
	NotDeleted bool
}

type AttendanceCodeFilter struct {
	Cohort  int
	Session AttendanceSession
	Active  bool
}

type AttendanceRepository interface {
	InsertRecord(ctx interface{}, record *AttendanceRecord) error
	FindRecord(ctx interface{}, filter AttendanceRecordFilter) (*AttendanceRecord, error)
	FindRecords(ctx interface{}, filter AttendanceRecordFilter, opts interface{}) ([]AttendanceRecord, error)
	UpdateRecord(ctx interface{}, id primitive.ObjectID, update interface{}) error
	UpdateRecords(ctx interface{}, filter AttendanceRecordFilter, update interface{}) error
	DeleteRecord(ctx interface{}, id primitive.ObjectID, deletedBy string) error
	CountRecords(ctx interface{}, filter AttendanceRecordFilter) (int64, error)
	AggregateStats(ctx interface{}, pipeline interface{}) ([]AttendanceStats, error)
	AggregateDailyStats(ctx interface{}, pipeline interface{}) ([]map[string]interface{}, error)
}

type AttendanceCodeRepository interface {
	InsertCode(ctx interface{}, code *AttendanceCode) error
	FindActiveCode(ctx interface{}, cohort int, session AttendanceSession) (*AttendanceCode, error)
	DeactivateOldCodes(ctx interface{}, cohort int, session AttendanceSession) error
}
