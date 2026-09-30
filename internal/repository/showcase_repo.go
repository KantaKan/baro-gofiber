package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"gofiber-baro/internal/domain"
	"gofiber-baro/internal/service/showcase"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type ShowcaseRepository struct {
	users      *mongo.Collection
	characters *mongo.Collection
}

type showcaseReaction struct {
	ActorID primitive.ObjectID `bson:"actor_id"`
	Emoji   string             `bson:"emoji"`
}

func NewShowcaseRepository(db *mongo.Database) *ShowcaseRepository {
	return &ShowcaseRepository{users: db.Collection("users"), characters: db.Collection("baro_characters")}
}

func (r *ShowcaseRepository) Viewer(ctx context.Context, id primitive.ObjectID) (*showcase.Viewer, error) {
	var user struct {
		Cohort int    `bson:"cohort_number"`
		Role   string `bson:"role"`
	}
	err := r.users.FindOne(ctx, bson.M{"_id": id, "deleted": bson.M{"$ne": true}}, options.FindOne().SetProjection(bson.M{"cohort_number": 1, "role": 1})).Decode(&user)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &showcase.Viewer{Cohort: user.Cohort, Role: user.Role}, nil
}

func (r *ShowcaseRepository) FindOwned(ctx context.Context, ownerID, characterID primitive.ObjectID) (*domain.BaroCharacter, error) {
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

func (r *ShowcaseRepository) Save(ctx context.Context, ownerID primitive.ObjectID, characterID *primitive.ObjectID, message string, at time.Time) error {
	update := bson.M{"$unset": bson.M{"pinned_character_id": "", "showcase_message": "", "showcase_updated_at": "", "showcase_reactions": ""}}
	if characterID != nil {
		update = bson.M{"$set": bson.M{"pinned_character_id": *characterID, "showcase_message": message, "showcase_updated_at": at}}
	}
	result, err := r.users.UpdateOne(ctx, bson.M{"_id": ownerID, "deleted": bson.M{"$ne": true}}, update)
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return showcase.ErrAccountNotFound
	}
	return nil
}

func (r *ShowcaseRepository) List(ctx context.Context, cohort int, team string, includeHidden bool, viewerID primitive.ObjectID) ([]showcase.Entry, error) {
	return r.list(ctx, cohort, team, includeHidden, viewerID, nil)
}

func (r *ShowcaseRepository) Own(ctx context.Context, ownerID primitive.ObjectID) (*showcase.Entry, error) {
	entries, err := r.list(ctx, 0, "", true, ownerID, &ownerID)
	if err != nil || len(entries) == 0 {
		return nil, err
	}
	return &entries[0], nil
}

