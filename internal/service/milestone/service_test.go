package milestone

import (
	"context"
	"fmt"
	"sync"
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
	mu    sync.Mutex
	keys  map[string]bool
	boxes []domain.TeacherGiftBox
}

func (s *milestoneRewardStore) CreateOnce(_ context.Context, box domain.TeacherGiftBox) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.keys[box.GrantKey] {
		return false, nil
	}
	s.keys[box.GrantKey] = true
	s.boxes = append(s.boxes, box)
	return true, nil
}

func (s *milestoneRewardStore) CreateCharacterEggOnce(ctx context.Context, box domain.TeacherGiftBox) (bool, error) {
	return s.CreateOnce(ctx, box)
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
	service := NewService(milestoneUserStore{user: &domain.User{ID: userID, Role: "learner", Reflections: reflections}}, rewards, milestoneHolidayCalendar{})

	first, err := service.Reconcile(context.Background(), userID, bangkokTime(t, "2026-10-09T12:00:00+07:00"))
	if err != nil || len(first) != 2 {
		t.Fatalf("first Reconcile = %+v, err=%v", first, err)
	}
	second, err := service.Reconcile(context.Background(), userID, bangkokTime(t, "2026-10-09T12:00:00+07:00"))
	if err != nil || len(second) != 0 || len(rewards.boxes) != 2 {
		t.Fatalf("second Reconcile duplicated rewards: result=%+v boxes=%+v err=%v", second, rewards.boxes, err)
	}
}

func reflectionWorkdays(now time.Time, count int) []domain.Reflection {
	result := make([]domain.Reflection, 0, count)
	current := thailandDate(now)
	for len(result) < count {
		if !isWeekend(current) {
			result = append(result, domain.Reflection{Day: current.Format("2006-01-02"), CreatedAt: current})
		}
		current = current.AddDate(0, 0, -1)
	}
	return result
}

func TestMajorReflectionMilestonesGrantStandardEggsAtBoundaries(t *testing.T) {
	now := bangkokTime(t, "2026-09-30T12:00:00+07:00")
	tests := []struct {
		workdays int
		wantEggs int
	}{
		{29, 0}, {30, 1}, {31, 1},
		{59, 1}, {60, 2}, {61, 2},
		{99, 2}, {100, 3}, {101, 3},
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("%d workdays", test.workdays), func(t *testing.T) {
			userID := primitive.NewObjectID()
			rewards := &milestoneRewardStore{keys: map[string]bool{}}
			user := &domain.User{ID: userID, Role: "learner", Reflections: reflectionWorkdays(now, test.workdays)}
			service := NewService(milestoneUserStore{user: user}, rewards, milestoneHolidayCalendar{})
			if _, err := service.Reconcile(context.Background(), userID, now); err != nil {
				t.Fatal(err)
			}
			eggs := 0
			for _, box := range rewards.boxes {
				if box.RewardPool == "character-egg" {
					eggs++
					if box.MinimumRarity != "Common" || box.Source != "reflection-milestone" {
						t.Fatalf("milestone Egg = %+v", box)
					}
				}
			}
			if eggs != test.wantEggs {
				t.Fatalf("Eggs = %d, want %d", eggs, test.wantEggs)
			}
		})
	}
}

func TestMajorMilestoneEggReconciliationIsConcurrentSafe(t *testing.T) {
	now := bangkokTime(t, "2026-09-30T12:00:00+07:00")
	userID := primitive.NewObjectID()
	rewards := &milestoneRewardStore{keys: map[string]bool{}}
	user := &domain.User{ID: userID, Role: "learner", Reflections: reflectionWorkdays(now, 100)}
	service := NewService(milestoneUserStore{user: user}, rewards, milestoneHolidayCalendar{})
	var group sync.WaitGroup
	for range 8 {
		group.Add(1)
		go func() {
			defer group.Done()
			if _, err := service.Reconcile(context.Background(), userID, now); err != nil {
				t.Errorf("Reconcile() error = %v", err)
			}
		}()
	}
	group.Wait()
	eggs := 0
	for _, box := range rewards.boxes {
		if box.RewardPool == "character-egg" {
			eggs++
		}
	}
	if eggs != 3 {
		t.Fatalf("concurrent milestone Eggs = %d, want 3", eggs)
	}
}

func TestMilestoneEggsSkipIneligibleAccounts(t *testing.T) {
	now := bangkokTime(t, "2026-09-30T12:00:00+07:00")
	for _, user := range []*domain.User{
		{ID: primitive.NewObjectID(), Role: "admin", Reflections: reflectionWorkdays(now, 100)},
		{ID: primitive.NewObjectID(), Role: "learner", Deleted: true, Reflections: reflectionWorkdays(now, 100)},
	} {
		rewards := &milestoneRewardStore{keys: map[string]bool{}}
		service := NewService(milestoneUserStore{user: user}, rewards, milestoneHolidayCalendar{})
		if _, err := service.Reconcile(context.Background(), user.ID, now); err != nil {
			t.Fatal(err)
		}
		cosmetic := 0
		for _, box := range rewards.boxes {
			if box.RewardPool == "character-egg" {
				t.Fatalf("ineligible account received a milestone Egg: %+v", box)
			}
			cosmetic++
		}
		if user.Role == "admin" && cosmetic != len(rewardMilestones) {
			t.Fatalf("admin cosmetic milestone boxes = %d, want the unchanged %d", cosmetic, len(rewardMilestones))
		}
	}
}

func TestMilestoneEggsReconcileAPastStreakAfterItBreaks(t *testing.T) {
	now := bangkokTime(t, "2026-09-30T12:00:00+07:00")
	past := reflectionWorkdays(bangkokTime(t, "2026-08-14T12:00:00+07:00"), 32)
	recent := reflectionWorkdays(now, 2)
	userID := primitive.NewObjectID()
	rewards := &milestoneRewardStore{keys: map[string]bool{}}
	user := &domain.User{ID: userID, Role: "learner", Reflections: append(recent, past...)}
	service := NewService(milestoneUserStore{user: user}, rewards, milestoneHolidayCalendar{})
	for range 3 {
		if _, err := service.Reconcile(context.Background(), userID, now); err != nil {
			t.Fatal(err)
		}
	}
	eggs := 0
	for _, box := range rewards.boxes {
		if box.RewardPool == "character-egg" {
			eggs++
		}
	}
	if eggs != 1 {
		t.Fatalf("a past 32-workday streak granted %d Eggs, want 1", eggs)
	}
}
