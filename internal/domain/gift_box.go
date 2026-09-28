package domain

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type TeacherGiftBox struct {
	ID            primitive.ObjectID   `bson:"_id" json:"id"`
	UserID        primitive.ObjectID   `bson:"user_id" json:"user_id"`
	MinimumRarity string               `bson:"minimum_rarity" json:"minimum_rarity"`
	Message       string               `bson:"message" json:"message"`
	GrantedBy     primitive.ObjectID   `bson:"granted_by" json:"granted_by"`
	Status        string               `bson:"status" json:"status"`
	Reward        *CosmeticCatalogItem `bson:"reward,omitempty" json:"reward,omitempty"`
	CreatedAt     time.Time            `bson:"created_at" json:"created_at"`
	OpenedAt      *time.Time           `bson:"opened_at,omitempty" json:"opened_at,omitempty"`
	GrantKey      string               `bson:"grant_key,omitempty" json:"-"`
}
