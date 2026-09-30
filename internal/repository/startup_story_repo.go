package repository

import (
	"context"
	"errors"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type StartupStoryRepository struct {
	studios *mongo.Collection
	runs    *mongo.Collection
}

func NewStartupStoryRepository(db *mongo.Database) *StartupStoryRepository {
	return &StartupStoryRepository{studios: db.Collection("startup_studios"), runs: db.Collection("startup_runs")}
}

func (r *StartupStoryRepository) EnsureIndexes(ctx context.Context) error {
	if _, err := r.studios.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "owner_id", Value: 1}}, Options: options.Index().SetUnique(true).SetName("unique_studio_owner"),
	}); err != nil {
		return err
	}
	_, err := r.runs.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "owner_id", Value: 1}}, Options: options.Index().SetUnique(true).SetName("unique_active_run_per_owner").SetPartialFilterExpression(bson.M{"status": domain.StartupStatusActive})},
		{Keys: bson.D{{Key: "cohort", Value: 1}, {Key: "mode", Value: 1}, {Key: "week_key", Value: 1}, {Key: "score", Value: -1}}, Options: options.Index().SetName("run_leaderboard")},
	})
	return err
}

func (r *StartupStoryRepository) FindStudio(ctx context.Context, ownerID primitive.ObjectID) (*domain.StartupStudio, error) {
	var studio domain.StartupStudio
	err := r.studios.FindOne(ctx, bson.M{"owner_id": ownerID}).Decode(&studio)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &studio, nil
}

func (r *StartupStoryRepository) InsertStudio(ctx context.Context, studio domain.StartupStudio) error {
	_, err := r.studios.InsertOne(ctx, studio)
	if mongo.IsDuplicateKeyError(err) {
		return nil
	}
	return err
}

func (r *StartupStoryRepository) FindActiveRun(ctx context.Context, ownerID primitive.ObjectID) (*domain.StartupRun, error) {
	var run domain.StartupRun
	err := r.runs.FindOne(ctx, bson.M{"owner_id": ownerID, "status": domain.StartupStatusActive}).Decode(&run)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &run, nil
}

func (r *StartupStoryRepository) InsertRun(ctx context.Context, run *domain.StartupRun) error {
	_, err := r.runs.InsertOne(ctx, run)
	if mongo.IsDuplicateKeyError(err) {
		return domain.ErrStartupRunActive
	}
	return err
}

func (r *StartupStoryRepository) SaveRun(ctx context.Context, run *domain.StartupRun, expectedVersion int) error {
	result, err := r.runs.ReplaceOne(ctx, bson.M{"_id": run.ID, "version": expectedVersion}, run)
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return domain.ErrStartupRunConflict
	}
	return nil
}
