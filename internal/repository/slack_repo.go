package repository

import (
	"context"

	"gofiber-baro/internal/service/slack"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const slackConfigID = "slack"

// SlackRepository stores the Slack setup (slack_config), the send log (slack_log, 30 days)
// and each learner's Slack user id (users.slack_id).
type SlackRepository struct{ cfg, logs, users *mongo.Collection }

func NewSlackRepository(db *mongo.Database) *SlackRepository {
	return &SlackRepository{cfg: db.Collection("slack_config"), logs: db.Collection("slack_log"), users: db.Collection("users")}
}

func (r *SlackRepository) EnsureIndexes(c context.Context) error {
	_, err := r.logs.Indexes().CreateOne(c, mongo.IndexModel{Keys: bson.D{{Key: "at", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(30 * 24 * 3600)})
	return err
}

func (r *SlackRepository) Config() (*slack.Config, error) {
	c, cancel := staffCtx()
	defer cancel()
	var out slack.Config
	err := r.cfg.FindOne(c, bson.M{"_id": slackConfigID}).Decode(&out)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *SlackRepository) SaveConfig(cfg *slack.Config) error {
	c, cancel := staffCtx()
	defer cancel()
	_, err := r.cfg.UpdateOne(c, bson.M{"_id": slackConfigID}, bson.M{"$set": cfg}, options.Update().SetUpsert(true))
	return err
}

func (r *SlackRepository) DeleteConfig() error {
	c, cancel := staffCtx()
	defer cancel()
	_, err := r.cfg.DeleteOne(c, bson.M{"_id": slackConfigID})
	return err
}

func (r *SlackRepository) AddLog(e slack.LogEntry) error {
	c, cancel := staffCtx()
	defer cancel()
	_, err := r.logs.InsertOne(c, e)
	return err
}

func (r *SlackRepository) Log(limit int) ([]slack.LogEntry, error) {
	c, cancel := staffCtx()
	defer cancel()
	cur, err := r.logs.Find(c, bson.M{}, options.Find().SetSort(bson.M{"at": -1}).SetLimit(int64(limit)))
	if err != nil {
		return nil, err
	}
	out := []slack.LogEntry{}
	return out, cur.All(c, &out)
}

type learnerDoc struct {
	ID        primitive.ObjectID `bson:"_id"`
	First     string             `bson:"first_name"`
	Last      string             `bson:"last_name"`
	Zoom      string             `bson:"zoom_name"`
	Email     string             `bson:"email"`
	SlackID   string             `bson:"slack_id"`
	JSDNumber string             `bson:"jsd_number"`
}

func (d learnerDoc) learner() slack.Learner {
	name := d.First + " " + d.Last
	return slack.Learner{ID: d.ID, Name: name, Email: d.Email, SlackID: d.SlackID}
}

var learnerProjection = bson.M{"first_name": 1, "last_name": 1, "zoom_name": 1, "email": 1, "slack_id": 1, "jsd_number": 1}

func (r *SlackRepository) find(filter bson.M) ([]slack.Learner, error) {
	c, cancel := staffCtx()
	defer cancel()
	cur, err := r.users.Find(c, filter, options.Find().SetProjection(learnerProjection))
	if err != nil {
		return nil, err
	}
	var docs []learnerDoc
	if err := cur.All(c, &docs); err != nil {
		return nil, err
	}
	out := make([]slack.Learner, 0, len(docs))
	for _, d := range docs {
		out = append(out, d.learner())
	}
	return out, nil
}

// Learners: active learners of a cohort.
func (r *SlackRepository) Learners(cohort int) ([]slack.Learner, error) {
	return r.find(bson.M{"role": "learner", "cohort_number": cohort, "attendance_status": bson.M{"$nin": []string{"dropout", "dismissed"}}})
}

func (r *SlackRepository) LearnersByIDs(ids []primitive.ObjectID) ([]slack.Learner, error) {
	return r.find(bson.M{"_id": bson.M{"$in": ids}, "role": "learner"})
}

func (r *SlackRepository) SetSlackID(id primitive.ObjectID, slackID string) error {
	c, cancel := staffCtx()
	defer cancel()
	_, err := r.users.UpdateOne(c, bson.M{"_id": id}, bson.M{"$set": bson.M{"slack_id": slackID}})
	return err
}
