package repository

import (
	"context"

	"gofiber-baro/internal/service/report"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ReportRepository stores weekly report notes and sent snapshots (weekly_reports, one doc per cohort+week).
type ReportRepository struct{ col *mongo.Collection }

func NewReportRepository(db *mongo.Database) *ReportRepository {
	return &ReportRepository{col: db.Collection("weekly_reports")}
}

func (r *ReportRepository) EnsureIndexes(c context.Context) error {
	_, err := r.col.Indexes().CreateOne(c, mongo.IndexModel{Keys: bson.D{{Key: "cohort", Value: 1}, {Key: "week", Value: 1}}, Options: options.Index().SetUnique(true)})
	return err
}

func (r *ReportRepository) Get(cohort int, week string) (*report.Saved, error) {
	c, cancel := staffCtx()
	defer cancel()
	var s report.Saved
	err := r.col.FindOne(c, bson.M{"cohort": cohort, "week": week}).Decode(&s)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// unsent matches a week that has no sent report yet. The unique (cohort, week) index turns a
// race with a sent report into a duplicate-key error, which means "already sent".
func unsent(cohort int, week string) bson.M {
	return bson.M{"cohort": cohort, "week": week, "status": bson.M{"$ne": report.StatusSent}}
}

func (r *ReportRepository) SaveNotes(cohort int, week, notes string) (bool, error) {
	c, cancel := staffCtx()
	defer cancel()
	_, err := r.col.UpdateOne(c, unsent(cohort, week),
		bson.M{"$set": bson.M{"notes": notes}, "$setOnInsert": bson.M{"status": report.StatusDraft}},
		options.Update().SetUpsert(true))
	if mongo.IsDuplicateKeyError(err) {
		return false, nil
	}
	return err == nil, err
}

func (r *ReportRepository) SetPost(cohort int, week, errMsg string) error {
	c, cancel := staffCtx()
	defer cancel()
	_, err := r.col.UpdateOne(c, bson.M{"cohort": cohort, "week": week}, bson.M{"$set": bson.M{"post_error": errMsg}})
	return err
}

func (r *ReportRepository) MarkSent(s *report.Saved) (bool, error) {
	c, cancel := staffCtx()
	defer cancel()
	_, err := r.col.UpdateOne(c, unsent(s.Cohort, s.Week),
		bson.M{"$set": bson.M{"notes": s.Notes, "status": report.StatusSent, "sent_at": s.SentAt, "snapshot": s.Snapshot}},
		options.Update().SetUpsert(true))
	if mongo.IsDuplicateKeyError(err) {
		return false, nil
	}
	return err == nil, err
}
