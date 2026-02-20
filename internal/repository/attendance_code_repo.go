package repository

import (
	"context"
	"time"

	"gofiber-baro/internal/domain"
	"gofiber-baro/pkg/utils"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type attendanceCodeRepository struct {
	collection *mongo.Collection
}

func NewAttendanceCodeRepository(db *mongo.Database) domain.AttendanceCodeRepository {
	return &attendanceCodeRepository{
		collection: db.Collection("attendance_codes"),
	}
}

func (r *attendanceCodeRepository) InsertCode(ctx interface{}, code *domain.AttendanceCode) error {
	c := ctx.(context.Context)
	code.ID = primitive.NewObjectID()
	_, err := r.collection.InsertOne(c, code)
	return err
}

func (r *attendanceCodeRepository) FindActiveCode(ctx interface{}, cohort int, session domain.AttendanceSession) (*domain.AttendanceCode, error) {
	c := ctx.(context.Context)
	filter := bson.M{
		"cohort_number": cohort,
		"session":       session,
		"is_active":     true,
		"expires_at":    bson.M{"$gt": utils.GetThailandTime()},
	}

	var code domain.AttendanceCode
	err := r.collection.FindOne(c, filter).Decode(&code)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &code, nil
}

func (r *attendanceCodeRepository) DeactivateOldCodes(ctx interface{}, cohort int, session domain.AttendanceSession) error {
	c := ctx.(context.Context)
	filter := bson.M{
		"cohort_number": cohort,
		"session":       session,
		"is_active":     true,
	}
	update := bson.M{"$set": bson.M{"is_active": false}}
	_, err := r.collection.UpdateMany(c, filter, update)
	return err
}

func (r *attendanceCodeRepository) DeleteExpiredCodes(ctx interface{}) error {
	c := ctx.(context.Context)
	filter := bson.M{
		"expires_at": bson.M{"$lt": time.Now()},
	}
	_, err := r.collection.DeleteMany(c, filter)
	return err
}
