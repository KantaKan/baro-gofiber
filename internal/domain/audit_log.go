package domain

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type AuditLog struct {
	ID         primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Action     string             `bson:"action" json:"action"`         // e.g., "UPDATE_ATTENDANCE", "AWARD_BADGE"
	ActorID    primitive.ObjectID `bson:"actor_id" json:"actor_id"`     // Who did it
	ActorName  string             `bson:"actor_name" json:"actor_name"` // Cache name for easy display
	TargetID   primitive.ObjectID `bson:"target_id" json:"target_id"`   // Who/What it was done to
	TargetName string             `bson:"target_name" json:"target_name"`
	Details    string             `bson:"details" json:"details"`       // Human readable description
	IPAddress  string             `bson:"ip_address" json:"ip_address"`
	CreatedAt  time.Time          `bson:"createdAt" json:"createdAt"`
}

type AuditLogRepository interface {
	Insert(ctx context.Context, log *AuditLog) error
	FindAll(ctx context.Context, filter interface{}, limit int64) ([]AuditLog, error)
}
