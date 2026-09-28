package giftbox

import (
	"context"
	"errors"
	"strings"
	"time"

	"gofiber-baro/internal/domain"
	"gofiber-baro/internal/service/reward"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

var ErrBoxNotFound = errors.New("gift box not found")

type Store interface {
	Create(ctx context.Context, box domain.TeacherGiftBox) error
	CreateOnce(ctx context.Context, box domain.TeacherGiftBox) (bool, error)
	ListForUser(ctx context.Context, userID primitive.ObjectID) ([]domain.TeacherGiftBox, error)
	ListCohortLearners(ctx context.Context, cohort int) ([]primitive.ObjectID, error)
}

type CohortGrantFailure struct {
	UserID string `json:"user_id"`
	Error  string `json:"error"`
}

type CohortGrantResult struct {
	Total    int                  `json:"total"`
	Created  int                  `json:"created"`
	Existing int                  `json:"existing"`
	Failures []CohortGrantFailure `json:"failures"`
}

type Drawer interface {
	Open(ctx context.Context, request reward.DrawRequest) (*reward.DrawResult, error)
}

type Service struct {
	store  Store
	drawer Drawer
}

func NewService(store Store, drawer Drawer) *Service {
	return &Service{store: store, drawer: drawer}
}

func (s *Service) Grant(ctx context.Context, userID, adminID, minimumRarity, message string) (*domain.TeacherGiftBox, error) {
	learner, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, errors.New("invalid learner ID")
	}
	admin, err := primitive.ObjectIDFromHex(adminID)
	if err != nil {
		return nil, errors.New("invalid admin ID")
	}
	box := domain.TeacherGiftBox{
		ID:            primitive.NewObjectID(),
		UserID:        learner,
		MinimumRarity: minimumRarity,
		Message:       strings.TrimSpace(message),
		GrantedBy:     admin,
		Status:        "unopened",
		CreatedAt:     time.Now(),
	}
	if err := s.store.Create(ctx, box); err != nil {
		return nil, err
	}
	return &box, nil
}

func (s *Service) GrantCohort(ctx context.Context, cohort int, adminID, minimumRarity, message, idempotencyKey string) (*CohortGrantResult, error) {
	admin, err := primitive.ObjectIDFromHex(adminID)
	if err != nil {
		return nil, errors.New("invalid admin ID")
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if cohort <= 0 || idempotencyKey == "" {
		return nil, errors.New("cohort and idempotency key are required")
	}
	learners, err := s.store.ListCohortLearners(ctx, cohort)
	if err != nil {
		return nil, err
	}
	result := &CohortGrantResult{Total: len(learners), Failures: []CohortGrantFailure{}}
	for _, learner := range learners {
		box := domain.TeacherGiftBox{
			ID: primitive.NewObjectID(), UserID: learner, MinimumRarity: minimumRarity,
			Message: strings.TrimSpace(message), GrantedBy: admin, Status: "unopened",
			CreatedAt: time.Now(), GrantKey: idempotencyKey + ":" + learner.Hex(),
		}
		created, createErr := s.store.CreateOnce(ctx, box)
		if createErr != nil {
			result.Failures = append(result.Failures, CohortGrantFailure{UserID: learner.Hex(), Error: createErr.Error()})
		} else if created {
			result.Created++
		} else {
			result.Existing++
		}
	}
	return result, nil
}

func (s *Service) List(ctx context.Context, userID string) ([]domain.TeacherGiftBox, error) {
	id, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, errors.New("invalid learner ID")
	}
	return s.store.ListForUser(ctx, id)
}

func (s *Service) Open(ctx context.Context, userID, boxID string) (*reward.DrawResult, error) {
	boxes, err := s.List(ctx, userID)
	if err != nil {
		return nil, err
	}
	var box *domain.TeacherGiftBox
	for index := range boxes {
		if boxes[index].ID.Hex() == boxID {
			box = &boxes[index]
			break
		}
	}
	if box == nil {
		return nil, ErrBoxNotFound
	}
	return s.drawer.Open(ctx, reward.DrawRequest{
		IdempotencyKey: box.ID.Hex(),
		UserID:         userID,
		Pool:           drawPool(box.Source),
		MinimumRarity:  box.MinimumRarity,
	})
}

func drawPool(source string) string {
	switch source {
	case "achievement":
		return "achievement"
	case "reflection-milestone":
		return "reflection"
	default:
		return "teacher-box"
	}
}
