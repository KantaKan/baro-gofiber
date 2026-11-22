package repository

import (
	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// CohortListRepository answers "which cohorts exist": learner counts per cohort and start dates.
type CohortListRepository struct{ users, settings *mongo.Collection }

func NewCohortListRepository(db *mongo.Database) *CohortListRepository {
	return &CohortListRepository{users: db.Collection("users"), settings: db.Collection("cohort_settings")}
}

func (r *CohortListRepository) LearnerCounts() (map[int]int, error) {
	c, cancel := staffCtx()
	defer cancel()
	cur, err := r.users.Aggregate(c, mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"role": "learner", "cohort_number": bson.M{"$gt": 0}}}},
		{{Key: "$group", Value: bson.M{"_id": "$cohort_number", "n": bson.M{"$sum": 1}}}},
	})
	if err != nil {
		return nil, err
	}
	var rows []struct {
		ID int `bson:"_id"`
		N  int `bson:"n"`
	}
	if err := cur.All(c, &rows); err != nil {
		return nil, err
	}
	out := map[int]int{}
	for _, r := range rows {
		out[r.ID] = r.N
	}
	return out, nil
}

func (r *CohortListRepository) StartDates() (map[int]string, error) {
	c, cancel := staffCtx()
	defer cancel()
	cur, err := r.settings.Find(c, bson.M{}, options.Find().SetProjection(bson.M{"cohort_number": 1, "start_date": 1}))
	if err != nil {
		return nil, err
	}
	var rows []domain.CohortSettings
	if err := cur.All(c, &rows); err != nil {
		return nil, err
	}
	out := map[int]string{}
	for _, s := range rows {
		out[s.CohortNumber] = s.StartDate
	}
	return out, nil
}
