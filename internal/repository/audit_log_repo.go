package repository

import (
	"context"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type auditLogRepository struct {
	collection *mongo.Collection
}

func NewAuditLogRepository(db *mongo.Database) domain.AuditLogRepository {
	return &auditLogRepository{
		collection: db.Collection("audit_logs"),
	}
}

func (r *auditLogRepository) Insert(ctx context.Context, entry *domain.AuditLog) error {
	_, err := r.collection.InsertOne(ctx, entry)
	return err
}

func (r *auditLogRepository) FindPage(ctx context.Context, filter bson.M, page, limit int) ([]domain.AuditLog, int64, error) {
	if filter == nil {
		filter = bson.M{}
	}

	total, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	skip := int64((page - 1) * limit)
	findOpts := options.Find().
		SetSort(bson.D{{Key: "createdAt", Value: -1}}).
		SetSkip(skip).
		SetLimit(int64(limit))

	cursor, err := r.collection.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var logs []domain.AuditLog
	if err := cursor.All(ctx, &logs); err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}
