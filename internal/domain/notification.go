package domain

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Notification struct {
	ID          primitive.ObjectID   `json:"id" bson:"_id,omitempty"`
	Title       string               `json:"title" bson:"title"`
	Message     string               `json:"message" bson:"message"`
	Link        string               `json:"link,omitempty" bson:"link,omitempty"`
	LinkText    string               `json:"link_text,omitempty" bson:"link_text,omitempty"`
	IsActive    bool                 `json:"is_active" bson:"is_active"`
	Priority    string               `json:"priority" bson:"priority"`
	StartDate   time.Time            `json:"start_date" bson:"start_date"`
	EndDate     time.Time            `json:"end_date" bson:"end_date"`
	CreatedAt   time.Time            `json:"created_at" bson:"created_at"`
	ReadByUsers []primitive.ObjectID `json:"read_by_users" bson:"read_by_users"`
}

type NotificationRepository interface {
	Create(notification *Notification) error
	GetByID(id primitive.ObjectID) (*Notification, error)
	GetAll() ([]Notification, error)
	GetActive() ([]Notification, error)
	Update(id primitive.ObjectID, updates map[string]interface{}) error
	Delete(id primitive.ObjectID) error
	MarkAsRead(id primitive.ObjectID, userID primitive.ObjectID) error
}
