package user

import (
	"context"
	"errors"
	"time"

	"gofiber-baro/internal/domain"
	"gofiber-baro/internal/service/holiday"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

const FeedPointsPerFertilizer = 10

var ErrInvalidProtectDate = errors.New("date must be a past weekday and not a holiday")
var ErrInvalidFeedQuantity = errors.New("quantity must be at least 1")
var ErrCannotGiftSelf = errors.New("cannot gift fertilizer to yourself")

type FertilizerService struct {
	userRepo   domain.UserRepository
	holidaySvc *holiday.Service
}

func NewFertilizerService(userRepo domain.UserRepository, holidaySvc *holiday.Service) *FertilizerService {
	return &FertilizerService{userRepo: userRepo, holidaySvc: holidaySvc}
}

func (s *FertilizerService) Grant(userID primitive.ObjectID, amount int, note, grantedBy string) error {
	ctx := context.Background()
	return s.userRepo.GrantFertilizer(ctx, userID, amount, note, grantedBy)
}

func (s *FertilizerService) validateProtectDate(dateStr string) error {
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return ErrInvalidProtectDate
	}

	today := time.Now()
	todayDay := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())
	if !date.Before(todayDay) {
		return ErrInvalidProtectDate
	}

	if date.Weekday() == time.Saturday || date.Weekday() == time.Sunday {
		return ErrInvalidProtectDate
	}

	isHoliday, _, err := s.holidaySvc.IsHoliday(dateStr)
	if err != nil {
		return err
	}
	if isHoliday {
		return ErrInvalidProtectDate
	}

	return nil
}

func (s *FertilizerService) ProtectDate(userID primitive.ObjectID, dateStr string) error {
	if err := s.validateProtectDate(dateStr); err != nil {
		return err
	}

	ctx := context.Background()
	return s.userRepo.UseFertilizerProtect(ctx, userID, dateStr)
}

func (s *FertilizerService) Feed(userID primitive.ObjectID, quantity int) error {
	if quantity < 1 {
		return ErrInvalidFeedQuantity
	}
	ctx := context.Background()
	return s.userRepo.UseFertilizerFeed(ctx, userID, quantity, quantity*FeedPointsPerFertilizer)
}

func (s *FertilizerService) Gift(giverID, recipientID primitive.ObjectID, quantity int, note string) error {
	if quantity < 1 {
		return ErrInvalidFeedQuantity
	}
	if giverID == recipientID {
		return ErrCannotGiftSelf
	}
	ctx := context.Background()
	return s.userRepo.GiftFertilizer(ctx, giverID, recipientID, quantity, quantity*FeedPointsPerFertilizer, note)
}

func (s *FertilizerService) Rescue(giverID, recipientID primitive.ObjectID, dateStr, note string) error {
	if giverID == recipientID {
		return ErrCannotGiftSelf
	}
	if err := s.validateProtectDate(dateStr); err != nil {
		return err
	}

	ctx := context.Background()
	return s.userRepo.RescueFertilizer(ctx, giverID, recipientID, dateStr, note)
}
