package domain

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type TeacherGiftBox struct {
	ID              primitive.ObjectID   `bson:"_id" json:"id"`
	UserID          primitive.ObjectID   `bson:"user_id" json:"user_id"`
	MinimumRarity   string               `bson:"minimum_rarity" json:"minimum_rarity"`
	Message         string               `bson:"message" json:"message"`
	GrantedBy       primitive.ObjectID   `bson:"granted_by" json:"granted_by"`
	Status          string               `bson:"status" json:"status"`
	Reward          *CosmeticCatalogItem `bson:"reward,omitempty" json:"reward,omitempty"`
	Character       *BaroCharacter       `bson:"character,omitempty" json:"character,omitempty"`
	CreatedAt       time.Time            `bson:"created_at" json:"created_at"`
	OpenedAt        *time.Time           `bson:"opened_at,omitempty" json:"opened_at,omitempty"`
	GrantKey        string               `bson:"grant_key,omitempty" json:"-"`
	Source          string               `bson:"source,omitempty" json:"source,omitempty"`
	RewardPool      string               `bson:"reward_pool,omitempty" json:"reward_pool,omitempty"`
	TransferHistory []GiftBoxTransfer    `bson:"transfer_history,omitempty" json:"transfer_history,omitempty"`
}

type GiftBoxTransfer struct {
	FromID        primitive.ObjectID `bson:"from_id" json:"from_id"`
	FromName      string             `bson:"from_name" json:"from_name"`
	ToID          primitive.ObjectID `bson:"to_id" json:"to_id"`
	ToName        string             `bson:"to_name" json:"to_name"`
	TransferredAt time.Time          `bson:"transferred_at" json:"transferred_at"`
}
