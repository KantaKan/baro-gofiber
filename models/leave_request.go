package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type LeaveType string

const (
	LeaveTypeLate    LeaveType = "late"
	LeaveTypeHalfDay LeaveType = "half_day"
	LeaveTypeFullDay LeaveType = "full_day"
)

type LeaveRequestStatus string

const (
	LeaveStatusPending  LeaveRequestStatus = "pending"
	LeaveStatusApproved LeaveRequestStatus = "approved"
	LeaveStatusRejected LeaveRequestStatus = "rejected"
)

type LeaveRequest struct {
	ID             primitive.ObjectID  `bson:"_id,omitempty" json:"_id"`
	UserID         primitive.ObjectID  `bson:"user_id" json:"user_id"`
	JSDNumber      string              `bson:"jsd_number" json:"jsd_number"`
	FirstName      string              `bson:"first_name" json:"first_name"`
	LastName       string              `bson:"last_name" json:"last_name"`
	CohortNumber   int                 `bson:"cohort_number" json:"cohort_number"`
	Type           LeaveType           `bson:"type" json:"type"`
	Session        *AttendanceSession  `bson:"session,omitempty" json:"session,omitempty"`
	Date           string              `bson:"date" json:"date"`
	Reason         string              `bson:"reason" json:"reason"`
	Status         LeaveRequestStatus  `bson:"status" json:"status"`
	ReviewedBy     *primitive.ObjectID `bson:"reviewed_by,omitempty" json:"reviewed_by,omitempty"`
	ReviewedByName string              `bson:"reviewed_by_name,omitempty" json:"reviewed_by_name,omitempty"`
	ReviewedAt     *time.Time          `bson:"reviewed_at,omitempty" json:"reviewed_at,omitempty"`
	ReviewNotes    string              `bson:"review_notes,omitempty" json:"review_notes,omitempty"`
	CreatedAt      time.Time           `bson:"created_at" json:"created_at"`
	CreatedBy      string              `bson:"created_by,omitempty" json:"created_by,omitempty"`
	IsManualEntry  bool                `bson:"is_manual_entry" json:"is_manual_entry"`
}
