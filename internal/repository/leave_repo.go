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
	ErrLeaveRequestNotFound = errors.New("leave request not found")
)

type leaveRequestRepository struct {
	collection *mongo.Collection
}

func NewLeaveRequestRepository(db *mongo.Database) domain.LeaveRequestRepository {
	return &leaveRequestRepository{
		collection: db.Collection("leave_requests"),
	}
}

func (r *leaveRequestRepository) Insert(ctx interface{}, request *domain.LeaveRequest) error {
	c := ctx.(context.Context)
	request.ID = primitive.NewObjectID()
	request.CreatedAt = time.Now()
	_, err := r.collection.InsertOne(c, request)
	return err
}

func (r *leaveRequestRepository) FindByID(ctx interface{}, id primitive.ObjectID) (*domain.LeaveRequest, error) {
	c := ctx.(context.Context)
	var request domain.LeaveRequest
	err := r.collection.FindOne(c, bson.M{"_id": id}).Decode(&request)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrLeaveRequestNotFound
		}
		return nil, err
	}
	return &request, nil
}

func (r *leaveRequestRepository) FindAll(ctx interface{}, filter domain.LeaveRequestFilter) ([]domain.LeaveRequest, error) {
	c := ctx.(context.Context)
	bsonFilter := r.buildFilter(filter)

	cursor, err := r.collection.Find(c, bsonFilter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(c)

	var requests []domain.LeaveRequest
	if err := cursor.All(c, &requests); err != nil {
		return nil, err
	}
	return requests, nil
}

func (r *leaveRequestRepository) FindByUserID(ctx interface{}, userID primitive.ObjectID) ([]domain.LeaveRequest, error) {
	c := ctx.(context.Context)
	cursor, err := r.collection.Find(c, bson.M{"user_id": userID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(c)

	var requests []domain.LeaveRequest
	if err := cursor.All(c, &requests); err != nil {
		return nil, err
	}
	return requests, nil
}

func (r *leaveRequestRepository) UpdateStatus(ctx interface{}, id primitive.ObjectID, status domain.LeaveRequestStatus, reviewedBy primitive.ObjectID, reviewedByName, reviewNotes string) error {
	c := ctx.(context.Context)
	now := time.Now()
	filter := bson.M{"_id": id}
	update := bson.M{
		"$set": bson.M{
			"status":           status,
			"reviewed_by":      reviewedBy,
			"reviewed_by_name": reviewedByName,
			"reviewed_at":      now,
			"review_notes":     reviewNotes,
		},
	}
	_, err := r.collection.UpdateOne(c, filter, update)
	return err
}

func (r *leaveRequestRepository) buildFilter(filter domain.LeaveRequestFilter) bson.M {
	bsonFilter := bson.M{}

	if filter.Cohort > 0 {
		bsonFilter["cohort_number"] = filter.Cohort
	}
	if filter.Status != "" {
		bsonFilter["status"] = filter.Status
	}
	if !filter.UserID.IsZero() {
		bsonFilter["user_id"] = filter.UserID
	}

	return bsonFilter
}
