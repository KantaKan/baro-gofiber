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

type StartupStoryRepository struct {
	studios *mongo.Collection
	runs    *mongo.Collection
	users   *mongo.Collection
}

func NewStartupStoryRepository(db *mongo.Database) *StartupStoryRepository {
	return &StartupStoryRepository{studios: db.Collection("startup_studios"), runs: db.Collection("startup_runs"), users: db.Collection("users")}
}

func leaderboardNameExpr() bson.M {
	nameLen := bson.M{"$strLenCP": bson.M{"$ifNull": bson.A{"$user.first_name", ""}}}
	return bson.M{"$cond": bson.M{
		"if":   bson.M{"$gt": bson.A{nameLen, 0}},
		"then": "$user.first_name",
		"else": bson.M{"$ifNull": bson.A{"$user.jsd_number", "?"}},
	}}
}

func weeklyLeaderboardPipeline(cohort int, weekKey string, limit int) mongo.Pipeline {
	return mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.M{"cohort": cohort, "mode": domain.StartupModeRanked, "week_key": weekKey, "status": domain.StartupStatusEnded, "role": bson.M{"$ne": "admin"}}}},
		bson.D{{Key: "$sort", Value: bson.M{"score": -1}}},
		bson.D{{Key: "$group", Value: bson.M{"_id": "$owner_id", "score": bson.M{"$first": "$score"}, "run_id": bson.M{"$first": "$_id"}, "outcome": bson.M{"$first": "$outcome"}}}},
		bson.D{{Key: "$lookup", Value: bson.M{"from": "users", "localField": "_id", "foreignField": "_id", "as": "user"}}},
		bson.D{{Key: "$unwind", Value: "$user"}},
		bson.D{{Key: "$project", Value: bson.M{"_id": 0, "owner_id": "$_id", "name": leaderboardNameExpr(), "score": 1, "best_run_id": "$run_id", "outcome": 1}}},
		bson.D{{Key: "$sort", Value: bson.M{"score": -1}}},
		bson.D{{Key: "$limit", Value: limit}},
	}
}

func deepestLeaderboardPipeline(cohort int, weekKey string, limit int) mongo.Pipeline {
	byDepth := bson.D{{Key: "depth", Value: -1}, {Key: "score", Value: -1}}
	return mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.M{"cohort": cohort, "mode": domain.StartupModeRanked, "week_key": weekKey, "status": domain.StartupStatusEnded, "role": bson.M{"$ne": "admin"}}}},
		bson.D{{Key: "$addFields", Value: bson.M{"depth": bson.M{"$ifNull": bson.A{"$max_act", 0}}}}},
		bson.D{{Key: "$sort", Value: byDepth}},
		bson.D{{Key: "$group", Value: bson.M{"_id": "$owner_id", "depth": bson.M{"$first": "$depth"}, "score": bson.M{"$first": "$score"}, "run_id": bson.M{"$first": "$_id"}, "outcome": bson.M{"$first": "$outcome"}}}},
		bson.D{{Key: "$lookup", Value: bson.M{"from": "users", "localField": "_id", "foreignField": "_id", "as": "user"}}},
		bson.D{{Key: "$unwind", Value: "$user"}},
		bson.D{{Key: "$sort", Value: byDepth}},
		bson.D{{Key: "$limit", Value: limit}},
		bson.D{{Key: "$project", Value: bson.M{"_id": 0, "owner_id": "$_id", "name": leaderboardNameExpr(), "max_act": "$depth", "score": 1, "best_run_id": "$run_id", "outcome": 1}}},
	}
}

func fameLeaderboardPipeline(cohort int, limit int) mongo.Pipeline {
	return mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.M{"cohort": cohort}}},
		bson.D{{Key: "$lookup", Value: bson.M{"from": "users", "localField": "owner_id", "foreignField": "_id", "as": "user"}}},
		bson.D{{Key: "$unwind", Value: "$user"}},
		bson.D{{Key: "$match", Value: bson.M{"user.role": bson.M{"$ne": "admin"}}}},
		bson.D{{Key: "$project", Value: bson.M{"_id": 0, "owner_id": "$owner_id", "name": leaderboardNameExpr(), "fame": 1}}},
		bson.D{{Key: "$sort", Value: bson.M{"fame": -1}}},
		bson.D{{Key: "$limit", Value: limit}},
	}
}

func (r *StartupStoryRepository) LeaderboardDeepest(ctx context.Context, cohort int, weekKey string, limit int) ([]domain.StartupLeaderboardEntry, error) {
	return r.runLeaderboard(ctx, r.runs, deepestLeaderboardPipeline(cohort, weekKey, limit))
}

func (r *StartupStoryRepository) LeaderboardWeekly(ctx context.Context, cohort int, weekKey string, limit int) ([]domain.StartupLeaderboardEntry, error) {
	return r.runLeaderboard(ctx, r.runs, weeklyLeaderboardPipeline(cohort, weekKey, limit))
}

func (r *StartupStoryRepository) LeaderboardFame(ctx context.Context, cohort int, limit int) ([]domain.StartupLeaderboardEntry, error) {
	return r.runLeaderboard(ctx, r.studios, fameLeaderboardPipeline(cohort, limit))
}

