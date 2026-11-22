package repository

import (
	"reflect"
	"testing"

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
