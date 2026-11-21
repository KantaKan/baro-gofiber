package domain

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// CupAward is one group-points award in the weekly GenMate Cup.
type CupAward struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Cohort    int                `bson:"cohort" json:"cohort"`
	Group     string             `bson:"group" json:"group"`
	Category  string             `bson:"category" json:"category"` // coding | soft | showing_up | helping
	Points    int                `bson:"points" json:"points"`
	Reason    string             `bson:"reason" json:"reason"`
	Day       string             `bson:"day" json:"day"`   // YYYY-MM-DD (Bangkok)
	Week      string             `bson:"week" json:"week"` // Monday YYYY-MM-DD
	AwardedBy primitive.ObjectID `bson:"awarded_by,omitempty" json:"-"`
	Key       string             `bson:"key,omitempty" json:"-"` // idempotency for server awards
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
}

// CupWin records a week's winning group; Members is frozen so medals survive group changes.
type CupWin struct {
	ID        primitive.ObjectID   `bson:"_id,omitempty" json:"-"`
	Cohort    int                  `bson:"cohort" json:"cohort"`
	Week      string               `bson:"week" json:"week"`
	WeekNo    int                  `bson:"week_no" json:"week_no"`
	Group     string               `bson:"group" json:"group"`
	Points    int                  `bson:"points" json:"points"`
	Members   []primitive.ObjectID `bson:"members" json:"-"`
	CreatedAt time.Time            `bson:"created_at" json:"created_at"`
}
