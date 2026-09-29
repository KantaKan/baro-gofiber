package character

import (
	"context"
	"errors"
	"sync"
	"testing"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ownershipStore struct {
	mu         sync.Mutex
	characters map[primitive.ObjectID][]domain.BaroCharacter
	selections map[primitive.ObjectID]domain.CharacterSelection
}

func (s *ownershipStore) AccountExists(context.Context, primitive.ObjectID) (bool, error) {
	return true, nil
}
func (s *ownershipStore) FindStarter(_ context.Context, owner primitive.ObjectID) (*domain.BaroCharacter, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, item := range s.characters[owner] {
		if item.IsStarter {
			found := item
			return &found, nil
		}
	}
	return nil, nil
}
func (s *ownershipStore) ListForOwner(_ context.Context, owner primitive.ObjectID) ([]domain.BaroCharacter, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]domain.BaroCharacter{}, s.characters[owner]...), nil
}
func (s *ownershipStore) FindOwned(_ context.Context, owner, characterID primitive.ObjectID) (*domain.BaroCharacter, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, item := range s.characters[owner] {
		if item.ID == characterID {
			found := item
			return &found, nil
		}
	}
	return nil, nil
}
func (s *ownershipStore) InsertCharacter(_ context.Context, item domain.BaroCharacter) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, all := range s.characters {
		for _, existing := range all {
			if existing.Fingerprint == item.Fingerprint {
				return domain.ErrCharacterConflict
			}
		}
	}
	s.characters[item.OwnerID] = append(s.characters[item.OwnerID], item)
	return nil
}
func (s *ownershipStore) ReadSelection(_ context.Context, owner primitive.ObjectID) (domain.CharacterSelection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.selections[owner], nil
}
func (s *ownershipStore) SetEquipped(_ context.Context, owner, id primitive.ObjectID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	selection := s.selections[owner]
	selection.EquippedID = id.Hex()
	s.selections[owner] = selection
	return nil
}
func (s *ownershipStore) SetPinned(_ context.Context, owner primitive.ObjectID, id *primitive.ObjectID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	selection := s.selections[owner]
	selection.PinnedID = ""
	if id != nil {
		selection.PinnedID = id.Hex()
	}
	s.selections[owner] = selection
	return nil
}

type sequencePicker struct {
	mu   sync.Mutex
	next int
}

func (p *sequencePicker) Intn(limit int) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.next++
	return p.next % limit, nil
}

func TestGrantKeepsAllCharactersAndUniqueDNA(t *testing.T) {
	owner := primitive.NewObjectID()
	starter := domain.BaroCharacter{ID: primitive.NewObjectID(), OwnerID: owner, IsStarter: true, Fingerprint: "starter"}
	store := &ownershipStore{characters: map[primitive.ObjectID][]domain.BaroCharacter{owner: {starter}}, selections: map[primitive.ObjectID]domain.CharacterSelection{}}
	service := NewOwnershipService(store, &sequencePicker{})
	first, err := service.Grant(context.Background(), owner.Hex())
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Grant(context.Background(), owner.Hex())
	if err != nil {
		t.Fatal(err)
	}
	collection, err := store.ListForOwner(context.Background(), owner)
	if err != nil || len(collection) != 3 || !collection[0].IsStarter || first.Fingerprint == second.Fingerprint || first.IsStarter || second.IsStarter {
		t.Fatalf("collection = %+v, %v", collection, err)
	}
}

func TestEquipAndPinAreIndependentAndOwned(t *testing.T) {
	owner := primitive.NewObjectID()
	other := primitive.NewObjectID()
	first := domain.BaroCharacter{ID: primitive.NewObjectID(), OwnerID: owner, IsStarter: true}
	second := domain.BaroCharacter{ID: primitive.NewObjectID(), OwnerID: owner}
	foreign := domain.BaroCharacter{ID: primitive.NewObjectID(), OwnerID: other}
	store := &ownershipStore{characters: map[primitive.ObjectID][]domain.BaroCharacter{owner: {first, second}, other: {foreign}}, selections: map[primitive.ObjectID]domain.CharacterSelection{}}
	service := NewOwnershipService(store, &sequencePicker{})
	initial, err := service.Selection(context.Background(), owner.Hex())
	if err != nil || initial.EquippedID != first.ID.Hex() || initial.PinnedID != "" {
		t.Fatalf("initial = %+v, %v", initial, err)
	}
	if _, err := service.Pin(context.Background(), owner.Hex(), second.ID.Hex()); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Equip(context.Background(), owner.Hex(), first.ID.Hex()); err != nil {
		t.Fatal(err)
	}
	chosen, err := service.Selection(context.Background(), owner.Hex())
	if err != nil || chosen.EquippedID != first.ID.Hex() || chosen.PinnedID != second.ID.Hex() {
		t.Fatalf("selection = %+v, %v", chosen, err)
	}
	if _, err := service.Equip(context.Background(), owner.Hex(), foreign.ID.Hex()); !errors.Is(err, ErrCharacterNotOwned) {
		t.Fatalf("foreign equip error = %v", err)
	}
	if _, err := service.Pin(context.Background(), owner.Hex(), foreign.ID.Hex()); !errors.Is(err, ErrCharacterNotOwned) {
		t.Fatalf("foreign pin error = %v", err)
	}
	if _, err := service.Pin(context.Background(), owner.Hex(), ""); err != nil {
		t.Fatal(err)
	}
	unpinned, _ := service.Selection(context.Background(), owner.Hex())
	if unpinned.PinnedID != "" || unpinned.EquippedID != first.ID.Hex() {
		t.Fatalf("unpinned = %+v", unpinned)
	}
}
