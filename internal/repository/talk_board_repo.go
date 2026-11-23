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

// AddReaction sets the caller's one reaction on a post: added if they have none, else replaced.
// The add only matches when they have no reaction yet, so two quick taps can't leave two.
func (r *talkBoardRepository) AddReaction(ctx context.Context, postID primitive.ObjectID, reaction domain.Reaction) error {
	if _, err := r.collection.UpdateOne(ctx, bson.M{"_id": postID, "reactions": nil}, bson.M{"$set": bson.M{"reactions": bson.A{}}}); err != nil {
		return err
	}
	f, u := postReactionAddIfNone(postID, reaction)
	res, err := r.collection.UpdateOne(ctx, f, u)
	if err != nil || res.MatchedCount > 0 {
		return err
	}
	f, u, filters := postReactionReplace(postID, reaction)
	_, err = r.collection.UpdateOne(ctx, f, u, options.Update().SetArrayFilters(options.ArrayFilters{Filters: filters}))
	return err
}

func postReactionAddIfNone(postID primitive.ObjectID, r domain.Reaction) (bson.M, bson.M) {
	return bson.M{"_id": postID, "reactions.userId": bson.M{"$ne": r.UserID}}, bson.M{"$push": bson.M{"reactions": r}}
}

func postReactionReplace(postID primitive.ObjectID, r domain.Reaction) (bson.M, bson.M, []interface{}) {
	return bson.M{"_id": postID}, bson.M{"$set": bson.M{"reactions.$[r]": r}}, []interface{}{bson.M{"r.userId": r.UserID}}
}

func commentReactionAddIfNone(postID, commentID primitive.ObjectID, r domain.Reaction) (bson.M, bson.M) {
	return bson.M{"_id": postID, "comments": bson.M{"$elemMatch": bson.M{"_id": commentID, "reactions.userId": bson.M{"$ne": r.UserID}}}},
		bson.M{"$push": bson.M{"comments.$.reactions": r}}
}

func commentReactionReplace(postID, commentID primitive.ObjectID, r domain.Reaction) (bson.M, bson.M, []interface{}) {
	return bson.M{"_id": postID}, bson.M{"$set": bson.M{"comments.$[c].reactions.$[r]": r}}, []interface{}{bson.M{"c._id": commentID}, bson.M{"r.userId": r.UserID}}
}

// nullReplyReactionsRepair turns a reply's null reactions (older replies) into an empty list.
func nullReplyReactionsRepair(commentID primitive.ObjectID) (bson.M, []interface{}) {
	return bson.M{"$set": bson.M{"comments.$[c].reactions": bson.A{}}}, []interface{}{bson.M{"c._id": commentID, "c.reactions": nil}}
}

// AddCommentReaction sets the caller's one reaction on a reply, like AddReaction.
func (r *talkBoardRepository) AddCommentReaction(ctx context.Context, postID primitive.ObjectID, commentID primitive.ObjectID, reaction domain.Reaction) error {
	repair, rf := nullReplyReactionsRepair(commentID)
	if _, err := r.collection.UpdateOne(ctx, bson.M{"_id": postID}, repair, options.Update().SetArrayFilters(options.ArrayFilters{Filters: rf})); err != nil {
		log.Printf("ERROR: AddCommentReaction repair: %v", err)
		return err
	}
	f, u := commentReactionAddIfNone(postID, commentID, reaction)
	res, err := r.collection.UpdateOne(ctx, f, u)
	if err != nil || res.MatchedCount > 0 {
		return err
	}
	f, u, filters := commentReactionReplace(postID, commentID, reaction)
	_, err = r.collection.UpdateOne(ctx, f, u, options.Update().SetArrayFilters(options.ArrayFilters{Filters: filters}))
	return err
}

func (r *talkBoardRepository) RemoveCommentReaction(ctx context.Context, postID, commentID, userID primitive.ObjectID) error {
	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"_id": postID},
		bson.M{"$pull": bson.M{"comments.$[c].reactions": bson.M{"userId": userID}}},
		options.Update().SetArrayFilters(options.ArrayFilters{
			Filters: []interface{}{bson.M{"c._id": commentID}},
		}),
	)
	return err
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
