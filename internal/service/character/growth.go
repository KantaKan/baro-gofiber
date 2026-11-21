package character

import (
	"context"
	"errors"
	"sort"
	"time"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type GrowthUserStore interface {
	FindByID(ctx interface{}, id primitive.ObjectID) (*domain.User, error)
}

type GrowthHolidayCalendar interface {
	GetHolidayDatesInRange(startDate, endDate string) (map[string]bool, error)
}

type GrowthService struct {
	users    GrowthUserStore
	holidays GrowthHolidayCalendar
}

func NewGrowthService(users GrowthUserStore, holidays GrowthHolidayCalendar) *GrowthService {
	return &GrowthService{users: users, holidays: holidays}
}

func (s *GrowthService) Snapshot(ctx context.Context, ownerHex string, now time.Time) (domain.CharacterGrowthSnapshot, error) {
	ownerID, err := primitive.ObjectIDFromHex(ownerHex)
	if err != nil {
		return domain.CharacterGrowthSnapshot{}, errors.New("invalid account ID")
	}
	user, err := s.users.FindByID(ctx, ownerID)
	if err != nil {
		return domain.CharacterGrowthSnapshot{}, err
	}
	if user == nil || user.Deleted {
		return domain.CharacterGrowthSnapshot{}, ErrAccountNotFound
	}
	reflections := make([]string, 0, len(user.Reflections))
	protected := make([]string, 0)
	today := thaiDay(now)
	earliest := today
	for _, reflection := range user.Reflections {
		key := validDay(reflection.Day)
		if key == "" {
			date := reflection.Date
			if date.IsZero() {
				date = reflection.CreatedAt
			}
			if !date.IsZero() {
				key = thaiDay(date).Format("2006-01-02")
			}
		}
		if key == "" {
			continue
		}
		reflections = append(reflections, key)
		parsed, _ := time.ParseInLocation("2006-01-02", key, today.Location())
		if parsed.Before(earliest) {
			earliest = parsed
		}
	}
	for _, entry := range user.CareEnergyLog {
		if entry.Kind != "protect" {
			continue
		}
		key := validDay(entry.RelatedDate)
		if key == "" {
			continue
		}
		protected = append(protected, key)
		parsed, _ := time.ParseInLocation("2006-01-02", key, today.Location())
		if parsed.Before(earliest) {
			earliest = parsed
		}
	}
	holidayMap, err := s.holidays.GetHolidayDatesInRange(earliest.Format("2006-01-02"), today.Format("2006-01-02"))
	if err != nil {
		return domain.CharacterGrowthSnapshot{}, err
	}
	holidays := make([]string, 0, len(holidayMap))
	for key := range holidayMap {
		holidays = append(holidays, key)
	}
	return CalculateGrowthSnapshot(now, reflections, protected, holidays), nil
}

func CalculateGrowthSnapshot(now time.Time, reflectionDates, protectedDates, holidayDates []string) domain.CharacterGrowthSnapshot {
	today := thaiDay(now)
	todayKey := today.Format("2006-01-02")
	holidays := make(map[string]bool, len(holidayDates))
	for _, raw := range holidayDates {
		if key := validDay(raw); key != "" {
			holidays[key] = true
		}
	}
	isOff := func(day time.Time) bool {
		return day.Weekday() == time.Saturday || day.Weekday() == time.Sunday || holidays[day.Format("2006-01-02")]
	}
	previousWorkday := func(day time.Time) time.Time {
		for day = day.AddDate(0, 0, -1); isOff(day); day = day.AddDate(0, 0, -1) {
		}
		return day
	}
	active := map[string]bool{}
	for _, raw := range append(append([]string{}, reflectionDates...), protectedDates...) {
		key := validDay(raw)
		if key != "" && key <= todayKey {
			active[key] = true
		}
	}
	keys := make([]string, 0, len(active))
	for key := range active {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	best, run := 0, 0
	var prior time.Time
	for _, key := range keys {
		day, _ := time.ParseInLocation("2006-01-02", key, today.Location())
		if isOff(day) {
			continue
		}
		if !prior.IsZero() && previousWorkday(day).Equal(prior) {
			run++
		} else {
			run = 1
		}
		if run > best {
			best = run
		}
		prior = day
	}
	latest := today
	for isOff(latest) {
		latest = latest.AddDate(0, 0, -1)
	}
	start := latest
	if !active[latest.Format("2006-01-02")] {
		start = previousWorkday(latest)
	}
	current := 0
	if active[start.Format("2006-01-02")] {
		for day := start; active[day.Format("2006-01-02")]; day = previousWorkday(day) {
			current++
		}
	}
	mood := "resting"
	if current > 0 {
		mood = "active"
	}
	return domain.CharacterGrowthSnapshot{CharacterGrowth: domain.ResolveCharacterGrowth(best), CurrentStreak: current, Mood: mood}
}

func validDay(raw string) string {
	if len(raw) < 10 {
		return ""
	}
	key := raw[:10]
	if _, err := time.Parse("2006-01-02", key); err != nil {
		return ""
	}
	return key
}

func thaiDay(value time.Time) time.Time {
	location, err := time.LoadLocation("Asia/Bangkok")
	if err != nil {
		location = time.FixedZone("Asia/Bangkok", 7*60*60)
	}
	local := value.In(location)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
}