func (r *ShowcaseRepository) list(ctx context.Context, cohort int, team string, includeHidden bool, viewerID primitive.ObjectID, ownerFilter *primitive.ObjectID) ([]showcase.Entry, error) {
	filter := bson.M{"deleted": bson.M{"$ne": true}, "pinned_character_id": bson.M{"$exists": true, "$ne": nil}}
	if ownerFilter != nil {
		filter["_id"] = *ownerFilter
	}
	if !includeHidden {
		filter["showcase_hidden"] = bson.M{"$ne": true}
	}
	if cohort > 0 {
		filter["cohort_number"] = cohort
	}
	if team != "" {
		filter["genmate_group"] = team
	}
	projection := bson.M{"first_name": 1, "zoom_name": 1, "cohort_number": 1, "genmate_group": 1, "pinned_character_id": 1, "showcase_message": 1, "showcase_updated_at": 1, "showcase_hidden": 1, "showcase_reactions": 1, "equipped_cosmetics.character_prop": 1}
	cursor, err := r.users.Find(ctx, filter, options.Find().SetProjection(projection).SetSort(bson.D{{Key: "showcase_updated_at", Value: -1}, {Key: "_id", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var users []struct {
		ID        primitive.ObjectID `bson:"_id"`
		FirstName string             `bson:"first_name"`
		ZoomName  string             `bson:"zoom_name"`
		Cohort    int                `bson:"cohort_number"`
		Team      string             `bson:"genmate_group"`
		PinnedID  primitive.ObjectID `bson:"pinned_character_id"`
		Message   string             `bson:"showcase_message"`
		UpdatedAt time.Time          `bson:"showcase_updated_at"`
		Hidden    bool               `bson:"showcase_hidden"`
		Reactions []showcaseReaction `bson:"showcase_reactions"`
		Equipped  map[string]string  `bson:"equipped_cosmetics"`
	}
	if err := cursor.All(ctx, &users); err != nil {
		return nil, err
	}
	ids := make([]primitive.ObjectID, 0, len(users))
	for _, user := range users {
		if !user.PinnedID.IsZero() {
			ids = append(ids, user.PinnedID)
		}
	}
	entries := []showcase.Entry{}
	if len(ids) == 0 {
		return entries, nil
	}
	characters, err := r.characters.Find(ctx, bson.M{"_id": bson.M{"$in": ids}})
	if err != nil {
		return nil, err
	}
	defer characters.Close(ctx)
	byID := map[primitive.ObjectID]domain.BaroCharacter{}
	for characters.Next(ctx) {
		var character domain.BaroCharacter
		if err := characters.Decode(&character); err != nil {
			return nil, err
		}
		byID[character.ID] = character
	}
	if err := characters.Err(); err != nil {
		return nil, err
	}
	for _, user := range users {
		character, ok := byID[user.PinnedID]
		if !ok || character.OwnerID != user.ID {
			continue
		}
		counts := map[string]int{}
		reacted := map[string]bool{}
		for _, reaction := range user.Reactions {
			counts[reaction.Emoji]++
			if reaction.ActorID == viewerID {
				reacted[reaction.Emoji] = true
			}
		}
		summaries := make([]showcase.ReactionSummary, 0, 4)
		for _, emoji := range []string{"❤️", "✨", "😂", "🙌"} {
			summaries = append(summaries, showcase.ReactionSummary{Emoji: emoji, Count: counts[emoji], Reacted: reacted[emoji]})
		}
		entries = append(entries, showcase.Entry{OwnerID: user.ID.Hex(), Name: showcaseDisplayName(user.ZoomName, user.FirstName), Cohort: user.Cohort, Team: user.Team, Character: character, Prop: showcaseProp(user.Equipped["character_prop"]), Message: user.Message, UpdatedAt: user.UpdatedAt, Hidden: user.Hidden, Reactions: summaries})
	}
	return entries, nil
}

func showcaseDisplayName(zoomName, firstName string) string {
	for _, name := range []string{zoomName, firstName} {
		if name = strings.TrimSpace(name); name != "" {
			return name
		}
	}
	return "Baro friend"
}

func showcaseProp(equippedID string) string {
	switch equippedID {
	case "character_prop:flower", "character_prop:cat-ears", "character_prop:egg", "character_prop:halo", "character_prop:headphones", "character_prop:pixel-glasses", "character_prop:tiny-crown":
		return strings.TrimPrefix(equippedID, "character_prop:")
	default:
		return ""
	}
}

func (r *ShowcaseRepository) ReactionTarget(ctx context.Context, ownerID primitive.ObjectID) (*showcase.Target, error) {
	var row struct {
		Cohort int  `bson:"cohort_number"`
		Hidden bool `bson:"showcase_hidden"`
	}
	err := r.users.FindOne(ctx, bson.M{"_id": ownerID, "deleted": bson.M{"$ne": true}, "pinned_character_id": bson.M{"$exists": true, "$ne": nil}}, options.FindOne().SetProjection(bson.M{"cohort_number": 1, "showcase_hidden": 1})).Decode(&row)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &showcase.Target{Cohort: row.Cohort, Hidden: row.Hidden}, nil
}

func (r *ShowcaseRepository) ToggleReaction(ctx context.Context, ownerID, actorID primitive.ObjectID, emoji string) (bool, error) {
	reaction := bson.M{"actor_id": actorID, "emoji": emoji}
	visible := bson.M{"_id": ownerID, "deleted": bson.M{"$ne": true}, "showcase_hidden": bson.M{"$ne": true}, "pinned_character_id": bson.M{"$exists": true, "$ne": nil}}
	removeFilter := bson.M{}
	for key, value := range visible {
		removeFilter[key] = value
	}
	removeFilter["showcase_reactions"] = bson.M{"$elemMatch": reaction}
	removed, err := r.users.UpdateOne(ctx, removeFilter, bson.M{"$pull": bson.M{"showcase_reactions": reaction}})
	if err != nil {
		return false, err
	}
	if removed.ModifiedCount == 1 {
		return false, nil
	}
	addFilter := bson.M{}
	for key, value := range visible {
		addFilter[key] = value
	}
	addFilter["$nor"] = []bson.M{{"showcase_reactions": bson.M{"$elemMatch": reaction}}}
	added, err := r.users.UpdateOne(ctx, addFilter, bson.M{"$addToSet": bson.M{"showcase_reactions": showcaseReaction{ActorID: actorID, Emoji: emoji}}})
	if err != nil {
		return false, err
	}
	if added.MatchedCount == 0 {
		count, countErr := r.users.CountDocuments(ctx, visible)
		if countErr != nil {
			return false, countErr
		}
		if count == 0 {
			return false, showcase.ErrEntryNotFound
		}
	}
	return true, nil
}

func (r *ShowcaseRepository) Moderate(ctx context.Context, ownerID, adminID primitive.ObjectID, hidden bool, reason string, at time.Time) error {
	filter := bson.M{"_id": ownerID, "deleted": bson.M{"$ne": true}, "pinned_character_id": bson.M{"$exists": true, "$ne": nil}}
	if hidden {
		filter["showcase_hidden"] = bson.M{"$ne": true}
	} else {
		filter["showcase_hidden"] = true
	}
	event := bson.M{"actor_id": adminID, "hidden": hidden, "reason": reason, "at": at}
	result, err := r.users.UpdateOne(ctx, filter, bson.M{"$set": bson.M{"showcase_hidden": hidden}, "$push": bson.M{"showcase_moderation": event}})
	if err != nil {
		return err
	}
	if result.ModifiedCount == 1 {
		return nil
	}
	exists, err := r.users.CountDocuments(ctx, bson.M{"_id": ownerID, "deleted": bson.M{"$ne": true}, "pinned_character_id": bson.M{"$exists": true, "$ne": nil}})
	if err != nil {
		return err
	}
	if exists == 0 {
		return showcase.ErrEntryNotFound
	}
	return nil
}
