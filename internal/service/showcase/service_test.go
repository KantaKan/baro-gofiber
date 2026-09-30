package showcase

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
	viewers    map[primitive.ObjectID]Viewer
	owned      map[primitive.ObjectID]domain.BaroCharacter
	entries    map[primitive.ObjectID]Entry
	equippedID primitive.ObjectID
	reactions  map[string]bool
	moderation []string
}

func (s *memoryStore) Viewer(_ context.Context, id primitive.ObjectID) (*Viewer, error) {
	viewer, ok := s.viewers[id]
	if !ok {
		return nil, nil
	}
	return &viewer, nil
}

func (s *memoryStore) FindOwned(_ context.Context, ownerID, characterID primitive.ObjectID) (*domain.BaroCharacter, error) {
	character, ok := s.owned[characterID]
	if !ok || character.OwnerID != ownerID {
		return nil, nil
	}
	return &character, nil
}

func (s *memoryStore) Save(_ context.Context, ownerID primitive.ObjectID, characterID *primitive.ObjectID, message string, at time.Time) error {
	if _, ok := s.viewers[ownerID]; !ok {
		return ErrAccountNotFound
	}
	if characterID == nil {
		delete(s.entries, ownerID)
		return nil
	}
	character := s.owned[*characterID]
	entry := s.entries[ownerID]
	entry.OwnerID = ownerID.Hex()
	entry.Character = character
	entry.Message = message
	entry.UpdatedAt = at
	s.entries[ownerID] = entry
	return nil
}

func (s *memoryStore) List(_ context.Context, cohort int, team string, includeHidden bool, viewerID primitive.ObjectID) ([]Entry, error) {
	items := []Entry{}
	for ownerID, entry := range s.entries {
		if (cohort == 0 || entry.Cohort == cohort) && (team == "" || entry.Team == team) && (includeHidden || !entry.Hidden) {
			entry.Reactions = []ReactionSummary{}
			for _, emoji := range []string{"❤️", "✨", "😂", "🙌"} {
				count := 0
				for key, active := range s.reactions {
					if active && strings.HasPrefix(key, ownerID.Hex()+":") && strings.HasSuffix(key, ":"+emoji) {
						count++
					}
				}
				entry.Reactions = append(entry.Reactions, ReactionSummary{Emoji: emoji, Count: count, Reacted: s.reactions[ownerID.Hex()+":"+viewerID.Hex()+":"+emoji]})
			}
			items = append(items, entry)
		}
	}
	return items, nil
}

func (s *memoryStore) Own(_ context.Context, ownerID primitive.ObjectID) (*Entry, error) {
	entry, ok := s.entries[ownerID]
	if !ok {
		return nil, nil
	}
	return &entry, nil
}

func (s *memoryStore) ReactionTarget(_ context.Context, ownerID primitive.ObjectID) (*Target, error) {
	entry, ok := s.entries[ownerID]
	if !ok {
		return nil, nil
	}
	return &Target{Cohort: entry.Cohort, Hidden: entry.Hidden}, nil
}

func (s *memoryStore) ToggleReaction(_ context.Context, ownerID, actorID primitive.ObjectID, emoji string) (bool, error) {
	key := ownerID.Hex() + ":" + actorID.Hex() + ":" + emoji
	s.reactions[key] = !s.reactions[key]
	return s.reactions[key], nil
}

func (s *memoryStore) SetMood(_ context.Context, ownerID primitive.ObjectID, mood string, until time.Time) error {
	entry, ok := s.entries[ownerID]
	if !ok {
		return ErrEntryNotFound
	}
	entry.Mood, entry.MoodUntil = mood, &until
	if mood == "" {
		entry.MoodUntil = nil
	}
	s.entries[ownerID] = entry
	return nil
}

