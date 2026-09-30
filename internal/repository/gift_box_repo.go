package repository

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"gofiber-baro/internal/domain"
	"gofiber-baro/internal/service/giftbox"
	"gofiber-baro/internal/service/reward"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type GiftBoxRepository struct {
	users         *mongo.Collection
	notifications *mongo.Collection
	client        *mongo.Client
}

func NewGiftBoxRepository(db *mongo.Database) *GiftBoxRepository {
	return &GiftBoxRepository{users: db.Collection("users"), notifications: db.Collection("notifications"), client: db.Client()}
}

func (r *GiftBoxRepository) CreateCharacterEgg(ctx context.Context, box domain.TeacherGiftBox) error {
	return r.withCharacterEggTransaction(ctx, box, false)
}

func (r *GiftBoxRepository) CreateCharacterEggOnce(ctx context.Context, box domain.TeacherGiftBox) (bool, error) {
	err := r.withCharacterEggTransaction(ctx, box, true)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, errCharacterEggAlreadyGranted) {
		return false, nil
	}
	return false, err
}

var errCharacterEggAlreadyGranted = errors.New("character egg already granted")

func (r *GiftBoxRepository) withCharacterEggTransaction(ctx context.Context, box domain.TeacherGiftBox, idempotent bool) error {
	session, err := r.client.StartSession()
	if err != nil {
		return err
	}
	defer session.EndSession(ctx)
	_, err = session.WithTransaction(ctx, func(tx mongo.SessionContext) (interface{}, error) {
		filter := bson.M{"_id": box.UserID, "role": "learner", "deleted": bson.M{"$ne": true}}
		if idempotent {
			filter["gift_boxes.grant_key"] = bson.M{"$ne": box.GrantKey}
		}
		result, updateErr := r.users.UpdateOne(tx, filter, bson.M{"$push": bson.M{"gift_boxes": box}})
		if updateErr != nil {
			return nil, updateErr
		}
		if result.ModifiedCount != 1 {
			if idempotent {
				count, countErr := r.users.CountDocuments(tx, bson.M{"_id": box.UserID, "gift_boxes.grant_key": box.GrantKey})
				if countErr != nil {
					return nil, countErr
				}
				if count == 1 {
					return nil, errCharacterEggAlreadyGranted
				}
			}
			return nil, domain.ErrUserNotFound
		}
		now := time.Now()
		notification := domain.Notification{
			ID: primitive.NewObjectID(), Title: "A Character Egg arrived", Message: "A mystery friend is waiting for you to hatch it.",
			Link: "/learner", LinkText: "Open Character Eggs", IsActive: true, Priority: "normal",
			StartDate: now, EndDate: now.AddDate(1, 0, 0), CreatedAt: now, ReadByUsers: []primitive.ObjectID{}, RecipientIDs: []primitive.ObjectID{box.UserID},
		}
		if _, insertErr := r.notifications.InsertOne(tx, notification); insertErr != nil {
			return nil, insertErr
		}
		return nil, nil
	})
	return err
}

func giftBoxDisplayName(user domain.User) string {
	name := strings.TrimSpace(user.FirstName + " " + user.LastName)
	if name == "" {
		name = strings.TrimSpace(user.ZoomName)
	}
	if name == "" {
		name = user.ID.Hex()
	}
	return name
}

func (r *GiftBoxRepository) SearchRecipients(ctx context.Context, query string, exclude primitive.ObjectID) ([]giftbox.Recipient, error) {
	pattern := primitive.Regex{Pattern: regexp.QuoteMeta(query), Options: "i"}
	filter := bson.M{
		"_id": bson.M{"$ne": exclude}, "role": "learner", "deleted": bson.M{"$ne": true},
		"$or": []bson.M{{"first_name": pattern}, {"last_name": pattern}, {"zoom_name": pattern}, {"email": pattern}},
	}
	projection := bson.M{"first_name": 1, "last_name": 1, "zoom_name": 1, "cohort_number": 1, "genmate_group": 1, "role": 1}
	cursor, err := r.users.Find(ctx, filter, options.Find().SetProjection(projection).SetSort(bson.D{{Key: "first_name", Value: 1}}).SetLimit(20))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	items := []giftbox.Recipient{}
	for cursor.Next(ctx) {
		var user domain.User
		if err := cursor.Decode(&user); err != nil {
			return nil, err
		}
		items = append(items, giftbox.Recipient{ID: user.ID.Hex(), DisplayName: giftBoxDisplayName(user), CohortNumber: user.CohortNumber, Group: user.GenmateGroup, Role: user.Role})
	}
	return items, cursor.Err()
}

func (r *GiftBoxRepository) IsLearner(ctx context.Context, userID primitive.ObjectID) (bool, error) {
	err := r.users.FindOne(ctx, bson.M{"_id": userID, "role": "learner", "deleted": bson.M{"$ne": true}}, options.FindOne().SetProjection(bson.M{"_id": 1})).Err()
	if errors.Is(err, mongo.ErrNoDocuments) {
		return false, nil
	}
	return err == nil, err
}

