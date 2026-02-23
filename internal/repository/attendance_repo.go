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
	ErrRecordNotFound = errors.New("attendance record not found")
)

type attendanceRepository struct {
	collection *mongo.Collection
}

func NewAttendanceRepository(db *mongo.Database) domain.AttendanceRepository {
	return &attendanceRepository{
		collection: db.Collection("attendance_records"),
	}
}

func (r *attendanceRepository) InsertRecord(ctx interface{}, record *domain.AttendanceRecord) error {
	c := ctx.(context.Context)
	record.ID = primitive.NewObjectID()
	_, err := r.collection.InsertOne(c, record)
	return err
}

func (r *attendanceRepository) FindRecord(ctx interface{}, filter domain.AttendanceRecordFilter) (*domain.AttendanceRecord, error) {
	c := ctx.(context.Context)
	bsonFilter := r.buildFilter(filter)

	var record domain.AttendanceRecord
	err := r.collection.FindOne(c, bsonFilter).Decode(&record)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrRecordNotFound
		}
		return nil, err
	}
	return &record, nil
}

func (r *attendanceRepository) FindRecords(ctx interface{}, filter domain.AttendanceRecordFilter, opts interface{}) ([]domain.AttendanceRecord, error) {
	c := ctx.(context.Context)
	bsonFilter := r.buildFilter(filter)

	findOpts := options.Find()
	if opts != nil {
		if o, ok := opts.(*options.FindOptions); ok {
			findOpts = o
		}
	}

	cursor, err := r.collection.Find(c, bsonFilter, findOpts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(c)

	var records []domain.AttendanceRecord
	if err := cursor.All(c, &records); err != nil {
		return nil, err
	}
	return records, nil
}

func (r *attendanceRepository) UpdateRecord(ctx interface{}, id primitive.ObjectID, update interface{}) error {
	c := ctx.(context.Context)
	filter := bson.M{"_id": id}
	_, err := r.collection.UpdateOne(c, filter, bson.M{"$set": update})
	return err
}

func (r *attendanceRepository) UpdateRecords(ctx interface{}, filter domain.AttendanceRecordFilter, update interface{}) error {
	c := ctx.(context.Context)
	bsonFilter := r.buildFilter(filter)
	_, err := r.collection.UpdateMany(c, bsonFilter, bson.M{"$set": update})
	return err
}

func (r *attendanceRepository) DeleteRecord(ctx interface{}, id primitive.ObjectID, deletedBy string) error {
	c := ctx.(context.Context)
	filter := bson.M{"_id": id}
	update := bson.M{
		"$set": bson.M{
			"deleted":    true,
			"deleted_at": time.Now(),
			"deleted_by": deletedBy,
		},
	}
	_, err := r.collection.UpdateOne(c, filter, update)
	return err
}

func (r *attendanceRepository) CountRecords(ctx interface{}, filter domain.AttendanceRecordFilter) (int64, error) {
	c := ctx.(context.Context)
	bsonFilter := r.buildFilter(filter)
	return r.collection.CountDocuments(c, bsonFilter)
}

func (r *attendanceRepository) AggregateStats(ctx interface{}, pipeline interface{}) ([]domain.AttendanceStats, error) {
	c := ctx.(context.Context)
	cursor, err := r.collection.Aggregate(c, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(c)

	var stats []domain.AttendanceStats
	if err := cursor.All(c, &stats); err != nil {
		return nil, err
	}
	return stats, nil
}

func (r *attendanceRepository) AggregateDailyStats(ctx interface{}, pipeline interface{}) ([]map[string]interface{}, error) {
	c := ctx.(context.Context)
	cursor, err := r.collection.Aggregate(c, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(c)

	var results []bson.M
	if err := cursor.All(c, &results); err != nil {
		return nil, err
	}

	stats := make([]map[string]interface{}, len(results))
	for i, r := range results {
		stats[i] = map[string]interface{}{
			"date":           r["_id"],
			"present":        r["present"],
			"late":           r["late"],
			"absent":         r["absent"],
			"late_excused":   r["late_excused"],
			"absent_excused": r["absent_excused"],
			"total":          r["present"].(int32) + r["late"].(int32) + r["absent"].(int32) + r["late_excused"].(int32) + r["absent_excused"].(int32),
		}
	}
	return stats, nil
}

func (r *attendanceRepository) buildFilter(filter domain.AttendanceRecordFilter) bson.M {
	bsonFilter := bson.M{}

	if filter.NotDeleted {
		bsonFilter["deleted"] = bson.M{"$ne": true}
	}
	if filter.Cohort > 0 {
		bsonFilter["cohort_number"] = filter.Cohort
	}
	if filter.Date != "" {
		bsonFilter["date"] = filter.Date
	}
	if filter.Session != "" {
		bsonFilter["session"] = filter.Session
	}
	if !filter.UserID.IsZero() {
		bsonFilter["user_id"] = filter.UserID
	}

	return bsonFilter
}
