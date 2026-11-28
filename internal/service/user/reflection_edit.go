package user

import (
	"context"
	"errors"
	"time"

	"gofiber-baro/internal/domain"
	"gofiber-baro/pkg/utils"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

var (
	ErrReflectionNotFound = errors.New("reflection not found")
	ErrReflectionNotToday = errors.New("only today's reflection can be edited")
	ErrReflectionReplied  = errors.New("a TA has already replied to this reflection")
)

// CanEditReflection: a learner may edit their reflection on the same Bangkok day, until a TA replies.
func CanEditReflection(r domain.Reflection, now time.Time) error {
	loc, err := time.LoadLocation("Asia/Bangkok")
	if err != nil {
		loc = time.FixedZone("ICT", 7*3600)
	}
	day := r.Day
	if day == "" {
		day = r.CreatedAt.In(loc).Format("2006-01-02")
	}
	if day != now.In(loc).Format("2006-01-02") {
		return ErrReflectionNotToday
	}
	if r.AdminFeedback != "" {
		return ErrReflectionReplied
	}
	return nil
}

// EditReflection replaces the content of the learner's own reflection when CanEditReflection allows it.
func (s *Service) EditReflection(userID, reflectionID primitive.ObjectID, content domain.ReflectionContent) error {
	ctx := context.Background()
	u, err := s.repo.FindByID(ctx, userID)
	if err != nil || u == nil {
		return domain.ErrUserNotFound
	}
	for _, r := range u.Reflections {
		if r.ID != reflectionID {
			continue
		}
		if err := CanEditReflection(r, utils.GetThailandTime()); err != nil {
			return err
		}
		return s.repo.EditOwnReflection(ctx, userID, reflectionID, content)
	}
	return ErrReflectionNotFound
}
