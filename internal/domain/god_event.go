package domain

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type GodEvent struct {
	ID          primitive.ObjectID `bson:"_id" json:"id"`
	Preset      string             `bson:"preset" json:"preset"`
	Caption     string             `bson:"caption" json:"caption"`
	Cohort      int                `bson:"cohort" json:"cohort"`
	CastBy      primitive.ObjectID `bson:"cast_by" json:"cast_by"`
	Character   *BaroCharacter     `bson:"character,omitempty" json:"character,omitempty"`
	CreatedAt   time.Time          `bson:"created_at" json:"created_at"`
	ActiveUntil time.Time          `bson:"active_until" json:"active_until"`
}
