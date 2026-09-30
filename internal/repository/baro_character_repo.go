package repository

import (
	"context"
	"errors"
	"time"

	"gofiber-baro/internal/domain"
	"gofiber-baro/internal/service/giftbox"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type BaroCharacterRepository struct {
	characters *mongo.Collection
	users      *mongo.Collection
	client     *mongo.Client
}

func NewBaroCharacterRepository(db *mongo.Database) *BaroCharacterRepository {
	return &BaroCharacterRepository{characters: db.Collection("baro_characters"), users: db.Collection("users"), client: db.Client()}
}

func (r *BaroCharacterRepository) EnsureIndexes(ctx context.Context) error {
	_, err := r.characters.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "fingerprint", Value: 1}}, Options: options.Index().SetUnique(true).SetName("unique_character_fingerprint")},
		{Keys: bson.D{{Key: "owner_id", Value: 1}}, Options: options.Index().SetUnique(true).SetName("unique_starter_per_owner").SetPartialFilterExpression(bson.M{"is_starter": true})},
		{Keys: bson.D{{Key: "origin_key", Value: 1}}, Options: options.Index().SetUnique(true).SetName("unique_character_origin").SetPartialFilterExpression(bson.M{"origin_key": bson.M{"$type": "string"}})},
	})
	return err
}

func (r *BaroCharacterRepository) FindCharacterEgg(ctx context.Context, ownerID, eggID primitive.ObjectID) (*domain.BaroCharacter, error) {
	var character domain.BaroCharacter
	err := r.characters.FindOne(ctx, bson.M{"owner_id": ownerID, "origin_key": "character-egg:" + eggID.Hex()}).Decode(&character)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &character, nil
}

func (r *BaroCharacterRepository) CommitCharacterEgg(ctx context.Context, ownerID, eggID primitive.ObjectID, candidate domain.BaroCharacter) (*domain.BaroCharacter, error) {
	session, err := r.client.StartSession()
	if err != nil {
		return nil, err
	}
	defer session.EndSession(ctx)
	result, err := session.WithTransaction(ctx, func(tx mongo.SessionContext) (interface{}, error) {
		var record struct {
			GiftBoxes []domain.TeacherGiftBox `bson:"gift_boxes"`
		}
		filter := bson.M{"_id": ownerID, "role": "learner", "deleted": bson.M{"$ne": true}, "gift_boxes._id": eggID}
		if err := r.users.FindOne(tx, filter, options.FindOne().SetProjection(bson.M{"gift_boxes": 1})).Decode(&record); err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				return nil, giftbox.ErrBoxNotFound
			}
			return nil, err
		}
		var egg *domain.TeacherGiftBox
		for index := range record.GiftBoxes {
			if record.GiftBoxes[index].ID == eggID {
				egg = &record.GiftBoxes[index]
				break
			}
		}
		if egg == nil || egg.UserID != ownerID || egg.RewardPool != giftbox.CharacterEggPool {
			return nil, giftbox.ErrBoxNotFound
		}
		if egg.Status == "opened" && egg.Character != nil {
			return egg.Character, nil
		}
		if egg.Status != "unopened" {
			return nil, giftbox.ErrBoxNotFound
		}
		if _, err := r.characters.InsertOne(tx, candidate); err != nil {
			if mongo.IsDuplicateKeyError(err) {
				return nil, domain.ErrCharacterConflict
			}
			return nil, err
		}
		openedAt := time.Now().UTC()
		updated, err := r.users.UpdateOne(tx, bson.M{
			"_id":        ownerID,
			"gift_boxes": bson.M{"$elemMatch": bson.M{"_id": eggID, "status": "unopened", "reward_pool": giftbox.CharacterEggPool}},
		}, bson.M{"$set": bson.M{
			"gift_boxes.$.status": "opened", "gift_boxes.$.character": candidate, "gift_boxes.$.opened_at": openedAt,
		}})
		if err != nil {
			return nil, err
		}
		if updated.ModifiedCount != 1 {
			return nil, giftbox.ErrBoxNotFound
		}
		return &candidate, nil
	})
	if err != nil {
		return nil, err
	}
	character, ok := result.(*domain.BaroCharacter)
	if !ok {
		return nil, errors.New("character egg did not return a character")
	}
	return character, nil
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