func (r *GiftBoxRepository) Transfer(ctx context.Context, boxID, fromID, toID primitive.ObjectID) (*domain.TeacherGiftBox, error) {
	session, err := r.client.StartSession()
	if err != nil {
		return nil, err
	}
	defer session.EndSession(ctx)
	result, err := session.WithTransaction(ctx, func(tx mongo.SessionContext) (interface{}, error) {
		active := bson.M{"$elemMatch": bson.M{"_id": boxID, "status": "unopened"}}
		nameProjection := bson.M{"first_name": 1, "last_name": 1, "zoom_name": 1}
		senderProjection := bson.M{"first_name": 1, "last_name": 1, "zoom_name": 1, "gift_boxes": 1}
		var sender domain.User
		if err := r.users.FindOne(tx, bson.M{"_id": fromID, "deleted": bson.M{"$ne": true}, "gift_boxes": active}, options.FindOne().SetProjection(senderProjection)).Decode(&sender); err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				return nil, giftbox.ErrBoxNotFound
			}
			return nil, err
		}
		var recipient domain.User
		if err := r.users.FindOne(tx, bson.M{"_id": toID, "role": "learner", "deleted": bson.M{"$ne": true}}, options.FindOne().SetProjection(nameProjection)).Decode(&recipient); err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				return nil, giftbox.ErrRecipientNotFound
			}
			return nil, err
		}
		var box *domain.TeacherGiftBox
		for index := range sender.GiftBoxes {
			if sender.GiftBoxes[index].ID == boxID && sender.GiftBoxes[index].Status == "unopened" {
				copy := sender.GiftBoxes[index]
				box = &copy
				break
			}
		}
		if box == nil {
			return nil, giftbox.ErrBoxNotFound
		}
		box.UserID = toID
		box.TransferHistory = append(box.TransferHistory, domain.GiftBoxTransfer{
			FromID: fromID, FromName: giftBoxDisplayName(sender), ToID: toID,
			ToName: giftBoxDisplayName(recipient), TransferredAt: time.Now(),
		})
		removed, err := r.users.UpdateOne(tx, bson.M{"_id": fromID, "gift_boxes": active}, bson.M{"$pull": bson.M{"gift_boxes": bson.M{"_id": boxID}}})
		if err != nil {
			return nil, err
		}
		if removed.ModifiedCount != 1 {
			return nil, giftbox.ErrBoxNotFound
		}
		added, err := r.users.UpdateOne(tx, bson.M{"_id": toID, "role": "learner", "deleted": bson.M{"$ne": true}, "gift_boxes._id": bson.M{"$ne": boxID}}, bson.M{"$push": bson.M{"gift_boxes": box}})
		if err != nil {
			return nil, err
		}
		if added.ModifiedCount != 1 {
			return nil, giftbox.ErrRecipientNotFound
		}
		return box, nil
	})
	if err != nil {
		return nil, err
	}
	box, ok := result.(*domain.TeacherGiftBox)
	if !ok {
		return nil, errors.New("transfer did not return a gift box")
	}
	return box, nil
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
	return r.listAudienceLearners(ctx, bson.M{"cohort_number": cohort, "role": "learner", "deleted": bson.M{"$ne": true}})
}

func (r *GiftBoxRepository) ListTeamLearners(ctx context.Context, cohort int, team string) ([]primitive.ObjectID, error) {
	return r.listAudienceLearners(ctx, bson.M{"cohort_number": cohort, "genmate_group": team, "role": "learner", "deleted": bson.M{"$ne": true}})
}

func (r *GiftBoxRepository) listAudienceLearners(ctx context.Context, filter bson.M) ([]primitive.ObjectID, error) {
	cursor, err := r.users.Find(ctx, filter, options.Find().SetProjection(bson.M{"_id": 1}))
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
	return nil, reward.ErrInventoryChanged
}

func drawResult(userID primitive.ObjectID, box domain.TeacherGiftBox) *reward.DrawResult {
	createdAt := box.CreatedAt
	if box.OpenedAt != nil {
		createdAt = *box.OpenedAt
	}
	return &reward.DrawResult{
		IdempotencyKey: box.ID.Hex(),
		UserID:         userID.Hex(),
		Pool:           giftBoxPool(box.Source, box.RewardPool),
		MinimumRarity:  box.MinimumRarity,
		Item:           *box.Reward,
		CreatedAt:      createdAt,
	}
}

func giftBoxPool(source, rewardPool string) string {
	if rewardPool == "character-box" {
		return rewardPool
	}
	switch source {
	case "achievement":
		return "achievement"
	case "reflection-milestone":
		return "reflection"
	default:
		return "teacher-box"
	}
}