func (s *memoryStore) Moderate(_ context.Context, ownerID, adminID primitive.ObjectID, hidden bool, reason string, at time.Time) error {
	entry, ok := s.entries[ownerID]
	if !ok {
		return ErrEntryNotFound
	}
	entry.Hidden = hidden
	s.entries[ownerID] = entry
	s.moderation = append(s.moderation, adminID.Hex()+":"+reason)
	return nil
}

func TestSaveEditRemoveKeepsOneOwnedPinIndependentOfEquip(t *testing.T) {
	owner := primitive.NewObjectID()
	first := primitive.NewObjectID()
	second := primitive.NewObjectID()
	other := primitive.NewObjectID()
	store := &memoryStore{
		viewers: map[primitive.ObjectID]Viewer{owner: {Cohort: 16, Role: "learner"}},
		owned: map[primitive.ObjectID]domain.BaroCharacter{
			first: {ID: first, OwnerID: owner}, second: {ID: second, OwnerID: owner}, other: {ID: other, OwnerID: primitive.NewObjectID()},
		},
		entries:    map[primitive.ObjectID]Entry{owner: {Cohort: 16, Team: "Garden Alpha"}},
		equippedID: first,
	}
	service := NewService(store)
	if err := service.Save(context.Background(), owner.Hex(), other.Hex(), "not mine"); !errors.Is(err, ErrCharacterNotOwned) {
		t.Fatalf("foreign pin error = %v", err)
	}
	if err := service.Save(context.Background(), owner.Hex(), second.Hex(), strings.Repeat("ก", 161)); err == nil {
		t.Fatal("overlong Unicode message accepted")
	}
	if err := service.Save(context.Background(), owner.Hex(), second.Hex(), "  สวัสดีเพื่อน ๆ  "); err != nil {
		t.Fatal(err)
	}
	if got := store.entries[owner]; got.Character.ID != second || got.Message != "สวัสดีเพื่อน ๆ" || store.equippedID != first {
		t.Fatalf("pin or equipped character changed incorrectly: %+v", got)
	}
	if err := service.Save(context.Background(), owner.Hex(), first.Hex(), "new note"); err != nil {
		t.Fatal(err)
	}
	if len(store.entries) != 1 || store.entries[owner].Character.ID != first {
		t.Fatalf("more than one pin: %+v", store.entries)
	}
	if err := service.Remove(context.Background(), owner.Hex()); err != nil || len(store.entries) != 0 || store.equippedID != first {
		t.Fatalf("remove changed equipment or left a pin: %+v, err=%v", store.entries, err)
	}
}

func TestLearnerSeesOnlyOwnCohortAndCanFilterTeam(t *testing.T) {
	learner := primitive.NewObjectID()
	admin := primitive.NewObjectID()
	store := &memoryStore{
		viewers: map[primitive.ObjectID]Viewer{learner: {Cohort: 16, Role: "learner"}, admin: {Cohort: 0, Role: "admin"}},
		entries: map[primitive.ObjectID]Entry{
			primitive.NewObjectID(): {Cohort: 16, Team: "Alpha"},
			primitive.NewObjectID(): {Cohort: 16, Team: "Beta"},
			primitive.NewObjectID(): {Cohort: 17, Team: "Alpha"},
		},
	}
	service := NewService(store)
	items, err := service.List(context.Background(), learner.Hex(), false, 0, "Alpha", false)
	if err != nil || len(items) != 1 || items[0].Cohort != 16 {
		t.Fatalf("learner team = %+v, err=%v", items, err)
	}
	if _, err := service.List(context.Background(), learner.Hex(), false, 17, "", false); !errors.Is(err, ErrCohortForbidden) {
		t.Fatalf("other cohort error = %v", err)
	}
	items, err = service.List(context.Background(), admin.Hex(), true, 0, "", false)
	if err != nil || len(items) != 3 {
		t.Fatalf("admin all cohorts = %+v, err=%v", items, err)
	}
	items, err = service.List(context.Background(), admin.Hex(), true, 17, "", false)
	if err != nil || len(items) != 1 || items[0].Cohort != 17 {
		t.Fatalf("admin cohort filter = %+v, err=%v", items, err)
	}
	items, err = service.List(context.Background(), learner.Hex(), true, 0, "", false)
	if err != nil || len(items) != 2 {
		t.Fatalf("forged admin scope = %+v, err=%v", items, err)
	}
}

