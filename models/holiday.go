package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Holiday struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"_id"`
	Name        string             `bson:"name" json:"name"`
	StartDate   string             `bson:"start_date" json:"start_date"`
	EndDate     string             `bson:"end_date" json:"end_date"`
	Description string             `bson:"description,omitempty" json:"description,omitempty"`
	CreatedAt   time.Time          `bson:"created_at" json:"created_at"`
	CreatedBy   string             `bson:"created_by" json:"created_by"`
}
