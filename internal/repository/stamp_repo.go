package repository

import (
	"context"
	"fmt"
	"time"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const defaultLockWeeks = 15

type stampRepository struct {
	collection *mongo.Collection
}

func NewStampRepository(db *mongo.Database) domain.StampRepository {
	return &stampRepository{
		collection: db.Collection("stamps"),
	}
}

func (r *stampRepository) InsertStamp(ctx context.Context, stamp *domain.Stamp) error {
	stamp.ID = primitive.NewObjectID()
	_, err := r.collection.InsertOne(ctx, stamp)
	return err
}

func (r *stampRepository) FindByCohort(ctx context.Context, cohort int) ([]domain.Stamp, error) {
	findOpts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: 1}})
	cursor, err := r.collection.Find(ctx, bson.M{"cohortNumber": cohort}, findOpts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var stamps []domain.Stamp
	if err := cursor.All(ctx, &stamps); err != nil {
		return nil, err
	}
	return stamps, nil
}

func (r *stampRepository) HasStampAfter(ctx context.Context, ownerID primitive.ObjectID, after time.Time) (bool, error) {
	count, err := r.collection.CountDocuments(ctx, bson.M{
		"ownerId":   ownerID,
		"createdAt": bson.M{"$gte": after},
	})
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *stampRepository) DeleteByID(ctx context.Context, cohort int, id primitive.ObjectID) error {
	res, err := r.collection.DeleteOne(ctx, bson.M{"_id": id, "cohortNumber": cohort})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (r *stampRepository) DeleteByCohort(ctx context.Context, cohort int) error {
	_, err := r.collection.DeleteMany(ctx, bson.M{"cohortNumber": cohort})
	return err
}

type cohortRepository struct {
	collection *mongo.Collection
}

func NewCohortRepository(db *mongo.Database) domain.CohortRepository {
	return &cohortRepository{
		collection: db.Collection("cohorts"),
	}
}

func (r *cohortRepository) FindByCohortNumber(ctx context.Context, cohort int) (*domain.Cohort, error) {
	var doc domain.Cohort
	err := r.collection.FindOne(ctx, bson.M{"cohort_number": cohort}).Decode(&doc)
	if err != nil {
		return nil, err
	}
	return &doc, nil
}

func (r *cohortRepository) EnsureExists(ctx context.Context, cohort int) (*domain.Cohort, error) {
	now := time.Now()
	update := bson.M{
		"$setOnInsert": bson.M{
			"cohort_number": cohort,
			"name":          fmt.Sprintf("Cohort %d", cohort),
			"start_date":    now,
			"lock_at":       now.Add(defaultLockWeeks * 7 * 24 * time.Hour),
			"is_locked":     false,
			"created_at":    now,
		},
	}
	opts := options.Update().SetUpsert(true)
	if _, err := r.collection.UpdateOne(ctx, bson.M{"cohort_number": cohort}, update, opts); err != nil {
		return nil, err
	}
	return r.FindByCohortNumber(ctx, cohort)
}

func (r *cohortRepository) Update(ctx context.Context, cohort int, update interface{}) error {
	_, err := r.collection.UpdateOne(ctx, bson.M{"cohort_number": cohort}, update)
	return err
}

func (r *cohortRepository) List(ctx context.Context) ([]domain.Cohort, error) {
	findOpts := options.Find().SetSort(bson.D{{Key: "cohort_number", Value: 1}})
	cursor, err := r.collection.Find(ctx, bson.M{}, findOpts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var cohorts []domain.Cohort
	if err := cursor.All(ctx, &cohorts); err != nil {
		return nil, err
	}
	return cohorts, nil
}

func (r *cohortRepository) LockExpired(ctx context.Context) error {
	_, err := r.collection.UpdateMany(
		ctx,
		bson.M{"lock_at": bson.M{"$lte": time.Now()}, "is_locked": false},
		bson.M{"$set": bson.M{"is_locked": true}},
	)
	return err
}
