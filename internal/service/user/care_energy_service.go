package user

import (
	"context"
	"errors"
	"time"

	"gofiber-baro/internal/domain"
	"gofiber-baro/internal/service/holiday"
	"gofiber-baro/pkg/utils"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

const FeedPointsPerCareEnergy = 10

var ErrInvalidProtectDate = errors.New("date must be a past weekday and not a holiday")
var ErrInvalidFeedQuantity = errors.New("quantity must be at least 1")
var ErrCannotGiftSelf = errors.New("cannot gift Care Energy to yourself")

type CareEnergyService struct {
	userRepo   careEnergyRepository
	holidaySvc *holiday.Service
}

type careEnergyRepository interface {
	FindByID(ctx interface{}, id primitive.ObjectID) (*domain.User, error)
	GrantCareEnergy(ctx interface{}, userID primitive.ObjectID, amount int, note, grantedBy string) error
	UseCareEnergyProtect(ctx interface{}, userID primitive.ObjectID, dateStr string) error
	UseCareEnergyFeed(ctx interface{}, userID primitive.ObjectID, quantity, points int) error
	GiftCareEnergy(ctx interface{}, giverID, recipientID primitive.ObjectID, quantity, points int, note string) error
	RescueCareEnergy(ctx interface{}, giverID, recipientID primitive.ObjectID, dateStr, note string) error
	UseCareEnergyCharacter(ctx interface{}, userID primitive.ObjectID, effect string) error
}

func NewCareEnergyService(userRepo careEnergyRepository, holidaySvc *holiday.Service) *CareEnergyService {
	return &CareEnergyService{userRepo: userRepo, holidaySvc: holidaySvc}
}

func careEnergyState(user *domain.User) domain.CareEnergyState {
	log := user.CareEnergyLog
	if log == nil {
		log = []domain.CareEnergyLogEntry{}
	}
	return domain.CareEnergyState{Balance: user.CareEnergyBalance, Log: log, CareCount: user.CharacterCareCount, LastCaredAt: user.LastCaredAt}
}

func (s *CareEnergyService) CareEnergyState(userID primitive.ObjectID) (*domain.CareEnergyState, error) {
	user, err := s.userRepo.FindByID(context.Background(), userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, domain.ErrUserNotFound
	}
	state := careEnergyState(user)
	return &state, nil
}

func (s *CareEnergyService) CareCharacter(userID primitive.ObjectID) (*domain.CharacterCareResult, error) {
	const effect = "happy-hop"
	if err := s.userRepo.UseCareEnergyCharacter(context.Background(), userID, effect); err != nil {
		return nil, err
	}
	state, err := s.CareEnergyState(userID)
	if err != nil {
		return nil, err
	}
	return &domain.CharacterCareResult{Effect: effect, State: *state}, nil
}

func (s *CareEnergyService) Grant(userID primitive.ObjectID, amount int, note, grantedBy string) error {
	ctx := context.Background()
	return s.userRepo.GrantCareEnergy(ctx, userID, amount, note, grantedBy)
}

func (s *CareEnergyService) validateProtectDate(dateStr string) error {
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return ErrInvalidProtectDate
	}

	today := utils.GetThailandTime()
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

func (s *CareEnergyService) ProtectDate(userID primitive.ObjectID, dateStr string) error {
	if err := s.validateProtectDate(dateStr); err != nil {
		return err
	}

	ctx := context.Background()
	return s.userRepo.UseCareEnergyProtect(ctx, userID, dateStr)
}

func (s *CareEnergyService) Feed(userID primitive.ObjectID, quantity int) error {
	if quantity < 1 {
		return ErrInvalidFeedQuantity
	}
	ctx := context.Background()
	return s.userRepo.UseCareEnergyFeed(ctx, userID, quantity, quantity*FeedPointsPerCareEnergy)
}

func (s *CareEnergyService) Gift(giverID, recipientID primitive.ObjectID, quantity int, note string) error {
	if quantity < 1 {
		return ErrInvalidFeedQuantity
	}
	if giverID == recipientID {
		return ErrCannotGiftSelf
	}
	ctx := context.Background()
	return s.userRepo.GiftCareEnergy(ctx, giverID, recipientID, quantity, quantity*FeedPointsPerCareEnergy, note)
}

func (s *CareEnergyService) Rescue(giverID, recipientID primitive.ObjectID, dateStr, note string) error {
	if giverID == recipientID {
		return ErrCannotGiftSelf
	}
	if err := s.validateProtectDate(dateStr); err != nil {
		return err
	}

	ctx := context.Background()
	return s.userRepo.RescueCareEnergy(ctx, giverID, recipientID, dateStr, note)
}
