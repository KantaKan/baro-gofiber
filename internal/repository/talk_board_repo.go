package repository

import (
	"context"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type talkBoardRepository struct {
	collection *mongo.Collection
}

func NewTalkBoardRepository(db *mongo.Database) domain.TalkBoardRepository {
	return &talkBoardRepository{
		collection: db.Collection("talk_board"),
	}
}

func (r *talkBoardRepository) InsertPost(ctx context.Context, post *domain.Post) error {
	post.ID = primitive.NewObjectID()
	_, err := r.collection.InsertOne(ctx, post)
	return err
}

func (r *talkBoardRepository) FindPosts(ctx context.Context, filter domain.PostFilter, opts interface{}) ([]domain.Post, error) {
	bsonFilter := bson.M{}
	if filter.Cohort > 0 {
		bsonFilter["cohort"] = filter.Cohort
	}

	findOpts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}})
	cursor, err := r.collection.Find(ctx, bsonFilter, findOpts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var posts []domain.Post
	if err := cursor.All(ctx, &posts); err != nil {
		return nil, err
	}
	return posts, nil
}

func (r *talkBoardRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*domain.Post, error) {
	var post domain.Post
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&post)
	if err != nil {
		return nil, err
	}
	return &post, nil
}

func (r *talkBoardRepository) UpdatePost(ctx context.Context, id primitive.ObjectID, update interface{}) error {
	_, err := r.collection.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": update})
	return err
}

func (r *talkBoardRepository) DeletePost(ctx context.Context, id primitive.ObjectID) error {
	_, err := r.collection.DeleteOne(ctx, bson.M{"_id": id})
	return err
}

func (r *talkBoardRepository) AddComment(ctx context.Context, postID primitive.ObjectID, comment domain.Comment) error {
	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"_id": postID},
		bson.M{"$push": bson.M{"comments": comment}},
	)
	return err
}

func (r *talkBoardRepository) AddReaction(ctx context.Context, postID primitive.ObjectID, reaction domain.Reaction) error {
	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"_id": postID},
		bson.M{"$push": bson.M{"reactions": reaction}},
	)
	return err
}
