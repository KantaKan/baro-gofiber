package user

import (
	"context"
	"errors"
	"time"

	"gofiber-baro/internal/domain"
	"gofiber-baro/pkg/utils"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	ErrUserNotFound = errors.New("user not found")
)

type Service struct {
	repo domain.UserRepository
}

func NewService(repo domain.UserRepository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetUserByID(id string) (*domain.User, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, ErrUserNotFound
	}

	ctx := context.Background()
	return s.repo.FindByID(ctx, oid)
}

func (s *Service) GetUserByEmail(email string) (*domain.User, error) {
	ctx := context.Background()
	return s.repo.FindByEmail(ctx, email)
}

func (s *Service) GetAllUsers(cohort int, role, email, search, sort string, sortDir, page, limit int) ([]domain.User, int, error) {
	ctx := context.Background()

	filter := domain.UserFilter{
		Cohort: cohort,
		Role:   role,
		Email:  email,
		Search: search,
	}

	findOpts := options.Find()
	if limit > 0 {
		findOpts.SetLimit(int64(limit))
	}
	if page > 1 {
		skip := int64((page - 1) * limit)
		findOpts.SetSkip(skip)
	}
	if sort != "" {
		direction := 1
		if sortDir == -1 {
			direction = -1
		}
		findOpts.SetSort(bson.D{{Key: sort, Value: direction}})
	}

	return s.repo.FindAll(ctx, filter, findOpts)
}

func (s *Service) UpdateUser(id string, update interface{}) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return ErrUserNotFound
	}

	ctx := context.Background()
	return s.repo.Update(ctx, oid, update)
}

func (s *Service) AwardBadge(userID primitive.ObjectID, badgeType, badgeName, emoji, imageUrl, color, style string) error {
	ctx := context.Background()

	badge := domain.Badge{
		ID:        primitive.NewObjectID(),
		Type:      badgeType,
		Name:      badgeName,
		Emoji:     emoji,
		ImageUrl:  imageUrl,
		Color:     color,
		Style:     style,
		AwardedAt: time.Now(),
	}

	return s.repo.AddBadge(ctx, userID, badge)
}

func (s *Service) UpdateReflectionFeedback(userID, reflectionID primitive.ObjectID, feedback string) error {
	ctx := context.Background()
	return s.repo.UpdateReflectionFeedback(ctx, userID, reflectionID, feedback)
}

func (s *Service) CreateReflection(userID primitive.ObjectID, reflection domain.Reflection) (*domain.Reflection, error) {
	ctx := context.Background()

	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return nil, ErrUserNotFound
	}

	now := utils.GetThailandTime()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	for _, r := range user.Reflections {
		reflectionDate := time.Date(r.CreatedAt.Year(), r.CreatedAt.Month(), r.CreatedAt.Day(), 0, 0, 0, 0, now.Location())
		if reflectionDate.Equal(today) {
			return nil, errors.New("user has already created a reflection today")
		}
	}

	reflection.CreatedAt = now
	reflection.ID = primitive.NewObjectID()

	if err := s.repo.CreateReflection(ctx, userID, reflection); err != nil {
		return nil, err
	}

	return &reflection, nil
}

func (s *Service) GetReflections(userID primitive.ObjectID) ([]domain.Reflection, error) {
	ctx := context.Background()

	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return nil, ErrUserNotFound
	}

	return user.Reflections, nil
}
