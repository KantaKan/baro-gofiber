package services

import (
	"context"
	"errors"
	"gofiber-baro/config"
	"gofiber-baro/models"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var talkBoardCollection *mongo.Collection

func InitTalkBoardService() {
	if config.DB != nil {
		talkBoardCollection = config.DB.Collection("talk_board")
	} else {
		log.Fatal("Failed to initialize talk board service: database connection is nil")
	}
}

func CreatePost(post *models.Post) (*models.Post, error) {
	post.ID = primitive.NewObjectID()
	post.CreatedAt = time.Now()
	post.UpdatedAt = time.Now()
	post.Reactions = []models.Reaction{}
	post.Comments = []models.Comment{}

	_, err := talkBoardCollection.InsertOne(context.Background(), post)
	if err != nil {
		return nil, err
	}
	return post, nil
}

func GetPosts() ([]models.Post, error) {
	var posts []models.Post
	opts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}})
	cursor, err := talkBoardCollection.Find(context.Background(), bson.M{}, opts)
	if err != nil {
		return nil, err
	}
	if err = cursor.All(context.Background(), &posts); err != nil {
		return nil, err
	}
	return posts, nil
}

func GetPostByID(postID primitive.ObjectID) (*models.Post, error) {
	var post models.Post
	err := talkBoardCollection.FindOne(context.Background(), bson.M{"_id": postID}).Decode(&post)
	if err != nil {
		return nil, err
	}
	return &post, nil
}

func AddCommentToPost(postId primitive.ObjectID, comment *models.Comment) (*models.Post, error) {
	comment.ID = primitive.NewObjectID()
	comment.CreatedAt = time.Now()
	comment.UpdatedAt = time.Now()
	comment.Reactions = []models.Reaction{}

	update := bson.M{
		"$push": bson.M{"comments": comment},
		"$set":  bson.M{"updatedAt": time.Now()},
	}

	_, err := talkBoardCollection.UpdateOne(context.Background(), bson.M{"_id": postId}, update)
	if err != nil {
		return nil, err
	}

	var post models.Post
	err = talkBoardCollection.FindOne(context.Background(), bson.M{"_id": postId}).Decode(&post)
	if err != nil {
		return nil, err
	}

	return &post, nil
}

func AddReactionToPost(postId primitive.ObjectID, reaction *models.Reaction) (*models.Post, error) {
	reaction.ID = primitive.NewObjectID()
	reaction.CreatedAt = time.Now()

	update := bson.M{
		"$push": bson.M{"reactions": reaction},
		"$set":  bson.M{"updatedAt": time.Now()},
	}

	_, err := talkBoardCollection.UpdateOne(context.Background(), bson.M{"_id": postId}, update)
	if err != nil {
		return nil, err
	}

	var post models.Post
	err = talkBoardCollection.FindOne(context.Background(), bson.M{"_id": postId}).Decode(&post)
	if err != nil {
		return nil, err
	}

	return &post, nil
}

func AddReactionToComment(postId, commentId primitive.ObjectID, reaction *models.Reaction) (*models.Post, error) {
	reaction.ID = primitive.NewObjectID()
	reaction.CreatedAt = time.Now()

	update := bson.M{
		"$push": bson.M{"comments.$[elem].reactions": reaction},
		"$set":  bson.M{"updatedAt": time.Now()},
	}

	arrayFilters := options.Update().SetArrayFilters(options.ArrayFilters{
		Filters: []interface{}{bson.M{"elem._id": commentId}},
	})

	result, err := talkBoardCollection.UpdateOne(context.Background(), bson.M{"_id": postId}, update, arrayFilters)
	if err != nil {
		return nil, err
	}

	if result.MatchedCount == 0 {
		return nil, errors.New("post or comment not found")
	}

	var post models.Post
	err = talkBoardCollection.FindOne(context.Background(), bson.M{"_id": postId}).Decode(&post)
	if err != nil {
		return nil, err
	}

	return &post, nil
}