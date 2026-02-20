package repository

import (
	"context"
	"errors"
	"time"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

var (
	ErrHolidayNotFound = errors.New("holiday not found")
)

type holidayRepository struct {
	collection *mongo.Collection
}

func NewHolidayRepository(db *mongo.Database) domain.HolidayRepository {
	return &holidayRepository{
		collection: db.Collection("holidays"),
	}
}

func (r *holidayRepository) Insert(ctx interface{}, holiday *domain.Holiday) error {
	c := ctx.(context.Context)
	holiday.ID = primitive.NewObjectID()
	holiday.CreatedAt = time.Now()
	_, err := r.collection.InsertOne(c, holiday)
	return err
}

func (r *holidayRepository) FindAll(ctx interface{}) ([]domain.Holiday, error) {
	c := ctx.(context.Context)
	cursor, err := r.collection.Find(c, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(c)

	var holidays []domain.Holiday
	if err := cursor.All(c, &holidays); err != nil {
		return nil, err
	}
	return holidays, nil
}

func (r *holidayRepository) FindByID(ctx interface{}, id primitive.ObjectID) (*domain.Holiday, error) {
	c := ctx.(context.Context)
	var holiday domain.Holiday
	err := r.collection.FindOne(c, bson.M{"_id": id}).Decode(&holiday)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrHolidayNotFound
		}
		return nil, err
	}
	return &holiday, nil
}

func (r *holidayRepository) Delete(ctx interface{}, id primitive.ObjectID) error {
	c := ctx.(context.Context)
	_, err := r.collection.DeleteOne(c, bson.M{"_id": id})
	return err
}
