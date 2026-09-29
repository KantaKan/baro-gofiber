package milestone

import (
	"context"
	"testing"
	"time"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func bangkokTime(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func TestCalculateWorkdayStreak(t *testing.T) {
	tests := []struct {
		name        string
		now         time.Time
		reflections []string
		protected   []string
		holidays    []string
		want        int
	}{
		{name: "weekend preserves Friday without adding days", now: bangkokTime(t, "2026-10-04T12:00:00+07:00"), reflections: []string{"2026-10-02"}, want: 1},
		{name: "holiday is skipped instead of counted", now: bangkokTime(t, "2026-10-06T12:00:00+07:00"), reflections: []string{"2026-10-06", "2026-10-02"}, holidays: []string{"2026-10-05"}, want: 2},
		{name: "protected workday counts", now: bangkokTime(t, "2026-10-07T12:00:00+07:00"), reflections: []string{"2026-10-07", "2026-10-05"}, protected: []string{"2026-10-06"}, want: 3},
		{name: "UTC instant uses Thailand calendar day", now: bangkokTime(t, "2026-10-05T18:30:00Z"), reflections: []string{"2026-10-06", "2026-10-05"}, want: 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := CalculateWorkdayStreak(test.now, test.reflections, test.protected, test.holidays); got != test.want {
				t.Fatalf("CalculateWorkdayStreak() = %d, want %d", got, test.want)
			}
		})
	}
}

type milestoneUserStore struct{ user *domain.User }

func (s milestoneUserStore) FindByID(_ interface{}, _ primitive.ObjectID) (*domain.User, error) {
	return s.user, nil
}

type milestoneRewardStore struct {
	keys  map[string]bool
	boxes []domain.TeacherGiftBox
}

func (s *milestoneRewardStore) CreateOnce(_ context.Context, box domain.TeacherGiftBox) (bool, error) {
	if s.keys[box.GrantKey] {
		return false, nil
	}
	s.keys[box.GrantKey] = true
	s.boxes = append(s.boxes, box)
	return true, nil
}

type milestoneHolidayCalendar struct{ dates map[string]bool }

func (c milestoneHolidayCalendar) GetHolidayDatesInRange(_, _ string) (map[string]bool, error) {
	return c.dates, nil
}

func TestReconcileGrantsEachCrossedMilestoneOnce(t *testing.T) {
	userID := primitive.NewObjectID()
	reflectionDays := []int{30, 1, 2, 5, 6, 7, 8, 9}
	reflections := make([]domain.Reflection, 0, len(reflectionDays))
	for index, day := range reflectionDays {
		month := time.October
		if index == 0 {
			month = time.September
		}
		date := time.Date(2026, month, day, 12, 0, 0, 0, time.FixedZone("Asia/Bangkok", 7*60*60))
		reflections = append(reflections, domain.Reflection{Day: date.Format("2006-01-02"), CreatedAt: date})
	}
	rewards := &milestoneRewardStore{keys: map[string]bool{}}
	service := NewService(milestoneUserStore{user: &domain.User{ID: userID, Reflections: reflections}}, rewards, milestoneHolidayCalendar{})

	first, err := service.Reconcile(context.Background(), userID, bangkokTime(t, "2026-10-09T12:00:00+07:00"))
	if err != nil || len(first) != 2 {
		t.Fatalf("first Reconcile = %+v, err=%v", first, err)
	}
	second, err := service.Reconcile(context.Background(), userID, bangkokTime(t, "2026-10-09T12:00:00+07:00"))
	if err != nil || len(second) != 0 || len(rewards.boxes) != 2 {
		t.Fatalf("second Reconcile duplicated rewards: result=%+v boxes=%+v err=%v", second, rewards.boxes, err)
	}
}
