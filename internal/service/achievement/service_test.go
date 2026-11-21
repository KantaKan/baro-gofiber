package achievement

import (
	"context"
	"fmt"
	"testing"
	"time"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type achievementUserStore struct{ user *domain.User }

func (s achievementUserStore) FindByID(_ interface{}, _ primitive.ObjectID) (*domain.User, error) {
	return s.user, nil
}

type achievementRewardStore struct {
	keys  map[string]bool
	boxes []domain.TeacherGiftBox
}

func (s *achievementRewardStore) CreateOnce(_ context.Context, box domain.TeacherGiftBox) (bool, error) {
	if s.keys[box.GrantKey] {
		return false, nil
	}
	s.keys[box.GrantKey] = true
	s.boxes = append(s.boxes, box)
	return true, nil
}

type achievementHolidayCalendar struct{ dates map[string]bool }

func (c achievementHolidayCalendar) GetHolidayDatesInRange(_, _ string) (map[string]bool, error) {
	return c.dates, nil
}

func reflections(dates ...string) []domain.Reflection {
	result := make([]domain.Reflection, 0, len(dates))
	for _, date := range dates {
		result = append(result, domain.Reflection{Day: date})
	}
	return result
}

func TestReconcileReflectionAchievementsAndRepeatPolicies(t *testing.T) {
	userID := primitive.NewObjectID()
	dates := []string{"2026-04-13", "2026-09-28", "2026-09-29", "2026-09-30", "2026-10-02", "2026-10-05", "2026-10-06", "2026-10-07", "2026-10-08", "2026-10-09"}
	for day := 1; day <= 16; day++ {
		dates = append(dates, fmt.Sprintf("2026-06-%02d", day))
	}
	store := &achievementRewardStore{keys: map[string]bool{}}
	service := NewService(
		achievementUserStore{user: &domain.User{ID: userID, Reflections: reflections(dates...)}},
		store,
		achievementHolidayCalendar{dates: map[string]bool{"2026-10-01": true}},
	)

	first, err := service.Reconcile(context.Background(), userID, mustTime(t, "2026-10-09T12:00:00+07:00"))
	if err != nil || len(first) != 7 {
		t.Fatalf("Reconcile() = %+v, err=%v", first, err)
	}
	second, err := service.Reconcile(context.Background(), userID, mustTime(t, "2026-10-09T12:00:00+07:00"))
	if err != nil || len(second) != 0 || len(store.boxes) != 7 {
		t.Fatalf("replayed reconciliation duplicated rewards: result=%+v boxes=%+v err=%v", second, store.boxes, err)
	}
}

func TestReflectionEvaluationIgnoresPrivateContentAndZone(t *testing.T) {
	dates := []string{"2026-09-28", "2026-09-29", "2026-09-30", "2026-10-01", "2026-10-02"}
	left := reflections(dates...)
	right := reflections(dates...)
	for index := range left {
		left[index].ReflectionData.Barometer = "Comfort Zone"
		left[index].ReflectionData.TechSessions.Happy = "private left text"
		right[index].ReflectionData.Barometer = "Panic Zone"
		right[index].ReflectionData.TechSessions.Happy = "different private text"
	}
	if fmt.Sprint(EligibleReflectionAchievements(left, nil, nil)) != fmt.Sprint(EligibleReflectionAchievements(right, nil, nil)) {
		t.Fatal("private reflection content changed achievement eligibility")
	}
}

func TestRecordSocialIsOneTimeAcrossReplayedEvents(t *testing.T) {
	userID := primitive.NewObjectID()
	store := &achievementRewardStore{keys: map[string]bool{}}
	service := NewService(achievementUserStore{}, store, achievementHolidayCalendar{})
	first, err := service.RecordSocial(context.Background(), userID, "garden-cheer", mustTime(t, "2026-10-09T12:00:00+07:00"))
	if err != nil || first == nil {
		t.Fatalf("first RecordSocial() = %+v, err=%v", first, err)
	}
	second, err := service.RecordSocial(context.Background(), userID, "garden-cheer", mustTime(t, "2026-10-09T12:00:00+07:00"))
	if err != nil || second != nil || len(store.boxes) != 1 {
		t.Fatalf("replayed social event duplicated reward: result=%+v boxes=%+v err=%v", second, store.boxes, err)
	}
}

func TestAchievementDefinitionsHaveStableRepeatPolicies(t *testing.T) {
	want := map[string]string{
		"weekly-consistency": "once-per-iso-week",
		"songkran-growth":    "once-per-year",
		"gentle-comeback":    "once",
		"quiet-gardener":     "once",
		"social-gardener":    "once",
	}
	for _, definition := range Definitions() {
		if want[definition.Identity] != definition.RepeatPolicy {
			t.Fatalf("definition %+v has an unstable repeat policy", definition)
		}
		delete(want, definition.Identity)
	}
	if len(want) != 0 {
		t.Fatalf("missing definitions: %+v", want)
	}
}

func mustTime(t *testing.T, value string) time.Time {
	t.Helper()
	result, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatal(err)
	}
	return result
}
