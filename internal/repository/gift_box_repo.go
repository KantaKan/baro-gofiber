package repository

import (
	"context"
	"errors"
	"time"

	"gofiber-baro/internal/domain"
	"gofiber-baro/internal/service/reward"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type GiftBoxRepository struct {
	users *mongo.Collection
}

func NewGiftBoxRepository(db *mongo.Database) *GiftBoxRepository {
	return &GiftBoxRepository{users: db.Collection("users")}
}

func (r *GiftBoxRepository) Create(ctx context.Context, box domain.TeacherGiftBox) error {
	result, err := r.users.UpdateOne(ctx, bson.M{"_id": box.UserID}, bson.M{"$push": bson.M{"gift_boxes": box}})
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return domain.ErrUserNotFound
	}
	return nil
}

func (r *GiftBoxRepository) CreateOnce(ctx context.Context, box domain.TeacherGiftBox) (bool, error) {
	result, err := r.users.UpdateOne(ctx, bson.M{
		"_id":                  box.UserID,
		"gift_boxes.grant_key": bson.M{"$ne": box.GrantKey},
	}, bson.M{"$push": bson.M{"gift_boxes": box}})
	if err != nil {
		return false, err
	}
	if result.ModifiedCount == 1 {
		return true, nil
	}
	count, err := r.users.CountDocuments(ctx, bson.M{"_id": box.UserID, "gift_boxes.grant_key": box.GrantKey})
	if err != nil {
		return false, err
	}
	if count == 1 {
		return false, nil
	}
	return false, domain.ErrUserNotFound
}

func (r *GiftBoxRepository) ListCohortLearners(ctx context.Context, cohort int) ([]primitive.ObjectID, error) {
	cursor, err := r.users.Find(ctx, bson.M{"cohort_number": cohort, "role": "learner", "deleted": bson.M{"$ne": true}})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	ids := []primitive.ObjectID{}
	for cursor.Next(ctx) {
		var user struct {
			ID primitive.ObjectID `bson:"_id"`
		}
		if err := cursor.Decode(&user); err != nil {
			return nil, err
		}
		ids = append(ids, user.ID)
	}
	return ids, cursor.Err()
}

func (r *GiftBoxRepository) ListForUser(ctx context.Context, userID primitive.ObjectID) ([]domain.TeacherGiftBox, error) {
	var result struct {
		GiftBoxes []domain.TeacherGiftBox `bson:"gift_boxes"`
	}
	if err := r.users.FindOne(ctx, bson.M{"_id": userID}).Decode(&result); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, domain.ErrUserNotFound
		}
		return nil, err
	}
	if result.GiftBoxes == nil {
		result.GiftBoxes = []domain.TeacherGiftBox{}
	}
	return result.GiftBoxes, nil
}

func (r *GiftBoxRepository) FindDraw(ctx context.Context, idempotencyKey string) (*reward.DrawResult, error) {
	boxID, err := primitive.ObjectIDFromHex(idempotencyKey)
	if err != nil {
		return nil, nil
	}
	var user domain.User
	if err := r.users.FindOne(ctx, bson.M{"gift_boxes._id": boxID}).Decode(&user); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, err
	}
	for _, box := range user.GiftBoxes {
		if box.ID == boxID && box.Status == "opened" && box.Reward != nil {
			return drawResult(user.ID, box), nil
		}
	}
	return nil, nil
}

func (r *GiftBoxRepository) LoadLearnerState(ctx context.Context, userID string) (reward.LearnerRewardState, error) {
	id, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return reward.LearnerRewardState{}, errors.New("invalid learner ID")
	}
	var user domain.User
	if err := r.users.FindOne(ctx, bson.M{"_id": id}).Decode(&user); err != nil {
		return reward.LearnerRewardState{}, err
	}
	owned := make(map[string]bool, len(user.OwnedCosmeticIDs))
	for _, cosmeticID := range user.OwnedCosmeticIDs {
		owned[cosmeticID] = true
	}
	return reward.LearnerRewardState{Species: user.SelectedSpecies, OwnedIDs: owned}, nil
}

func (r *GiftBoxRepository) CommitDraw(ctx context.Context, result reward.DrawResult) (*reward.DrawResult, error) {
	userID, err := primitive.ObjectIDFromHex(result.UserID)
	if err != nil {
		return nil, errors.New("invalid learner ID")
	}
	boxID, err := primitive.ObjectIDFromHex(result.IdempotencyKey)
	if err != nil {
		return nil, errors.New("invalid gift box ID")
	}
	openedAt := time.Now()
	update := bson.M{
		"$set": bson.M{
			"gift_boxes.$.status":    "opened",
			"gift_boxes.$.reward":    result.Item,
			"gift_boxes.$.opened_at": openedAt,
		},
		"$addToSet": bson.M{
			"owned_cosmetic_ids": result.Item.ID,
			"new_cosmetic_ids":   result.Item.ID,
		},
	}
	updated, err := r.users.UpdateOne(ctx, bson.M{
		"_id":                userID,
		"gift_boxes":         bson.M{"$elemMatch": bson.M{"_id": boxID, "status": "unopened"}},
		"owned_cosmetic_ids": bson.M{"$ne": result.Item.ID},
	}, update)
	if err != nil {
		return nil, err
	}
	if updated.ModifiedCount == 1 {
		result.CreatedAt = openedAt
		return &result, nil
	}
	existing, err := r.FindDraw(ctx, result.IdempotencyKey)
	if err != nil || existing != nil {
		return existing, err
	}
	return nil, errors.New("reward inventory changed; draw can be retried")
}

func drawResult(userID primitive.ObjectID, box domain.TeacherGiftBox) *reward.DrawResult {
	createdAt := box.CreatedAt
	if box.OpenedAt != nil {
		createdAt = *box.OpenedAt
	}
	return &reward.DrawResult{
		IdempotencyKey: box.ID.Hex(),
		UserID:         userID.Hex(),
		Pool:           "teacher-box",
		MinimumRarity:  box.MinimumRarity,
		Item:           *box.Reward,
		CreatedAt:      createdAt,
	}
}
