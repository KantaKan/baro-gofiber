package milestone

import (
	"context"
	"fmt"
	"time"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type UserStore interface {
	FindByID(ctx interface{}, id primitive.ObjectID) (*domain.User, error)
}

type RewardStore interface {
	CreateOnce(ctx context.Context, box domain.TeacherGiftBox) (bool, error)
}

type HolidayCalendar interface {
	GetHolidayDatesInRange(startDate, endDate string) (map[string]bool, error)
}

type Service struct {
	users    UserStore
	rewards  RewardStore
	holidays HolidayCalendar
}

type rewardMilestone struct {
	Days          int
	MinimumRarity string
}

var rewardMilestones = []rewardMilestone{
	{Days: 3, MinimumRarity: "Common"},
	{Days: 7, MinimumRarity: "Common"},
	{Days: 14, MinimumRarity: "Rare"},
	{Days: 21, MinimumRarity: "Rare"},
	{Days: 30, MinimumRarity: "Epic"},
	{Days: 45, MinimumRarity: "Epic"},
	{Days: 60, MinimumRarity: "Legendary"},
	{Days: 75, MinimumRarity: "Legendary"},
	{Days: 100, MinimumRarity: "Legendary"},
}

func NewService(users UserStore, rewards RewardStore, holidays HolidayCalendar) *Service {
	return &Service{users: users, rewards: rewards, holidays: holidays}
}

func (s *Service) Reconcile(ctx context.Context, userID primitive.ObjectID, now time.Time) ([]domain.TeacherGiftBox, error) {
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	reflectionDates := make([]string, 0, len(user.Reflections))
	earliest := thailandDate(now).AddDate(-1, 0, 0)
	for _, reflection := range user.Reflections {
		key := reflection.Day
		if key == "" {
			key = thailandDate(reflection.CreatedAt).Format("2006-01-02")
		}
		reflectionDates = append(reflectionDates, key)
		if parsed, parseErr := time.Parse("2006-01-02", key); parseErr == nil && parsed.Before(earliest) {
			earliest = parsed
		}
	}
	protectedDates := []string{}
	for _, entry := range user.CareEnergyLog {
		if entry.Kind == "protect" && entry.RelatedDate != "" {
			protectedDates = append(protectedDates, entry.RelatedDate)
		}
	}
	holidayMap, err := s.holidays.GetHolidayDatesInRange(earliest.Format("2006-01-02"), thailandDate(now).Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	holidays := make([]string, 0, len(holidayMap))
	for date := range holidayMap {
		holidays = append(holidays, date)
	}
	streak := CalculateWorkdayStreak(now, reflectionDates, protectedDates, holidays)
	granted := []domain.TeacherGiftBox{}
	for _, milestone := range rewardMilestones {
		if streak < milestone.Days {
			continue
		}
		box := domain.TeacherGiftBox{
			ID: primitive.NewObjectID(), UserID: userID, MinimumRarity: milestone.MinimumRarity,
			Message: fmt.Sprintf("Your %d-workday reflection streak helped your garden grow.", milestone.Days),
			Status:  "unopened", CreatedAt: now, GrantKey: fmt.Sprintf("milestone:%d:%s", milestone.Days, userID.Hex()),
			Source: "reflection-milestone",
		}
		created, createErr := s.rewards.CreateOnce(ctx, box)
		if createErr != nil {
			return granted, createErr
		}
		if created {
			granted = append(granted, box)
		}
	}
	return granted, nil
}

func CalculateWorkdayStreak(now time.Time, reflectionDates, protectedDates, holidayDates []string) int {
	reflections := stringSet(reflectionDates)
	protected := stringSet(protectedDates)
	holidays := stringSet(holidayDates)
	current := thailandDate(now)
	for isWeekend(current) || holidays[current.Format("2006-01-02")] {
		current = current.AddDate(0, 0, -1)
	}
	streak := 0
	for daysChecked := 0; daysChecked < 3660; current = current.AddDate(0, 0, -1) {
		key := current.Format("2006-01-02")
		if isWeekend(current) || holidays[key] {
			continue
		}
		daysChecked++
		if !reflections[key] && !protected[key] {
			break
		}
		streak++
	}
	return streak
}

func thailandDate(value time.Time) time.Time {
	location, err := time.LoadLocation("Asia/Bangkok")
	if err != nil {
		location = time.FixedZone("Asia/Bangkok", 7*60*60)
	}
	localized := value.In(location)
	return time.Date(localized.Year(), localized.Month(), localized.Day(), 0, 0, 0, 0, location)
}

func isWeekend(value time.Time) bool {
	return value.Weekday() == time.Saturday || value.Weekday() == time.Sunday
}

func stringSet(values []string) map[string]bool {
	result := make(map[string]bool, len(values))
	for _, value := range values {
		result[value] = true
	}
	return result
}
