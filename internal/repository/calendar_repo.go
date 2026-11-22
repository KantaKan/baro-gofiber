package repository

import (
	"context"
	"time"

	"gofiber-baro/internal/service/calendar"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// CalendarRepository stores one calendar document per cohort (calendars). Every write bumps `version`,
// so an import based on an older version is refused instead of overwriting a newer publish.
type CalendarRepository struct{ col *mongo.Collection }

func NewCalendarRepository(db *mongo.Database) *CalendarRepository {
	return &CalendarRepository{col: db.Collection("calendars")}
}

func (r *CalendarRepository) EnsureIndexes(c context.Context) error {
	_, err := r.col.Indexes().CreateOne(c, mongo.IndexModel{Keys: bson.D{{Key: "cohort", Value: 1}}, Options: options.Index().SetUnique(true)})
	return err
}

func (r *CalendarRepository) Get(cohort int) (*calendar.Calendar, error) {
	c, cancel := staffCtx()
	defer cancel()
	var cal calendar.Calendar
	err := r.col.FindOne(c, bson.M{"cohort": cohort}).Decode(&cal)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &cal, nil
}

func (r *CalendarRepository) Save(cal *calendar.Calendar, expectVersion int) error {
	c, cancel := staffCtx()
	defer cancel()
	if expectVersion == 0 {
		cal.Version = 1
		_, err := r.col.InsertOne(c, cal)
		if mongo.IsDuplicateKeyError(err) {
			return calendar.ErrConflict
		}
		return err
	}
	res, err := r.col.UpdateOne(c, bson.M{"cohort": cal.Cohort, "version": expectVersion}, bson.M{"$set": bson.M{
		"sheet": cal.Sheet, "zoom_url": cal.ZoomURL, "weeks": cal.Weeks, "imported_at": cal.ImportedAt, "imported_by": cal.ImportedBy, "version": expectVersion + 1,
	}})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return calendar.ErrConflict
	}
	return nil
}

func (r *CalendarRepository) SetPublished(cohort, n int, published bool, at time.Time) (bool, error) {
	c, cancel := staffCtx()
	defer cancel()
	set := bson.M{"weeks.$.published": published, "weeks.$.published_at": nil}
	if published {
		set["weeks.$.published_at"] = at
	}
	res, err := r.col.UpdateOne(c, bson.M{"cohort": cohort, "weeks.n": n}, bson.M{"$set": set, "$inc": bson.M{"version": 1}})
	if err != nil {
		return false, err
	}
	return res.MatchedCount > 0, nil
}

func (r *CalendarRepository) SetZoom(cohort int, url string) error {
	c, cancel := staffCtx()
	defer cancel()
	res, err := r.col.UpdateOne(c, bson.M{"cohort": cohort}, bson.M{"$set": bson.M{"zoom_url": url}, "$inc": bson.M{"version": 1}})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return calendar.ErrNoCalendar
	}
	return nil
}
