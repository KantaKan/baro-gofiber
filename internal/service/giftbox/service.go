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
	ListForUser(ctx context.Context, userID primitive.ObjectID) ([]domain.TeacherGiftBox, error)
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
		Pool:           "teacher-box",
		MinimumRarity:  box.MinimumRarity,
	})
}
