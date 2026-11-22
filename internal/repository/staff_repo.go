package repository

import (
	"context"
	"time"

	"gofiber-baro/internal/domain"
	"gofiber-baro/internal/service/staff"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// StaffRepository stores staff roles (on users) and invites (staff_invites).
type StaffRepository struct {
	users, invites *mongo.Collection
}

func NewStaffRepository(db *mongo.Database) *StaffRepository {
	return &StaffRepository{users: db.Collection("users"), invites: db.Collection("staff_invites")}
}

func staffCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 10*time.Second)
}

func (r *StaffRepository) EnsureIndexes(c context.Context) error {
	_, err := r.invites.Indexes().CreateMany(c, []mongo.IndexModel{
		{Keys: bson.D{{Key: "token_hash", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "email", Value: 1}}},
	})
	return err
}

var staffProjection = options.Find().SetProjection(bson.M{"reflections": 0, "password": 0})

func (r *StaffRepository) Members() ([]domain.User, error) {
	c, cancel := staffCtx()
	defer cancel()
	cur, err := r.users.Find(c, bson.M{"role": "admin"}, staffProjection)
	if err != nil {
		return nil, err
	}
	var out []domain.User
	return out, cur.All(c, &out)
}

func (r *StaffRepository) one(filter bson.M) (*domain.User, error) {
	c, cancel := staffCtx()
	defer cancel()
	var u domain.User
	err := r.users.FindOne(c, filter, options.FindOne().SetProjection(bson.M{"reflections": 0})).Decode(&u)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *StaffRepository) UserByEmail(email string) (*domain.User, error) {
	return r.one(bson.M{"email": email})
}

func (r *StaffRepository) UserByID(id primitive.ObjectID) (*domain.User, error) {
	return r.one(bson.M{"_id": id})
}

func (r *StaffRepository) CreateUser(u *domain.User) error {
	c, cancel := staffCtx()
	defer cancel()
	u.ID = primitive.NewObjectID()
	_, err := r.users.InsertOne(c, u)
	return err
}

func (r *StaffRepository) SetStaff(id primitive.ObjectID, role, staffRole string, cohorts []int, groups []string) error {
	c, cancel := staffCtx()
	defer cancel()
	_, err := r.users.UpdateOne(c, bson.M{"_id": id}, bson.M{"$set": bson.M{"role": role, "staff_role": staffRole, "staff_cohorts": cohorts, "staff_groups": groups}})
	return err
}

func (r *StaffRepository) SaveInvite(i *staff.Invite) error {
	c, cancel := staffCtx()
	defer cancel()
	if i.ID.IsZero() {
		i.ID = primitive.NewObjectID()
	}
	_, err := r.invites.ReplaceOne(c, bson.M{"_id": i.ID}, i, options.Replace().SetUpsert(true))
	return err
}

func (r *StaffRepository) Invites() ([]staff.Invite, error) {
	c, cancel := staffCtx()
	defer cancel()
	cur, err := r.invites.Find(c, bson.M{"used_at": bson.M{"$exists": false}}, options.Find().SetSort(bson.M{"expires_at": -1}))
	if err != nil {
		return nil, err
	}
	var out []staff.Invite
	return out, cur.All(c, &out)
}

func (r *StaffRepository) invite(filter bson.M) (*staff.Invite, error) {
	c, cancel := staffCtx()
	defer cancel()
	var i staff.Invite
	err := r.invites.FindOne(c, filter).Decode(&i)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &i, nil
}

func (r *StaffRepository) InviteByID(id primitive.ObjectID) (*staff.Invite, error) {
	return r.invite(bson.M{"_id": id})
}

func (r *StaffRepository) InviteByHash(h string) (*staff.Invite, error) {
	return r.invite(bson.M{"token_hash": h})
}

func (r *StaffRepository) DeleteInvite(id primitive.ObjectID) error {
	c, cancel := staffCtx()
	defer cancel()
	_, err := r.invites.DeleteOne(c, bson.M{"_id": id, "used_at": bson.M{"$exists": false}})
	return err
}

// MarkUsed claims an invite atomically; false if it was already used.
func (r *StaffRepository) MarkUsed(id primitive.ObjectID, at time.Time) (bool, error) {
	c, cancel := staffCtx()
	defer cancel()
	res, err := r.invites.UpdateOne(c, bson.M{"_id": id, "used_at": bson.M{"$exists": false}}, bson.M{"$set": bson.M{"used_at": at}})
	if err != nil {
		return false, err
	}
	return res.ModifiedCount == 1, nil
}
