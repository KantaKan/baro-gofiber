package character

import (
	"context"
	"sync"
	"testing"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type fakeEggStore struct {
	mu        sync.Mutex
	character *domain.BaroCharacter
}

func (s *fakeEggStore) FindCharacterEgg(_ context.Context, ownerID, eggID primitive.ObjectID) (*domain.BaroCharacter, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.character == nil || s.character.OwnerID != ownerID || s.character.OriginKey != "character-egg:"+eggID.Hex() {
		return nil, nil
	}
	copy := *s.character
	return &copy, nil
}

func (s *fakeEggStore) CommitCharacterEgg(_ context.Context, ownerID, eggID primitive.ObjectID, candidate domain.BaroCharacter) (*domain.BaroCharacter, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.character != nil {
		copy := *s.character
		return &copy, nil
	}
	s.character = &candidate
	copy := candidate
	return &copy, nil
}

func TestEggHatchCreatesOnePermanentCharacterAndRetriesReturnIt(t *testing.T) {
	ownerID := primitive.NewObjectID()
	eggID := primitive.NewObjectID()
	store := &fakeEggStore{}
	picker := &fakePicker{values: []int{9800, 0, 5, 0, 0, 0, 2, 12345}}
	service := NewEggService(store, picker)

	first, err := service.Hatch(context.Background(), ownerID.Hex(), eggID.Hex())
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Hatch(context.Background(), ownerID.Hex(), eggID.Hex())
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID || first.IsStarter || first.DNA.Rarity != domain.CharacterLegendary {
		t.Fatalf("egg hatch results = first:%+v second:%+v", first, second)
	}
	if first.Source != "character_egg" || first.OriginKey != "character-egg:"+eggID.Hex() {
		t.Fatalf("egg origin = %+v", first)
	}
}
