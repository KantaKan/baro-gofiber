package chess

import (
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MongoStore keeps games in chess_games and chat in chess_messages.
type MongoStore struct {
	games, messages, settings *mongo.Collection
}

func NewMongoStore(db *mongo.Database) *MongoStore {
	return &MongoStore{games: db.Collection("chess_games"), messages: db.Collection("chess_messages"), settings: db.Collection("cohort_settings")}
}

func (m *MongoStore) EnsureIndexes(ctx context.Context) error {
	if _, err := m.games.Indexes().CreateOne(ctx, mongo.IndexModel{Keys: bson.D{{Key: "cohort_number", Value: 1}, {Key: "status", Value: 1}}}); err != nil {
		return err
	}
	_, err := m.messages.Indexes().CreateOne(ctx, mongo.IndexModel{Keys: bson.D{{Key: "game_id", Value: 1}, {Key: "room", Value: 1}, {Key: "at", Value: 1}}})
	return err
}

func (m *MongoStore) Active(ctx context.Context, cohort int) (*Game, error) {
	var g Game
	err := m.games.FindOne(ctx, bson.M{"cohort_number": cohort, "status": StatusActive}).Decode(&g)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	return &g, err
}

func (m *MongoStore) AllActive(ctx context.Context) ([]Game, error) {
	cur, err := m.games.Find(ctx, bson.M{"status": StatusActive})
	if err != nil {
		return nil, err
	}
	var out []Game
	return out, cur.All(ctx, &out)
}

func (m *MongoStore) Insert(ctx context.Context, g *Game) error {
	_, err := m.games.InsertOne(ctx, g)
	return err
}

// Save replaces the game only if nobody wrote since version was read.
func (m *MongoStore) Save(ctx context.Context, g *Game, version int) (bool, error) {
	res, err := m.games.ReplaceOne(ctx, bson.M{"_id": g.ID, "version": version}, g)
	if err != nil {
		return false, err
	}
	return res.MatchedCount == 1, nil
}

func (m *MongoStore) AddMessage(ctx context.Context, msg Message) error {
	_, err := m.messages.InsertOne(ctx, msg)
	return err
}

// Messages returns up to limit messages after the cursor, oldest first; an unknown cursor gives the latest page.
func (m *MongoStore) Messages(ctx context.Context, gameID, room, after string, limit int) ([]Message, error) {
	filter := bson.M{"game_id": gameID, "room": room}
	if after != "" {
		var cur Message
		if err := m.messages.FindOne(ctx, bson.M{"_id": after, "game_id": gameID, "room": room}).Decode(&cur); err == nil {
			filter["at"] = bson.M{"$gt": cur.At}
			c, err := m.messages.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "at", Value: 1}}).SetLimit(int64(limit)))
			if err != nil {
				return nil, err
			}
			out := []Message{}
			return out, c.All(ctx, &out)
		}
	}
	c, err := m.messages.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "at", Value: -1}}).SetLimit(int64(limit)))
	if err != nil {
		return nil, err
	}
	out := []Message{}
	if err := c.All(ctx, &out); err != nil {
		return nil, err
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

// EnabledCohorts lists cohorts whose settings turn team chess on.
func (m *MongoStore) EnabledCohorts(ctx context.Context) []int {
	cur, err := m.settings.Find(ctx, bson.M{"chess_enabled": true}, options.Find().SetProjection(bson.M{"cohort_number": 1}))
	if err != nil {
		return nil
	}
	var rows []struct {
		Cohort int `bson:"cohort_number"`
	}
	if cur.All(ctx, &rows) != nil {
		return nil
	}
	out := make([]int, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Cohort)
	}
	return out
}
