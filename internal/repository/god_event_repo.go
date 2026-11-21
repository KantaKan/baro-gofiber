package repository

import (
	"context"
	"errors"

	"gofiber-baro/internal/domain"
	"gofiber-baro/internal/service/godevent"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type GodEventRepository struct {
	events     *mongo.Collection
	users      *mongo.Collection
	characters *mongo.Collection
	cohorts    *mongo.Collection
}

func NewGodEventRepository(db *mongo.Database) *GodEventRepository {
	return &GodEventRepository{events: db.Collection("god_events"), users: db.Collection("users"), characters: db.Collection("baro_characters"), cohorts: db.Collection("cohorts")}
}

func (r *GodEventRepository) EnsureIndexes(ctx context.Context) error {
	_, err := r.events.Indexes().CreateOne(ctx, mongo.IndexModel{Keys: bson.D{{Key: "cohort", Value: 1}, {Key: "created_at", Value: -1}}, Options: options.Index().SetName("god_event_audience_history")})
	return err
}

func (r *GodEventRepository) Viewer(ctx context.Context, id primitive.ObjectID) (*godevent.Viewer, error) {
	var row struct {
		Role   string `bson:"role"`
		Cohort int    `bson:"cohort_number"`
	}
	err := r.users.FindOne(ctx, bson.M{"_id": id, "deleted": bson.M{"$ne": true}}, options.FindOne().SetProjection(bson.M{"role": 1, "cohort_number": 1})).Decode(&row)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &godevent.Viewer{Role: row.Role, Cohort: row.Cohort}, nil
}

func (r *GodEventRepository) CohortExists(ctx context.Context, cohort int) (bool, error) {
	count, err := r.cohorts.CountDocuments(ctx, bson.M{"cohort_number": cohort})
	return count > 0, err
}

func (r *GodEventRepository) AdminCharacter(ctx context.Context, adminID primitive.ObjectID) (*domain.BaroCharacter, error) {
	var user struct {
		EquippedID primitive.ObjectID `bson:"equipped_character_id"`
	}
	if err := r.users.FindOne(ctx, bson.M{"_id": adminID, "deleted": bson.M{"$ne": true}}, options.FindOne().SetProjection(bson.M{"equipped_character_id": 1})).Decode(&user); err != nil {
		return nil, err
	}
	filter := bson.M{"owner_id": adminID, "is_starter": true}
	if !user.EquippedID.IsZero() {
		filter = bson.M{"_id": user.EquippedID, "owner_id": adminID}
	}
	var character domain.BaroCharacter
	err := r.characters.FindOne(ctx, filter).Decode(&character)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &character, nil
}

func (r *GodEventRepository) Create(ctx context.Context, event domain.GodEvent) error {
	_, err := r.events.InsertOne(ctx, event)
	return err
}

func (r *GodEventRepository) ListVisible(ctx context.Context, viewer godevent.Viewer) ([]domain.GodEvent, error) {
	filter := bson.M{}
	if viewer.Role != "admin" {
		filter["cohort"] = bson.M{"$in": []int{0, viewer.Cohort}}
	}
	cursor, err := r.events.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}, {Key: "_id", Value: -1}}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	events := []domain.GodEvent{}
	if err := cursor.All(ctx, &events); err != nil {
		return nil, err
	}
	return events, nil
}
