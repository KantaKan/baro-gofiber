package domain

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Reaction struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	UserID    primitive.ObjectID `bson:"userId" json:"userId"`
	Type      string             `bson:"type" json:"type"`
	Value     string             `bson:"value" json:"value"`
	CreatedAt time.Time          `bson:"createdAt" json:"createdAt"`
}

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

type PostFilter struct {
	Cohort int
}

type TalkBoardRepository interface {
	InsertPost(ctx context.Context, post *Post) error
	FindPosts(ctx context.Context, filter PostFilter, opts interface{}) ([]Post, error)
	FindByID(ctx context.Context, id primitive.ObjectID) (*Post, error)
	UpdatePost(ctx context.Context, id primitive.ObjectID, update interface{}) error
	DeletePost(ctx context.Context, id primitive.ObjectID) error
	AddComment(ctx context.Context, postID primitive.ObjectID, comment Comment) error
	AddReaction(ctx context.Context, postID primitive.ObjectID, reaction Reaction) error
}
