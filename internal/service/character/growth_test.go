package character

import (
	"context"
	"testing"
	"time"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestCalculateGrowthSnapshot(t *testing.T) {
	now := time.Date(2026, time.September, 29, 9, 0, 0, 0, time.UTC)
	cases := []struct {
		name        string
		reflections []string
		protected   []string
		holidays    []string
		best        int
		current     int
		mood        string
	}{
		{name: "empty", best: 0, current: 0, mood: "resting"},
		{name: "weekend and grace", reflections: []string{"2026-09-25", "2026-09-28"}, best: 2, current: 2, mood: "active"},
		{name: "holiday and protected day", reflections: []string{"2026-09-22", "2026-09-24", "2026-09-28"}, protected: []string{"2026-09-25"}, holidays: []string{"2026-09-23"}, best: 4, current: 4, mood: "active"},
		{name: "broken streak keeps best", reflections: []string{"2026-09-14", "2026-09-15", "2026-09-16", "2026-09-17", "2026-09-18", "2026-09-21", "2026-09-22"}, best: 7, current: 0, mood: "resting"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := CalculateGrowthSnapshot(now, tc.reflections, tc.protected, tc.holidays)
			if result.BestStreak != tc.best || result.CurrentStreak != tc.current || result.Mood != tc.mood {
				t.Fatalf("growth = %+v", result)
			}
		})
	}
}

func TestGrowthUsesThailandCalendarDay(t *testing.T) {
	result := CalculateGrowthSnapshot(time.Date(2026, time.September, 28, 17, 30, 0, 0, time.UTC), []string{"2026-09-29"}, nil, nil)
	if result.CurrentStreak != 1 || result.Mood != "active" {
		t.Fatalf("Thai day growth = %+v", result)
	}
}

func TestGrowthMilestoneBoundaries(t *testing.T) {
	for _, tc := range []struct{ days, form, detail int }{{0, 0, 1}, {1, 0, 2}, {7, 1, 4}, {30, 2, 7}, {75, 3, 9}, {100, 3, 10}} {
		dates := make([]string, 0, tc.days)
		cursor := time.Date(2026, time.September, 29, 0, 0, 0, 0, time.UTC)
		for len(dates) < tc.days {
			if cursor.Weekday() != time.Saturday && cursor.Weekday() != time.Sunday {
				dates = append(dates, cursor.Format("2006-01-02"))
			}
			cursor = cursor.AddDate(0, 0, -1)
		}
		result := CalculateGrowthSnapshot(time.Date(2026, time.September, 29, 9, 0, 0, 0, time.UTC), dates, nil, nil)
		if result.FormIndex != tc.form || result.DetailIndex != tc.detail {
			t.Fatalf("%d workdays => form %d detail %d, want %d %d", tc.days, result.FormIndex, result.DetailIndex, tc.form, tc.detail)
		}
	}
}

type growthUserStore struct{ user *domain.User }

func (s growthUserStore) FindByID(_ interface{}, _ primitive.ObjectID) (*domain.User, error) {
	return s.user, nil
}

type growthHolidayStore struct{ dates map[string]bool }

func (s growthHolidayStore) GetHolidayDatesInRange(_, _ string) (map[string]bool, error) {
	return s.dates, nil
}

func TestGrowthSnapshotUsesAccountReflectionsAndProtection(t *testing.T) {
	user := &domain.User{ID: primitive.NewObjectID(), Reflections: []domain.Reflection{{Day: "2026-09-25"}, {Day: "2026-09-28"}}}
	service := NewGrowthService(growthUserStore{user}, growthHolidayStore{})
	result, err := service.Snapshot(context.Background(), user.ID.Hex(), time.Date(2026, time.September, 29, 9, 0, 0, 0, time.UTC))
	if err != nil || result.BestStreak != 2 {
		t.Fatalf("growth = %+v, %v", result, err)
	}
}
