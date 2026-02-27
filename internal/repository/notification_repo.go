package repository

import (
	"context"
	"errors"
	"time"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	ErrNotificationNotFound = errors.New("notification not found")
)

type notificationRepository struct {
	collection *mongo.Collection
}

func NewNotificationRepository(db *mongo.Database) domain.NotificationRepository {
	return &notificationRepository{
		collection: db.Collection("notifications"),
	}
}

func (r *notificationRepository) Create(notification *domain.Notification) error {
	notification.ID = primitive.NewObjectID()
	notification.CreatedAt = time.Now()
	notification.ReadByUsers = []primitive.ObjectID{}
	_, err := r.collection.InsertOne(context.Background(), notification)
	return err
}

func (r *notificationRepository) GetByID(id primitive.ObjectID) (*domain.Notification, error) {
	var notification domain.Notification
	err := r.collection.FindOne(context.Background(), bson.M{"_id": id}).Decode(&notification)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrNotificationNotFound
		}
		return nil, err
	}
	return &notification, nil
}

func (r *notificationRepository) GetAll() ([]domain.Notification, error) {
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})
	cursor, err := r.collection.Find(context.Background(), bson.M{}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())

	var notifications []domain.Notification
	if err := cursor.All(context.Background(), &notifications); err != nil {
		return nil, err
	}
	return notifications, nil
}

func (r *notificationRepository) GetActive() ([]domain.Notification, error) {
	now := time.Now()
	filter := bson.M{
		"is_active": true,
		"start_date": bson.M{
			"$lte": now,
		},
		"end_date": bson.M{
			"$gte": now,
		},
	}
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})
	cursor, err := r.collection.Find(context.Background(), filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())

	var notifications []domain.Notification
	if err := cursor.All(context.Background(), &notifications); err != nil {
		return nil, err
	}
	return notifications, nil
}

func (r *notificationRepository) Update(id primitive.ObjectID, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return nil
	}
	update := bson.M{"$set": updates}
	_, err := r.collection.UpdateOne(context.Background(), bson.M{"_id": id}, update)
	return err
}

func (r *notificationRepository) Delete(id primitive.ObjectID) error {
	_, err := r.collection.DeleteOne(context.Background(), bson.M{"_id": id})
	return err
}

func (r *notificationRepository) MarkAsRead(id primitive.ObjectID, userID primitive.ObjectID) error {
	update := bson.M{
		"$addToSet": bson.M{
			"read_by_users": userID,
		},
	}
	_, err := r.collection.UpdateOne(context.Background(), bson.M{"_id": id}, update)
	return err
}
