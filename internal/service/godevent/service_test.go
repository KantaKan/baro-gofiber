package godevent

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type memoryStore struct {
	viewers   map[primitive.ObjectID]Viewer
	cohorts   map[int]bool
	character *domain.BaroCharacter
	events    []domain.GodEvent
}

func (s *memoryStore) Viewer(_ context.Context, id primitive.ObjectID) (*Viewer, error) {
	viewer, ok := s.viewers[id]
	if !ok {
		return nil, nil
	}
	return &viewer, nil
}
func (s *memoryStore) CohortExists(_ context.Context, cohort int) (bool, error) {
	return s.cohorts[cohort], nil
}
func (s *memoryStore) AdminCharacter(context.Context, primitive.ObjectID) (*domain.BaroCharacter, error) {
	if s.character == nil {
		return nil, nil
	}
	copy := *s.character
	return &copy, nil
}
func (s *memoryStore) Create(_ context.Context, event domain.GodEvent) error {
	s.events = append(s.events, event)
	return nil
}
func (s *memoryStore) ListVisible(_ context.Context, viewer Viewer) ([]domain.GodEvent, error) {
	items := []domain.GodEvent{}
	for index := len(s.events) - 1; index >= 0; index-- {
		event := s.events[index]
		if viewer.Role == "admin" || event.Cohort == 0 || event.Cohort == viewer.Cohort {
			items = append(items, event)
		}
	}
	return items, nil
}

func TestCastPresetsAudienceHistoryAnd24HourBoundary(t *testing.T) {
	admin := primitive.NewObjectID()
	learner16 := primitive.NewObjectID()
	learner17 := primitive.NewObjectID()
	character := domain.BaroCharacter{ID: primitive.NewObjectID(), OwnerID: admin, Serial: "B-GOD"}
	store := &memoryStore{
		viewers: map[primitive.ObjectID]Viewer{admin: {Role: "admin"}, learner16: {Role: "learner", Cohort: 16}, learner17: {Role: "learner", Cohort: 17}},
		cohorts: map[int]bool{16: true, 17: true}, character: &character,
	}
	service := NewService(store)
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	for _, item := range []struct {
		preset string
		cohort int
	}{{"star_rain", 0}, {"god_entrance", 16}, {"character_parade", 17}} {
		cast, err := service.Cast(context.Background(), admin.Hex(), item.preset, "  You made today brighter ✨  ", item.cohort)
		if err != nil || !cast.Active || cast.Caption != "You made today brighter ✨" || cast.CastBy != admin || cast.ActiveUntil.Sub(cast.CreatedAt) != 24*time.Hour || cast.Character == nil {
			t.Fatalf("cast = %+v, err=%v", cast, err)
		}
	}
	store.character.Serial = "B-CHANGED"
	if store.events[0].Character.Serial != "B-GOD" {
		t.Fatal("historical character snapshot changed")
	}
	for _, test := range []struct {
		id   primitive.ObjectID
		want int
	}{{learner16, 2}, {learner17, 2}, {admin, 3}} {
		items, err := service.List(context.Background(), test.id.Hex())
		if err != nil || len(items) != test.want {
			t.Fatalf("history for %s = %+v, err=%v", test.id.Hex(), items, err)
		}
		for _, item := range items {
			if !item.Active {
				t.Fatalf("event not active at cast time: %+v", item)
			}
		}
	}
	now = now.Add(24 * time.Hour)
	items, err := service.List(context.Background(), learner16.Hex())
	if err != nil || len(items) != 2 || items[0].Active || items[1].Active {
		t.Fatalf("24-hour boundary = %+v, err=%v", items, err)
	}
	now = now.Add(7 * 24 * time.Hour)
	items, err = service.List(context.Background(), learner17.Hex())
	if err != nil || len(items) != 2 || items[0].Active {
		t.Fatalf("replay history = %+v, err=%v", items, err)
	}
}

func TestCastRejectsUnauthorizedUnsafeOrUnknownAudience(t *testing.T) {
	admin := primitive.NewObjectID()
	learner := primitive.NewObjectID()
	store := &memoryStore{viewers: map[primitive.ObjectID]Viewer{admin: {Role: "admin"}, learner: {Role: "learner", Cohort: 16}}, cohorts: map[int]bool{16: true}}
	service := NewService(store)
	for _, test := range []struct {
		actor, preset, caption string
		cohort                 int
		want                   error
	}{
		{learner.Hex(), "star_rain", "Nice work", 16, ErrAdminRequired},
		{admin.Hex(), "unknown", "Nice work", 16, nil},
		{admin.Hex(), "star_rain", "<script>alert(1)</script>", 16, nil},
		{admin.Hex(), "star_rain", strings.Repeat("ก", 161), 16, nil},
		{admin.Hex(), "star_rain", "Hello", 999, ErrAudienceNotFound},
	} {
		_, err := service.Cast(context.Background(), test.actor, test.preset, test.caption, test.cohort)
		if err == nil || (test.want != nil && !errors.Is(err, test.want)) {
			t.Fatalf("cast %q cohort %d error = %v", test.preset, test.cohort, err)
		}
	}
	if len(store.events) != 0 {
		t.Fatalf("invalid casts persisted: %+v", store.events)
	}
}
