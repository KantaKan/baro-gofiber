package repository

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// UserGroupsRepository changes a learner's GenMate group.
type UserGroupsRepository struct{ users *mongo.Collection }

func NewUserGroupsRepository(db *mongo.Database) *UserGroupsRepository {
	return &UserGroupsRepository{users: db.Collection("users")}
}

func (r *UserGroupsRepository) SetGenmateGroup(ctx context.Context, id primitive.ObjectID, group string) error {
	_, err := r.users.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{"genmate_group": group}})
	return err
}