func (r *StartupStoryRepository) runLeaderboard(ctx context.Context, coll *mongo.Collection, pipeline mongo.Pipeline) ([]domain.StartupLeaderboardEntry, error) {
	cur, err := coll.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	entries := []domain.StartupLeaderboardEntry{}
	if err := cur.All(ctx, &entries); err != nil {
		return nil, err
	}
	return entries, nil
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
		{Keys: bson.D{{Key: "owner_id", Value: 1}, {Key: "mode", Value: 1}, {Key: "week_key", Value: 1}}, Options: options.Index().SetName("run_weekly_attempts")},
	})
	return err
}

func (r *StartupStoryRepository) CountRankedRuns(ctx context.Context, ownerID primitive.ObjectID, weekKey string) (int, error) {
	n, err := r.runs.CountDocuments(ctx, bson.M{"owner_id": ownerID, "mode": domain.StartupModeRanked, "week_key": weekKey})
	return int(n), err
}

func genmateFilter(cohort int, excludeID primitive.ObjectID) bson.M {
	return bson.M{
		"cohort_number":            cohort,
		"role":                     bson.M{"$ne": "admin"},
		"_id":                      bson.M{"$ne": excludeID},
		"startup_story_opt_out":    bson.M{"$ne": true},
		"deleted":                  bson.M{"$ne": true},
	}
}

func genmateProjection() bson.M {
	return bson.M{"first_name": 1, "jsd_number": 1, "_id": 1}
}

func genmateDisplayName(firstName, jsdNumber string) string {
	if firstName != "" {
		return firstName
	}
	if jsdNumber != "" {
		return jsdNumber
	}
	return "?"
}

func (r *StartupStoryRepository) ListGenmates(ctx context.Context, cohort int, excludeID primitive.ObjectID) ([]domain.StartupGenmate, error) {
	cur, err := r.users.Find(ctx, genmateFilter(cohort, excludeID),
		options.Find().SetProjection(genmateProjection()).SetSort(bson.M{"_id": 1}).SetLimit(200))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var users []struct {
		ID        primitive.ObjectID `bson:"_id"`
		FirstName string             `bson:"first_name"`
		JSDNumber string             `bson:"jsd_number"`
	}
	if err := cur.All(ctx, &users); err != nil {
		return nil, err
	}
	out := make([]domain.StartupGenmate, 0, len(users))
	for _, u := range users {
		out = append(out, domain.StartupGenmate{UserID: u.ID, Name: genmateDisplayName(u.FirstName, u.JSDNumber)})
	}
	return out, nil
}

func (r *StartupStoryRepository) SetStartupOptOut(ctx context.Context, userID primitive.ObjectID, optOut bool) error {
	_, err := r.users.UpdateOne(ctx, bson.M{"_id": userID}, bson.M{"$set": bson.M{"startup_story_opt_out": optOut}})
	return err
}

func (r *StartupStoryRepository) FindStartupOptOut(ctx context.Context, userID primitive.ObjectID) (bool, error) {
	var doc struct {
		OptOut bool `bson:"startup_story_opt_out"`
	}
	err := r.users.FindOne(ctx, bson.M{"_id": userID}, options.FindOne().SetProjection(bson.M{"startup_story_opt_out": 1})).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return doc.OptOut, nil
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

func (r *StartupStoryRepository) AddDiscoveredCombo(ctx context.Context, ownerID primitive.ObjectID, key string) error {
	_, err := r.studios.UpdateOne(ctx,
		bson.M{"owner_id": ownerID},
		bson.M{
			"$addToSet": bson.M{"discovered_combos": key},
			"$set":      bson.M{"updated_at": time.Now().UTC()},
		},
		options.Update().SetUpsert(true),
	)
	return err
}

func (r *StartupStoryRepository) SettleRun(ctx context.Context, ownerID primitive.ObjectID, entry domain.StartupHallEntry, fameGain int, founders, items []string, skin string) error {
	setDoc := bson.M{"updated_at": time.Now().UTC()}
	if skin != "" {
		setDoc["office_skin"] = skin
	}
	update := bson.M{
		"$inc":  bson.M{"fame": fameGain},
		"$push": bson.M{"hall_of_fame": bson.M{"$each": []domain.StartupHallEntry{entry}, "$slice": -20}},
		"$set":  setDoc,
	}
	addToSet := bson.M{}
	if len(founders) > 0 {
		addToSet["unlocked_founders"] = bson.M{"$each": founders}
	}
	if len(items) > 0 {
		addToSet["unlocked_items"] = bson.M{"$each": items}
	}
	if len(addToSet) > 0 {
		update["$addToSet"] = addToSet
	}
	res, err := r.studios.UpdateOne(ctx,
		bson.M{"owner_id": ownerID, "hall_of_fame.run_id": bson.M{"$ne": entry.RunID}},
		update,
	)
	if err != nil {
		return err
	}
	if res.MatchedCount > 0 {
		return nil
	}
	var studio domain.StartupStudio
	err = r.studios.FindOne(ctx, bson.M{"owner_id": ownerID}).Decode(&studio)
	if errors.Is(err, mongo.ErrNoDocuments) {
		now := time.Now().UTC()
		fresh := domain.StartupStudio{
			ID: primitive.NewObjectID(), OwnerID: ownerID, Fame: fameGain,
			UnlockedFounders: founders, UnlockedItems: items, OfficeSkin: skin,
			HallOfFame: []domain.StartupHallEntry{entry}, CreatedAt: now, UpdatedAt: now,
		}
		_, err = r.studios.InsertOne(ctx, fresh)
		return err
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
