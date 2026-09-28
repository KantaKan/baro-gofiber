package achievement

import (
	"context"
	"errors"
	"fmt"
	"sort"
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

type Entitlement struct {
	Identity      string
	Scope         string
	RepeatPolicy  string
	MinimumRarity string
	Message       string
}

type Definition struct {
	Identity     string
	RepeatPolicy string
}

var repeatPolicies = map[string]string{
	"weekly-consistency": "once-per-iso-week",
	"songkran-growth":    "once-per-year",
	"gentle-comeback":    "once",
	"quiet-gardener":     "once",
	"social-gardener":    "once",
}

func Definitions() []Definition {
	result := make([]Definition, 0, len(repeatPolicies))
	for identity, policy := range repeatPolicies {
		result = append(result, Definition{Identity: identity, RepeatPolicy: policy})
	}
	sort.Slice(result, func(left, right int) bool { return result[left].Identity < result[right].Identity })
	return result
}

type Service struct {
	users    UserStore
	rewards  RewardStore
	holidays HolidayCalendar
}

func NewService(users UserStore, rewards RewardStore, holidays HolidayCalendar) *Service {
	return &Service{users: users, rewards: rewards, holidays: holidays}
}

func (s *Service) Reconcile(ctx context.Context, userID primitive.ObjectID, now time.Time) ([]domain.TeacherGiftBox, error) {
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if len(user.Reflections) == 0 {
		return []domain.TeacherGiftBox{}, nil
	}
	dates := reflectionDates(user.Reflections)
	if len(dates) == 0 {
		return []domain.TeacherGiftBox{}, nil
	}
	holidayMap, err := s.holidays.GetHolidayDatesInRange(dates[0], dates[len(dates)-1])
	if err != nil {
		return nil, err
	}
	protected := make([]string, 0, len(user.FertilizerLog))
	for _, entry := range user.FertilizerLog {
		if entry.Kind == "protect" && entry.RelatedDate != "" {
			protected = append(protected, entry.RelatedDate)
		}
	}
	entitlements := EligibleReflectionAchievements(user.Reflections, protected, mapKeys(holidayMap))
	return s.grant(ctx, userID, entitlements, now)
}

func (s *Service) RecordSocial(ctx context.Context, userID primitive.ObjectID, event string, now time.Time) (*domain.TeacherGiftBox, error) {
	if event != "garden-cheer" && event != "profile-reaction" && event != "fertilizer-gift" && event != "streak-rescue" {
		return nil, errors.New("unsupported social achievement event")
	}
	boxes, err := s.grant(ctx, userID, []Entitlement{
		newEntitlement("social-gardener", "", "Epic", "You helped another learner's garden feel more alive."),
	}, now)
	if err != nil || len(boxes) == 0 {
		return nil, err
	}
	return &boxes[0], nil
}

func (s *Service) grant(ctx context.Context, userID primitive.ObjectID, entitlements []Entitlement, now time.Time) ([]domain.TeacherGiftBox, error) {
	granted := []domain.TeacherGiftBox{}
	for _, entitlement := range entitlements {
		key := fmt.Sprintf("achievement:%s:%s", entitlement.Identity, userID.Hex())
		if entitlement.Scope != "" {
			key = fmt.Sprintf("achievement:%s:%s:%s", entitlement.Identity, entitlement.Scope, userID.Hex())
		}
		box := domain.TeacherGiftBox{
			ID: primitive.NewObjectID(), UserID: userID, MinimumRarity: entitlement.MinimumRarity,
			Message: entitlement.Message, Status: "unopened", CreatedAt: now, GrantKey: key, Source: "achievement",
		}
		created, err := s.rewards.CreateOnce(ctx, box)
		if err != nil {
			return granted, err
		}
		if created {
			granted = append(granted, box)
		}
	}
	return granted, nil
}

func EligibleReflectionAchievements(reflections []domain.Reflection, protectedDates, holidayDates []string) []Entitlement {
	dates := reflectionDates(reflections)
	if len(dates) == 0 {
		return []Entitlement{}
	}
	holidays := valueSet(holidayDates)
	activity := valueSet(dates)
	for _, date := range protectedDates {
		activity[date] = true
	}
	result := []Entitlement{}
	weekly := map[string]map[string]bool{}
	for _, date := range dates {
		parsed, err := time.Parse("2006-01-02", date)
		if err != nil || isWeekend(parsed) || holidays[date] {
			continue
		}
		year, week := parsed.ISOWeek()
		scope := fmt.Sprintf("%d-W%02d", year, week)
		if weekly[scope] == nil {
			weekly[scope] = map[string]bool{}
		}
		weekly[scope][date] = true
	}
	for scope, reflected := range weekly {
		if len(reflected) == eligibleDaysInISOWeek(scope, holidays) {
			result = append(result, newEntitlement("weekly-consistency", scope, "Common", "A steady week of reflection gave your garden a new surprise."))
		}
	}
	seasonalYears := map[int]bool{}
	for _, date := range dates {
		parsed, err := time.Parse("2006-01-02", date)
		if err == nil && parsed.Month() == time.April && parsed.Day() >= 13 && parsed.Day() <= 15 {
			seasonalYears[parsed.Year()] = true
		}
	}
	for year := range seasonalYears {
		result = append(result, newEntitlement("songkran-growth", fmt.Sprint(year), "Rare", "Your Songkran reflection brought a fresh splash of growth."))
	}
	if hasComeback(activity, holidays) {
		result = append(result, newEntitlement("gentle-comeback", "", "Epic", "Coming back helped your garden grow in a beautiful new direction."))
	}
	if len(valueSet(dates)) >= 25 {
		result = append(result, newEntitlement("quiet-gardener", "", "Legendary", "A hidden garden achievement has quietly bloomed."))
	}
	sort.Slice(result, func(left, right int) bool {
		return result[left].Identity+result[left].Scope < result[right].Identity+result[right].Scope
	})
	return result
}

func newEntitlement(identity, scope, rarity, message string) Entitlement {
	return Entitlement{Identity: identity, Scope: scope, RepeatPolicy: repeatPolicies[identity], MinimumRarity: rarity, Message: message}
}

func reflectionDates(reflections []domain.Reflection) []string {
	unique := map[string]bool{}
	location, _ := time.LoadLocation("Asia/Bangkok")
	for _, reflection := range reflections {
		date := reflection.Day
		if date == "" && !reflection.CreatedAt.IsZero() {
			date = reflection.CreatedAt.In(location).Format("2006-01-02")
		}
		if _, err := time.Parse("2006-01-02", date); err == nil {
			unique[date] = true
		}
	}
	result := mapKeys(unique)
	sort.Strings(result)
	return result
}

func eligibleDaysInISOWeek(scope string, holidays map[string]bool) int {
	var year, week int
	if _, err := fmt.Sscanf(scope, "%d-W%d", &year, &week); err != nil {
		return 5
	}
	date := time.Date(year, 1, 4, 0, 0, 0, 0, time.UTC)
	isoYear, isoWeek := date.ISOWeek()
	for isoYear != year || isoWeek != week {
		date = date.AddDate(0, 0, 1)
		isoYear, isoWeek = date.ISOWeek()
	}
	for date.Weekday() != time.Monday {
		date = date.AddDate(0, 0, -1)
	}
	count := 0
	for offset := 0; offset < 5; offset++ {
		if !holidays[date.AddDate(0, 0, offset).Format("2006-01-02")] {
			count++
		}
	}
	return count
}

func hasComeback(activity, holidays map[string]bool) bool {
	dates := mapKeys(activity)
	sort.Strings(dates)
	consecutive := 0
	var previous time.Time
	for _, value := range dates {
		current, err := time.Parse("2006-01-02", value)
		if err != nil || isWeekend(current) || holidays[value] {
			continue
		}
		if previous.IsZero() {
			consecutive = 1
			previous = current
			continue
		}
		missed := false
		for date := previous.AddDate(0, 0, 1); date.Before(current); date = date.AddDate(0, 0, 1) {
			key := date.Format("2006-01-02")
			if !isWeekend(date) && !holidays[key] && !activity[key] {
				missed = true
				break
			}
		}
		if missed {
			if consecutive >= 3 {
				return true
			}
			consecutive = 1
		} else {
			consecutive++
		}
		previous = current
	}
	return false
}

func isWeekend(value time.Time) bool {
	return value.Weekday() == time.Saturday || value.Weekday() == time.Sunday
}

func valueSet(values []string) map[string]bool {
	result := make(map[string]bool, len(values))
	for _, value := range values {
		result[value] = true
	}
	return result
}

func mapKeys(values map[string]bool) []string {
	result := make([]string, 0, len(values))
	for key := range values {
		result = append(result, key)
	}
	return result
}
