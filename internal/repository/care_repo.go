package repository

import (
	"context"
	"time"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// CareRepository stores TA notes, support-plan items and acknowledgements.
type CareRepository struct{ notes, items, acks *mongo.Collection }

func NewCareRepository(db *mongo.Database) *CareRepository {
	return &CareRepository{notes: db.Collection("admin_notes"), items: db.Collection("support_items"), acks: db.Collection("support_acks")}
}

func (r *CareRepository) EnsureIndexes(ctx context.Context) error {
	idx := mongo.IndexModel{Keys: bson.D{{Key: "user_id", Value: 1}, {Key: "created_at", Value: -1}}}
	if _, err := r.notes.Indexes().CreateOne(ctx, idx); err != nil {
		return err
	}
	if _, err := r.items.Indexes().CreateOne(ctx, idx); err != nil {
		return err
	}
	_, err := r.acks.Indexes().CreateOne(ctx, mongo.IndexModel{Keys: bson.D{{Key: "user_id", Value: 1}, {Key: "until", Value: 1}}})
	return err
}

func newest() *options.FindOptions {
	return options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})
}

func (r *CareRepository) AddNote(ctx context.Context, n *domain.AdminNote) error {
	n.ID = primitive.NewObjectID()
	_, err := r.notes.InsertOne(ctx, n)
	return err
}

func (r *CareRepository) Notes(ctx context.Context, u primitive.ObjectID) ([]domain.AdminNote, error) {
	out := []domain.AdminNote{}
	cur, err := r.notes.Find(ctx, bson.M{"user_id": u}, newest())
	if err != nil {
		return nil, err
	}
	return out, cur.All(ctx, &out)
}

func (r *CareRepository) Note(ctx context.Context, id primitive.ObjectID) (*domain.AdminNote, error) {
	var n domain.AdminNote
	if err := r.notes.FindOne(ctx, bson.M{"_id": id}).Decode(&n); err != nil {
		return nil, err
	}
	return &n, nil
}

func (r *CareRepository) DeleteNote(ctx context.Context, id primitive.ObjectID) error {
	_, err := r.notes.DeleteOne(ctx, bson.M{"_id": id})
	return err
}

func (r *CareRepository) AddItem(ctx context.Context, it *domain.SupportItem) error {
	it.ID = primitive.NewObjectID()
	_, err := r.items.InsertOne(ctx, it)
	return err
}

func (r *CareRepository) Items(ctx context.Context, u primitive.ObjectID) ([]domain.SupportItem, error) {
	out := []domain.SupportItem{}
	cur, err := r.items.Find(ctx, bson.M{"user_id": u}, newest())
	if err != nil {
		return nil, err
	}
	return out, cur.All(ctx, &out)
}

// ItemsFor returns support items for many learners at once.
func (r *CareRepository) ItemsFor(ctx context.Context, ids []primitive.ObjectID) ([]domain.SupportItem, error) {
	out := []domain.SupportItem{}
	cur, err := r.items.Find(ctx, bson.M{"user_id": bson.M{"$in": ids}})
	if err != nil {
		return nil, err
	}
	return out, cur.All(ctx, &out)
}

func (r *CareRepository) SetItemDone(ctx context.Context, id primitive.ObjectID, done bool) error {
	set := bson.M{"done": done, "done_at": nil}
	if done {
		set["done_at"] = time.Now()
	}
	res, err := r.items.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": set})
	if err == nil && res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return err
}

func (r *CareRepository) DeleteItem(ctx context.Context, id primitive.ObjectID) error {
	_, err := r.items.DeleteOne(ctx, bson.M{"_id": id})
	return err
}

func (r *CareRepository) Ack(ctx context.Context, a *domain.SupportAck) error {
	a.ID = primitive.NewObjectID()
	_, err := r.acks.InsertOne(ctx, a)
	return err
}

func (r *CareRepository) Acks(ctx context.Context, u primitive.ObjectID, now time.Time) ([]domain.SupportAck, error) {
	return r.AcksFor(ctx, []primitive.ObjectID{u}, now)
}

// AcksFor returns acknowledgements still in force for many learners.
func (r *CareRepository) AcksFor(ctx context.Context, ids []primitive.ObjectID, now time.Time) ([]domain.SupportAck, error) {
	out := []domain.SupportAck{}
	cur, err := r.acks.Find(ctx, bson.M{"user_id": bson.M{"$in": ids}, "until": bson.M{"$gt": now}})
	if err != nil {
		return nil, err
	}
	return out, cur.All(ctx, &out)
}
