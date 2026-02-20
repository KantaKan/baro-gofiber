package user

import (
	"context"
	"time"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type BadgeService struct {
	userRepo domain.UserRepository
}

func NewBadgeService(userRepo domain.UserRepository) *BadgeService {
	return &BadgeService{userRepo: userRepo}
}

func (s *BadgeService) AwardBadge(userID primitive.ObjectID, badgeType, badgeName, emoji, imageUrl, color, style string) error {
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

	return s.userRepo.AddBadge(ctx, userID, badge)
}
