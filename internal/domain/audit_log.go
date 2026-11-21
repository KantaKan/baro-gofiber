package domain

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type AuditLog struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	ActorID   primitive.ObjectID `bson:"actor_id" json:"actor_id"`
	ActorName string             `bson:"-" json:"actor_name"`
	ActorRole string             `bson:"actor_role" json:"actor_role"`
	Cohort    int                `bson:"cohort" json:"cohort"`
	Method    string             `bson:"method" json:"method"`
	Path      string             `bson:"path" json:"path"`
	Route     string             `bson:"route" json:"route"`
	Status    int                `bson:"status" json:"status"`
	IPAddress string             `bson:"ip_address" json:"ip_address"`
	CreatedAt time.Time          `bson:"createdAt" json:"createdAt"`
}

type AuditLogRepository interface {
	Insert(ctx context.Context, entry *AuditLog) error
	FindPage(ctx context.Context, filter bson.M, page, limit int) ([]AuditLog, int64, error)
}
