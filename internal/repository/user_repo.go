package repository

import (
	"context"
	"errors"
	"log"
	"regexp"
	"strings"
	"time"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
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
			return nil, domain.ErrUserNotFound
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
			return nil, domain.ErrUserNotFound
		}
		log.Printf("FindByEmail error for %s: %v", email, err)
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

func (r *userRepository) Create(ctx interface{}, user *domain.User) error {
	c := ctx.(context.Context)
	user.ID = primitive.NewObjectID()
	_, err := r.collection.InsertOne(c, user)
	return err
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

func (r *userRepository) GrantFertilizer(ctx interface{}, userID primitive.ObjectID, amount int, note, grantedBy string) error {
	c := ctx.(context.Context)
	entry := domain.FertilizerLogEntry{
		ID:        primitive.NewObjectID(),
		Kind:      "grant",
		Amount:    amount,
		Note:      note,
		GrantedBy: grantedBy,
		CreatedAt: time.Now(),
	}
	filter := bson.M{"_id": userID}
	update := bson.M{
		"$inc":  bson.M{"fertilizer_balance": amount},
		"$push": bson.M{"fertilizer_log": entry},
	}
	result, err := r.collection.UpdateOne(c, filter, update)
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return domain.ErrUserNotFound
	}
	return nil
}

func (r *userRepository) UseFertilizerProtect(ctx interface{}, userID primitive.ObjectID, dateStr string) error {
	c := ctx.(context.Context)
	filter := bson.M{
		"_id":                userID,
		"fertilizer_balance": bson.M{"$gte": 1},
		"fertilizer_log": bson.M{
			"$not": bson.M{"$elemMatch": bson.M{"kind": "protect", "relatedDate": dateStr}},
		},
	}
	entry := domain.FertilizerLogEntry{
		ID:          primitive.NewObjectID(),
		Kind:        "protect",
		Amount:      1,
		RelatedDate: dateStr,
		CreatedAt:   time.Now(),
	}
	update := bson.M{
		"$inc":  bson.M{"fertilizer_balance": -1},
		"$push": bson.M{"fertilizer_log": entry},
	}
	result, err := r.collection.UpdateOne(c, filter, update)
	if err != nil {
		return err
	}
	if result.ModifiedCount == 0 {
		exists, checkErr := r.hasProtectedDate(c, userID, dateStr)
		if checkErr == nil && exists {
			return domain.ErrDateAlreadyProtected
		}
		return domain.ErrInsufficientFertilizer
	}
	return nil
}

func (r *userRepository) UseFertilizerFeed(ctx interface{}, userID primitive.ObjectID, points int) error {
	c := ctx.(context.Context)
	filter := bson.M{
		"_id":                userID,
		"fertilizer_balance": bson.M{"$gte": 1},
	}
	entry := domain.FertilizerLogEntry{
		ID:        primitive.NewObjectID(),
		Kind:      "feed",
		Amount:    points,
		CreatedAt: time.Now(),
	}
	update := bson.M{
		"$inc":  bson.M{"fertilizer_balance": -1, "growth_points": points},
		"$push": bson.M{"fertilizer_log": entry},
	}
	result, err := r.collection.UpdateOne(c, filter, update)
	if err != nil {
		return err
	}
	if result.ModifiedCount == 0 {
		return domain.ErrInsufficientFertilizer
	}
	return nil
}

func (r *userRepository) hasProtectedDate(ctx context.Context, userID primitive.ObjectID, dateStr string) (bool, error) {
	count, err := r.collection.CountDocuments(ctx, bson.M{
		"_id":             userID,
		"fertilizer_log":  bson.M{"$elemMatch": bson.M{"kind": "protect", "relatedDate": dateStr}},
	})
	if err != nil {
		return false, err
	}
	return count > 0, nil
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
		escapedSearch := regexp.QuoteMeta(filter.Search)
		bsonFilter["$or"] = []bson.M{
			{"first_name": bson.M{"$regex": escapedSearch, "$options": "i"}},
			{"last_name": bson.M{"$regex": escapedSearch, "$options": "i"}},
			{"email": bson.M{"$regex": escapedSearch, "$options": "i"}},
		}
	}
	if filter.ExcludeAttendanceStatus != "" {
		statuses := strings.Split(filter.ExcludeAttendanceStatus, ",")
		if len(statuses) == 1 {
			bsonFilter["attendance_status"] = bson.M{"$ne": statuses[0]}
		} else {
			bsonFilter["attendance_status"] = bson.M{"$nin": statuses}
		}
	}

	return bsonFilter
}

func (r *userRepository) CreateReflection(ctx interface{}, userID primitive.ObjectID, reflection domain.Reflection) error {
	c := ctx.(context.Context)
	reflection.ID = primitive.NewObjectID()

	pipeline := mongo.Pipeline{
		{{Key: "$set", Value: bson.D{
			{Key: "reflections", Value: bson.D{
				{Key: "$ifNull", Value: bson.A{"$reflections", bson.A{}}},
			}},
		}}},
	}
	r.collection.UpdateOne(c, bson.M{"_id": userID}, pipeline)

	filter := bson.M{"_id": userID}
	update := bson.M{"$push": bson.M{"reflections": reflection}}
	_, err := r.collection.UpdateOne(c, filter, update)
	return err
}

func (r *userRepository) AddProfileComment(ctx interface{}, userID primitive.ObjectID, comment domain.ProfileComment) error {
	c := ctx.(context.Context)
	filter := bson.M{"_id": userID}

	if comment.ParentID != nil && !comment.ParentID.IsZero() {
		// Add reply to existing comment using array filters
		filter["profile_comments._id"] = comment.ParentID
		update := bson.M{
			"$push": bson.M{"profile_comments.$.replies": comment},
		}
		_, err := r.collection.UpdateOne(c, filter, update)
		return err
	}

	// Add new root comment
	update := bson.M{"$push": bson.M{"profile_comments": comment}}
	_, err := r.collection.UpdateOne(c, filter, update)
	return err
}

func (r *userRepository) DeleteProfileComment(ctx interface{}, userID primitive.ObjectID, commentID primitive.ObjectID) error {
	c := ctx.(context.Context)
	filter := bson.M{"_id": userID}

	// First try to delete from top-level comments
	update := bson.M{"$pull": bson.M{"profile_comments": bson.M{"_id": commentID}}}
	result, err := r.collection.UpdateOne(c, filter, update)
	if err != nil {
		return err
	}

	if result.ModifiedCount > 0 {
		return nil
	}

	// If not found in top-level, try to delete from replies (nested)
	updateReplies := bson.M{"$pull": bson.M{"profile_comments.$[].replies": bson.M{"_id": commentID}}}
	_, err = r.collection.UpdateOne(c, filter, updateReplies)
	return err
}

func (r *userRepository) AddProfileReaction(ctx interface{}, userID primitive.ObjectID, reaction domain.Reaction) error {
	c := ctx.(context.Context)
	filter := bson.M{"_id": userID}
	
	// First, remove any existing reaction by this user
	pull := bson.M{"$pull": bson.M{"profile_reactions": bson.M{"userId": reaction.UserID}}}
	_, err := r.collection.UpdateOne(c, filter, pull)
	if err != nil {
		return err
	}

	// Then, push the new reaction
	update := bson.M{"$push": bson.M{"profile_reactions": reaction}}
	_, err = r.collection.UpdateOne(c, filter, update)
	return err
}
