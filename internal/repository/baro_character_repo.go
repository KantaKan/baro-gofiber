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

type BaroCharacterRepository struct {
	characters *mongo.Collection
	users      *mongo.Collection
}

func NewBaroCharacterRepository(db *mongo.Database) *BaroCharacterRepository {
	return &BaroCharacterRepository{characters: db.Collection("baro_characters"), users: db.Collection("users")}
}

func (r *BaroCharacterRepository) EnsureIndexes(ctx context.Context) error {
	_, err := r.characters.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "fingerprint", Value: 1}}, Options: options.Index().SetUnique(true).SetName("unique_character_fingerprint")},
		{Keys: bson.D{{Key: "owner_id", Value: 1}}, Options: options.Index().SetUnique(true).SetName("unique_starter_per_owner").SetPartialFilterExpression(bson.M{"is_starter": true})},
	})
	return err
}

func (r *BaroCharacterRepository) AccountExists(ctx context.Context, ownerID primitive.ObjectID) (bool, error) {
	count, err := r.users.CountDocuments(ctx, bson.M{"_id": ownerID, "deleted": bson.M{"$ne": true}})
	return count > 0, err
}

func (r *BaroCharacterRepository) FindStarter(ctx context.Context, ownerID primitive.ObjectID) (*domain.BaroCharacter, error) {
	var character domain.BaroCharacter
	err := r.characters.FindOne(ctx, bson.M{"owner_id": ownerID, "is_starter": true}).Decode(&character)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &character, nil
}

func (r *BaroCharacterRepository) ListForOwner(ctx context.Context, ownerID primitive.ObjectID) ([]domain.BaroCharacter, error) {
	cursor, err := r.characters.Find(ctx, bson.M{"owner_id": ownerID}, options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	characters := []domain.BaroCharacter{}
	if err := cursor.All(ctx, &characters); err != nil {
		return nil, err
	}
	return characters, nil
}

func (r *BaroCharacterRepository) InsertStarter(ctx context.Context, character domain.BaroCharacter) error {
	return r.InsertCharacter(ctx, character)
}

func (r *BaroCharacterRepository) InsertCharacter(ctx context.Context, character domain.BaroCharacter) error {
	_, err := r.characters.InsertOne(ctx, character)
	if mongo.IsDuplicateKeyError(err) {
		return domain.ErrCharacterConflict
	}
	return err
}

func (r *BaroCharacterRepository) FindOwned(ctx context.Context, ownerID, characterID primitive.ObjectID) (*domain.BaroCharacter, error) {
	var character domain.BaroCharacter
	err := r.characters.FindOne(ctx, bson.M{"_id": characterID, "owner_id": ownerID}).Decode(&character)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &character, nil
}

func (r *BaroCharacterRepository) ReadSelection(ctx context.Context, ownerID primitive.ObjectID) (domain.CharacterSelection, error) {
	var record struct {
		EquippedID primitive.ObjectID `bson:"equipped_character_id"`
		PinnedID   primitive.ObjectID `bson:"pinned_character_id"`
	}
	err := r.users.FindOne(ctx, bson.M{"_id": ownerID, "deleted": bson.M{"$ne": true}}, options.FindOne().SetProjection(bson.M{"equipped_character_id": 1, "pinned_character_id": 1})).Decode(&record)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return domain.CharacterSelection{}, domain.ErrUserNotFound
	}
	if err != nil {
		return domain.CharacterSelection{}, err
	}
	selection := domain.CharacterSelection{}
	if !record.EquippedID.IsZero() {
		selection.EquippedID = record.EquippedID.Hex()
	}
	if !record.PinnedID.IsZero() {
		selection.PinnedID = record.PinnedID.Hex()
	}
	return selection, nil
}

func (r *BaroCharacterRepository) SetEquipped(ctx context.Context, ownerID, characterID primitive.ObjectID) error {
	result, err := r.users.UpdateOne(ctx, bson.M{"_id": ownerID, "deleted": bson.M{"$ne": true}}, bson.M{"$set": bson.M{"equipped_character_id": characterID}})
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return domain.ErrUserNotFound
	}
	return nil
}

func (r *BaroCharacterRepository) SetPinned(ctx context.Context, ownerID primitive.ObjectID, characterID *primitive.ObjectID) error {
	update := bson.M{"$unset": bson.M{"pinned_character_id": ""}}
	if characterID != nil {
		update = bson.M{"$set": bson.M{"pinned_character_id": *characterID}}
	}
	result, err := r.users.UpdateOne(ctx, bson.M{"_id": ownerID, "deleted": bson.M{"$ne": true}}, update)
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return domain.ErrUserNotFound
	}
	return nil
}
