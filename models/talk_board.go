package models

import (
	"time"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Post represents a single entry in the talk board
type Post struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	UserID    primitive.ObjectID `bson:"userId" json:"userId"`
	ZoomName  string             `bson:"zoomName" json:"zoomName"`
	Cohort    int                `bson:"cohort" json:"cohort"`
	Content   string             `bson:"content" json:"content"`
	Reactions []Reaction         `bson:"reactions" json:"reactions"`
	Comments  []Comment          `bson:"comments" json:"comments"`
	CreatedAt time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time          `bson:"updatedAt" json:"updatedAt"`
}

// Comment represents a reply to a post
type Comment struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	UserID    primitive.ObjectID `bson:"userId" json:"userId"`
	ZoomName  string             `bson:"zoomName" json:"zoomName"`
	Cohort    int                `bson:"cohort" json:"cohort"`
	Content   string             `bson:"content" json:"content"`
	Reactions []Reaction         `bson:"reactions" json:"reactions"`
	CreatedAt time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time          `bson:"updatedAt" json:"updatedAt"`
}

// Reaction represents a reaction to a post or comment
type Reaction struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	UserID    primitive.ObjectID `bson:"userId" json:"userId"`
	Type      string             `bson:"type" json:"type"` // "emoji" or "image"
	Value     string             `bson:"value" json:"value"` // emoji character or image URL
	CreatedAt time.Time          `bson:"createdAt" json:"createdAt"`
}
