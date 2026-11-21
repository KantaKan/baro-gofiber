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

// CupRepository stores GenMate Cup awards and weekly wins.
type CupRepository struct {
	awards *mongo.Collection
	wins   *mongo.Collection
	users  *mongo.Collection
}

func NewCupRepository(db *mongo.Database) *CupRepository {
	return &CupRepository{awards: db.Collection("cup_awards"), wins: db.Collection("cup_wins"), users: db.Collection("users")}
}

// EnsureIndexes makes server awards and weekly wins idempotent.
func (r *CupRepository) EnsureIndexes(ctx context.Context) error {
	if _, err := r.awards.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "key", Value: 1}}, Options: options.Index().SetUnique(true).SetSparse(true).SetName("cup_award_key")},
		{Keys: bson.D{{Key: "cohort", Value: 1}, {Key: "week", Value: 1}}, Options: options.Index().SetName("cup_award_week")},
	}); err != nil {
		return err
	}
	_, err := r.wins.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "cohort", Value: 1}, {Key: "week", Value: 1}}, Options: options.Index().SetUnique(true).SetName("cup_win_week"),
	})
	return err
}

func (r *CupRepository) Insert(ctx context.Context, a *domain.CupAward) error {
	a.ID = primitive.NewObjectID()
	_, err := r.awards.InsertOne(ctx, a)
	return err
}

func (r *CupRepository) InsertOnce(ctx context.Context, a *domain.CupAward) (bool, error) {
	if a.Key == "" {
		return false, errors.New("cup award key required")
	}
	err := r.Insert(ctx, a)
	if mongo.IsDuplicateKeyError(err) {
		return false, nil
	}
	return err == nil, err
}

func (r *CupRepository) List(ctx context.Context, cohort int, week string) ([]domain.CupAward, error) {
	cur, err := r.awards.Find(ctx, bson.M{"cohort": cohort, "week": week}, options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}))
	if err != nil {
		return nil, err
	}
	out := []domain.CupAward{}
	return out, cur.All(ctx, &out)
}

func (r *CupRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	res, err := r.awards.DeleteOne(ctx, bson.M{"_id": id, "category": bson.M{"$ne": "showing_up"}})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (r *CupRepository) InsertWinOnce(ctx context.Context, w *domain.CupWin) (bool, error) {
	w.ID = primitive.NewObjectID()
	_, err := r.wins.InsertOne(ctx, w)
	if mongo.IsDuplicateKeyError(err) {
		return false, nil
	}
	return err == nil, err
}

func (r *CupRepository) Wins(ctx context.Context, cohort int) ([]domain.CupWin, error) {
	cur, err := r.wins.Find(ctx, bson.M{"cohort": cohort}, options.Find().SetSort(bson.D{{Key: "week", Value: -1}}))
	if err != nil {
		return nil, err
	}
	out := []domain.CupWin{}
	return out, cur.All(ctx, &out)
}

// Groups lists active learners (not dropout/dismissed/deleted) per GenMate group.
func (r *CupRepository) Groups(ctx context.Context, cohort int) (map[string][]primitive.ObjectID, error) {
	cur, err := r.users.Find(ctx, bson.M{
		"cohort_number": cohort, "role": "learner", "deleted": bson.M{"$ne": true},
		"genmate_group": bson.M{"$nin": bson.A{"", nil}}, "attendance_status": bson.M{"$nin": bson.A{"dropout", "dismissed"}},
	}, options.Find().SetProjection(bson.M{"_id": 1, "genmate_group": 1}))
	if err != nil {
		return nil, err
	}
	var rows []struct {
		ID    primitive.ObjectID `bson:"_id"`
		Group string             `bson:"genmate_group"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, err
	}
	out := map[string][]primitive.ObjectID{}
	for _, r := range rows {
		out[r.Group] = append(out[r.Group], r.ID)
	}
	return out, nil
}