func TestReactionToggleAndModerationKeepOwnerMessageButHidePublicEntry(t *testing.T) {
	owner := primitive.NewObjectID()
	peer := primitive.NewObjectID()
	outsider := primitive.NewObjectID()
	admin := primitive.NewObjectID()
	characterID := primitive.NewObjectID()
	store := &memoryStore{
		viewers: map[primitive.ObjectID]Viewer{
			owner: {Cohort: 16, Role: "learner"}, peer: {Cohort: 16, Role: "learner"},
			outsider: {Cohort: 17, Role: "learner"}, admin: {Role: "admin"},
		},
		owned:     map[primitive.ObjectID]domain.BaroCharacter{characterID: {ID: characterID, OwnerID: owner}},
		entries:   map[primitive.ObjectID]Entry{owner: {OwnerID: owner.Hex(), Cohort: 16, Message: "Hello", Character: domain.BaroCharacter{ID: characterID, OwnerID: owner}}},
		reactions: map[string]bool{},
	}
	service := NewService(store)
	if _, err := service.React(context.Background(), peer.Hex(), owner.Hex(), "😡"); err == nil {
		t.Fatal("unsupported reaction accepted")
	}
	if _, err := service.React(context.Background(), outsider.Hex(), owner.Hex(), "❤️"); !errors.Is(err, ErrEntryNotFound) {
		t.Fatalf("cross-cohort reaction error = %v", err)
	}
	first, err := service.React(context.Background(), peer.Hex(), owner.Hex(), "❤️")
	if err != nil || !first {
		t.Fatalf("first reaction = %v, %v", first, err)
	}
	items, err := service.List(context.Background(), peer.Hex(), false, 0, "", false)
	if err != nil || items[0].Reactions[0].Count != 1 || !items[0].Reactions[0].Reacted {
		t.Fatalf("reaction summary = %+v, err=%v", items, err)
	}
	second, err := service.React(context.Background(), peer.Hex(), owner.Hex(), "❤️")
	if err != nil || second {
		t.Fatalf("second reaction = %v, %v", second, err)
	}
	items, _ = service.List(context.Background(), peer.Hex(), false, 0, "", false)
	if items[0].Reactions[0].Count != 0 || items[0].Reactions[0].Reacted {
		t.Fatalf("duplicate reaction count = %+v", items[0].Reactions)
	}
	if err := service.Moderate(context.Background(), peer.Hex(), owner.Hex(), true, "not for the lawn"); !errors.Is(err, ErrAdminRequired) {
		t.Fatalf("learner moderation error = %v", err)
	}
	if err := service.Moderate(context.Background(), admin.Hex(), owner.Hex(), true, "not for the lawn"); err != nil {
		t.Fatal(err)
	}
	if len(store.moderation) != 1 || !strings.Contains(store.moderation[0], admin.Hex()) {
		t.Fatalf("moderation audit = %+v", store.moderation)
	}
	if err := service.Save(context.Background(), owner.Hex(), characterID.Hex(), "Edited by owner"); err != nil {
		t.Fatal(err)
	}
	own, err := service.Mine(context.Background(), owner.Hex())
	if err != nil || own == nil || !own.Hidden || own.Message != "Edited by owner" {
		t.Fatalf("owner hidden entry = %+v, err=%v", own, err)
	}
	items, _ = service.List(context.Background(), peer.Hex(), false, 0, "", false)
	if len(items) != 0 {
		t.Fatalf("public saw hidden entry: %+v", items)
	}
	if _, err := service.List(context.Background(), peer.Hex(), false, 0, "", true); !errors.Is(err, ErrAdminRequired) {
		t.Fatalf("hidden list error = %v", err)
	}
	items, err = service.List(context.Background(), admin.Hex(), true, 0, "", true)
	if err != nil || len(items) != 1 || !items[0].Hidden || items[0].Message != "Edited by owner" {
		t.Fatalf("admin hidden view = %+v, err=%v", items, err)
	}
	if _, err := service.React(context.Background(), peer.Hex(), owner.Hex(), "✨"); !errors.Is(err, ErrEntryNotFound) {
		t.Fatalf("hidden reaction error = %v", err)
	}
	if err := service.Moderate(context.Background(), admin.Hex(), owner.Hex(), false, "restored"); err != nil {
		t.Fatal(err)
	}
	items, _ = service.List(context.Background(), peer.Hex(), false, 0, "", false)
	if len(items) != 1 || items[0].Message != "Edited by owner" || len(store.moderation) != 2 {
		t.Fatalf("restored entry = %+v, audit=%+v", items, store.moderation)
	}
}

