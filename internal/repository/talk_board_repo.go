package repository

import (
	"context"
	"log"

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
	_, err := r.collection.UpdateOne(ctx, bson.M{"_id": id}, update)
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
	// First ensure reactions field exists as an array (not null)
	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"_id": postID, "reactions": nil},
		bson.M{"$set": bson.M{"reactions": []domain.Reaction{}}},
	)
	if err != nil {
		return err
	}

	// Now push the reaction
	_, err = r.collection.UpdateOne(
		ctx,
		bson.M{"_id": postID},
		bson.M{"$push": bson.M{"reactions": reaction}},
	)
	return err
}

func (r *talkBoardRepository) AddCommentReaction(ctx context.Context, postID primitive.ObjectID, commentID primitive.ObjectID, reaction domain.Reaction) error {
	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"_id": postID},
		bson.M{"$push": bson.M{"comments.$[c].reactions": reaction}},
		options.Update().SetArrayFilters(options.ArrayFilters{
			Filters: []interface{}{bson.M{"c._id": commentID}},
		}),
	)
	if err != nil {
		log.Printf("ERROR: AddCommentReaction: %v", err)
		return err
	}
	return nil
}

func (r *talkBoardRepository) DeleteComment(ctx context.Context, postID primitive.ObjectID, commentID primitive.ObjectID) error {
	filter := bson.M{"_id": postID}
	update := bson.M{"$pull": bson.M{"comments": bson.M{"_id": commentID}}}
	_, err := r.collection.UpdateOne(ctx, filter, update)
	return err
}

func (r *talkBoardRepository) Exists(ctx context.Context, id primitive.ObjectID) (bool, error) {
	count, err := r.collection.CountDocuments(ctx, bson.M{"_id": id})
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
