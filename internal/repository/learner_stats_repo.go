package repository

import (
	"context"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type learnerStatsRepository struct{ col *mongo.Collection }

func NewLearnerStatsRepository(db *mongo.Database) domain.LearnerStatsRepository {
	col := db.Collection("learner_stats")
	_, _ = col.Indexes().CreateOne(context.Background(), mongo.IndexModel{Keys: bson.D{{Key: "cohort_number", Value: 1}}})
	return &learnerStatsRepository{col: col}
}

func (r *learnerStatsRepository) Upsert(ctx context.Context, s domain.LearnerStats) error {
	_, err := r.col.ReplaceOne(ctx, bson.M{"_id": s.UserID}, s, options.Replace().SetUpsert(true))
	return err
}

func (r *learnerStatsRepository) UpsertMany(ctx context.Context, all []domain.LearnerStats) error {
	if len(all) == 0 {
		return nil
	}
	models := make([]mongo.WriteModel, len(all))
	for i, s := range all {
		models[i] = mongo.NewReplaceOneModel().SetFilter(bson.M{"_id": s.UserID}).SetReplacement(s).SetUpsert(true)
	}
	_, err := r.col.BulkWrite(ctx, models, options.BulkWrite().SetOrdered(false))
	return err
}

func (r *learnerStatsRepository) FindByCohort(ctx context.Context, cohort int) ([]domain.LearnerStats, error) {
	cur, err := r.col.Find(ctx, bson.M{"cohort_number": cohort})
	if err != nil {
		return nil, err
	}
	var out []domain.LearnerStats
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}
