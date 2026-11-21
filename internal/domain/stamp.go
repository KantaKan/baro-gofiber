package domain

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Cohort struct {
	CohortNumber int        `bson:"cohort_number" json:"cohortNumber"`
	Name         string     `bson:"name" json:"name"`
	StartDate    time.Time  `bson:"start_date" json:"startDate"`
	LockAt       time.Time  `bson:"lock_at" json:"lockAt"`
	IsLocked     bool       `bson:"is_locked" json:"isLocked"`
	PosterURL    string     `bson:"poster_url,omitempty" json:"posterUrl,omitempty"`
	CreatedAt    time.Time  `bson:"created_at" json:"createdAt"`
}

type Stamp struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	OwnerID      primitive.ObjectID `bson:"ownerId" json:"ownerId"`
	CohortNumber int                `bson:"cohortNumber" json:"cohortNumber"`
	ImageURL     string             `bson:"imageUrl" json:"imageUrl"`
	CreatedAt    time.Time          `bson:"createdAt" json:"createdAt"`
}

type CohortRepository interface {
	FindByCohortNumber(ctx context.Context, cohort int) (*Cohort, error)
	EnsureExists(ctx context.Context, cohort int) (*Cohort, error)
	Update(ctx context.Context, cohort int, update interface{}) error
	List(ctx context.Context) ([]Cohort, error)
	LockExpired(ctx context.Context) error
}

type StampRepository interface {
	InsertStamp(ctx context.Context, stamp *Stamp) error
	FindByCohort(ctx context.Context, cohort int) ([]Stamp, error)
	HasStampAfter(ctx context.Context, ownerID primitive.ObjectID, after time.Time) (bool, error)
	DeleteByID(ctx context.Context, cohort int, id primitive.ObjectID) error
	DeleteByCohort(ctx context.Context, cohort int) error
}
