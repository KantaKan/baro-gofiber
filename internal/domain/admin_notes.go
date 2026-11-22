package domain

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AdminNote is a private TA note about a learner (never shown to learners).
type AdminNote struct {
	ID         primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID     primitive.ObjectID `bson:"user_id" json:"user_id"`
	AuthorID   primitive.ObjectID `bson:"author_id" json:"author_id"`
	AuthorName string             `bson:"author_name" json:"author_name"`
	Text       string             `bson:"text" json:"text"`
	CreatedAt  time.Time          `bson:"created_at" json:"created_at"`
}

// SupportItem is one step of a learner's support plan.
type SupportItem struct {
	ID         primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID     primitive.ObjectID `bson:"user_id" json:"user_id"`
	Text       string             `bson:"text" json:"text"`
	Done       bool               `bson:"done" json:"done"`
	DoneAt     *time.Time         `bson:"done_at,omitempty" json:"done_at,omitempty"`
	AuthorName string             `bson:"author_name" json:"author_name"`
	CreatedAt  time.Time          `bson:"created_at" json:"created_at"`
}

// SupportAck hides an automatic support reason for a learner until Until.
type SupportAck struct {
	ID         primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID     primitive.ObjectID `bson:"user_id" json:"user_id"`
	ReasonKey  string             `bson:"reason_key" json:"reason_key"`
	ReasonText string             `bson:"reason_text" json:"reason_text"` // the exact reason seen; a changed reason shows again
	Until      time.Time          `bson:"until" json:"until"`
	By         string             `bson:"by" json:"by"`
}