func TestMoodLastsTwentyFourHoursAndLeavesCharacterStateAlone(t *testing.T) {
	owner := primitive.NewObjectID()
	characterID := primitive.NewObjectID()
	character := domain.BaroCharacter{ID: characterID, OwnerID: owner, Fingerprint: "dna"}
	store := &memoryStore{
		viewers: map[primitive.ObjectID]Viewer{owner: {Cohort: 16, Role: "learner"}},
		owned:   map[primitive.ObjectID]domain.BaroCharacter{characterID: character},
		entries: map[primitive.ObjectID]Entry{}, reactions: map[string]bool{},
	}
	clock := time.Date(2026, 9, 30, 3, 0, 0, 0, time.UTC)
	service := NewService(store)
	service.now = func() time.Time { return clock }
	if _, err := service.SetMood(context.Background(), owner.Hex(), "quiet"); !errors.Is(err, ErrEntryNotFound) {
		t.Fatalf("mood without a pin err = %v", err)
	}
	if err := service.Save(context.Background(), owner.Hex(), characterID.Hex(), "hello"); err != nil {
		t.Fatal(err)
	}
	for _, mood := range []string{"greeting", "relaxing", "meal", "playful", "quiet", "surprise"} {
		state, err := service.SetMood(context.Background(), owner.Hex(), mood)
		if err != nil || state.Mood != mood || !state.Until.Equal(clock.Add(24*time.Hour)) {
			t.Fatalf("SetMood(%q) = %+v, %v", mood, state, err)
		}
	}
	for _, invalid := range []string{"angry", "fight:someone", "quiet "+owner.Hex()} {
		if _, err := service.SetMood(context.Background(), owner.Hex(), invalid); !errors.Is(err, ErrInvalidMood) {
			t.Fatalf("SetMood(%q) err = %v", invalid, err)
		}
	}
	entry, err := service.Mine(context.Background(), owner.Hex())
	if err != nil || entry.Mood != "surprise" || entry.Message != "hello" || entry.Character.Fingerprint != "dna" {
		t.Fatalf("active mood entry = %+v, %v", entry, err)
	}
	clock = clock.Add(24 * time.Hour)
	entry, err = service.Mine(context.Background(), owner.Hex())
	if err != nil || entry.Mood != "" || entry.MoodUntil != nil {
		t.Fatalf("expired mood entry = %+v, %v", entry, err)
	}
	if state, err := service.SetMood(context.Background(), owner.Hex(), ""); err != nil || state.Mood != "" {
		t.Fatalf("clear mood = %+v, %v", state, err)
	}
	if store.entries[owner].Mood != "" || store.entries[owner].Character.ID != characterID {
		t.Fatalf("cleared entry = %+v", store.entries[owner])
	}
}
