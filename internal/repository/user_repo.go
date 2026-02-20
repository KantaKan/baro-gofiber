package repository

import (
	"context"
	"errors"
	"log"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	ErrUserNotFound = errors.New("user not found")
)

type userRepository struct {
	collection *mongo.Collection
}

func NewUserRepository(db *mongo.Database) domain.UserRepository {
	return &userRepository{
		collection: db.Collection("users"),
	}
}

func (r *userRepository) FindByID(ctx interface{}, id primitive.ObjectID) (*domain.User, error) {
	c := ctx.(context.Context)
	var user domain.User
	err := r.collection.FindOne(c, bson.M{"_id": id}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) FindByEmail(ctx interface{}, email string) (*domain.User, error) {
	c := ctx.(context.Context)
	var user domain.User
	err := r.collection.FindOne(c, bson.M{"email": email}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) FindAll(ctx interface{}, filter domain.UserFilter, opts interface{}) ([]domain.User, int, error) {
	c := ctx.(context.Context)
	bsonFilter := r.buildFilter(filter)

	findOpts := options.Find()
	if opts != nil {
		if o, ok := opts.(*options.FindOptions); ok {
			findOpts = o
		}
	}

	cursor, err := r.collection.Find(c, bsonFilter, findOpts)
	if err != nil {
		log.Printf("Error fetching users: %v", err)
		return nil, 0, errors.New("error fetching users")
	}
	defer cursor.Close(c)

	var users []domain.User
	for cursor.Next(c) {
		var user domain.User
		if err := cursor.Decode(&user); err != nil {
			log.Printf("Error decoding user: %v", err)
			continue
		}
		users = append(users, user)
	}

	total, err := r.collection.CountDocuments(c, bsonFilter)
	if err != nil {
		return users, 0, nil
	}

	return users, int(total), nil
}

func (r *userRepository) Update(ctx interface{}, id primitive.ObjectID, update interface{}) error {
	c := ctx.(context.Context)
	filter := bson.M{"_id": id}
	_, err := r.collection.UpdateOne(c, filter, bson.M{"$set": update})
	return err
}

func (r *userRepository) AddBadge(ctx interface{}, userID primitive.ObjectID, badge domain.Badge) error {
	c := ctx.(context.Context)
	filter := bson.M{"_id": userID}
	update := bson.M{"$push": bson.M{"badges": badge}}
	_, err := r.collection.UpdateOne(c, filter, update)
	return err
}

func (r *userRepository) UpdateReflectionFeedback(ctx interface{}, userID, reflectionID primitive.ObjectID, feedback string) error {
	c := ctx.(context.Context)
	filter := bson.M{
		"_id":             userID,
		"reflections._id": reflectionID,
	}
	update := bson.M{
		"$set": bson.M{
			"reflections.$.admin_feedback": feedback,
		},
	}
	result, err := r.collection.UpdateOne(c, filter, update)
	if err != nil {
		return err
	}
	if result.ModifiedCount == 0 {
		return errors.New("user or reflection not found")
	}
	return nil
}

func (r *userRepository) buildFilter(filter domain.UserFilter) bson.M {
	bsonFilter := bson.M{}

	if filter.Cohort > 0 {
		bsonFilter["cohort_number"] = filter.Cohort
	}
	if filter.Role != "" {
		bsonFilter["role"] = filter.Role
	}
	if filter.Email != "" {
		bsonFilter["email"] = filter.Email
	}
	if filter.Search != "" {
		bsonFilter["$or"] = []bson.M{
			{"first_name": bson.M{"$regex": filter.Search, "$options": "i"}},
			{"last_name": bson.M{"$regex": filter.Search, "$options": "i"}},
			{"email": bson.M{"$regex": filter.Search, "$options": "i"}},
		}
	}

	return bsonFilter
}

func (r *userRepository) CreateReflection(ctx interface{}, userID primitive.ObjectID, reflection domain.Reflection) error {
	c := ctx.(context.Context)
	reflection.ID = primitive.NewObjectID()
	filter := bson.M{"_id": userID}
	update := bson.M{"$push": bson.M{"reflections": reflection}}
	_, err := r.collection.UpdateOne(c, filter, update)
	return err
}
