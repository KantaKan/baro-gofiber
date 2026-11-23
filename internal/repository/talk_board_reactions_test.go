package repository

import (
	"reflect"
	"testing"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestNullReplyReactionsRepair(t *testing.T) {
	id := primitive.NewObjectID()
	update, filters := nullReplyReactionsRepair(id)
	if !reflect.DeepEqual(update, bson.M{"$set": bson.M{"comments.$[c].reactions": bson.A{}}}) {
		t.Errorf("update: %v", update)
	}
	want := []interface{}{bson.M{"c._id": id, "c.reactions": nil}}
	if !reflect.DeepEqual(filters, want) {
		t.Errorf("filters: %v", filters)
	}
}

func TestReactionUpdatesAreOnePerPerson(t *testing.T) {
	post, comment, user := primitive.NewObjectID(), primitive.NewObjectID(), primitive.NewObjectID()
	r := domain.Reaction{ID: primitive.NewObjectID(), UserID: user, Value: "like"}

	// change an existing reaction in place
	f, u, filters := postReactionReplace(post, r)
	if !reflect.DeepEqual(f, bson.M{"_id": post}) || !reflect.DeepEqual(u, bson.M{"$set": bson.M{"reactions.$[r]": r}}) || !reflect.DeepEqual(filters, []interface{}{bson.M{"r.userId": user}}) {
		t.Errorf("post replace: %v %v %v", f, u, filters)
	}
	// add only if this person has none: the filter makes a second concurrent add match nothing
	f, u = postReactionAddIfNone(post, r)
	if !reflect.DeepEqual(f, bson.M{"_id": post, "reactions.userId": bson.M{"$ne": user}}) || !reflect.DeepEqual(u, bson.M{"$push": bson.M{"reactions": r}}) {
		t.Errorf("post add: %v %v", f, u)
	}

	f, u, filters = commentReactionReplace(post, comment, r)
	if !reflect.DeepEqual(f, bson.M{"_id": post}) || !reflect.DeepEqual(u, bson.M{"$set": bson.M{"comments.$[c].reactions.$[r]": r}}) ||
		!reflect.DeepEqual(filters, []interface{}{bson.M{"c._id": comment}, bson.M{"r.userId": user}}) {
		t.Errorf("comment replace: %v %v %v", f, u, filters)
	}
	f, u = commentReactionAddIfNone(post, comment, r)
	wantF := bson.M{"_id": post, "comments": bson.M{"$elemMatch": bson.M{"_id": comment, "reactions.userId": bson.M{"$ne": user}}}}
	if !reflect.DeepEqual(f, wantF) || !reflect.DeepEqual(u, bson.M{"$push": bson.M{"comments.$.reactions": r}}) {
		t.Errorf("comment add: %v %v", f, u)
	}
}
