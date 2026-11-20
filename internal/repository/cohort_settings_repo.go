package repository

import (
	"context"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type cohortSettingsRepository struct{ collection *mongo.Collection }

func NewCohortSettingsRepository(db *mongo.Database) domain.CohortSettingsRepository {
	col := db.Collection("cohort_settings")
	_, _ = col.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys:    bson.D{{Key: "cohort_number", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	return &cohortSettingsRepository{collection: col}
}

func (r *cohortSettingsRepository) FindByCohort(ctx interface{}, cohort int) (*domain.CohortSettings, error) {
	c := ctx.(context.Context)
	var s domain.CohortSettings
	err := r.collection.FindOne(c, bson.M{"cohort_number": cohort}).Decode(&s)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *cohortSettingsRepository) Upsert(ctx interface{}, s *domain.CohortSettings) error {
	c := ctx.(context.Context)
	_, err := r.collection.ReplaceOne(c, bson.M{"cohort_number": s.CohortNumber}, s, options.Replace().SetUpsert(true))
	return err
}
