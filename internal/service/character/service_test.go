package character

import (
	"context"
	"errors"
	"sync"
	"testing"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type fakeStore struct {
	mu        sync.Mutex
	exists    bool
	starter   *domain.BaroCharacter
	conflicts int
}

func (store *fakeStore) AccountExists(context.Context, primitive.ObjectID) (bool, error) {
	return store.exists, nil
}

func (store *fakeStore) FindStarter(context.Context, primitive.ObjectID) (*domain.BaroCharacter, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.starter, nil
}

func (store *fakeStore) ListForOwner(context.Context, primitive.ObjectID) ([]domain.BaroCharacter, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.starter == nil {
		return []domain.BaroCharacter{}, nil
	}
	return []domain.BaroCharacter{*store.starter}, nil
}

func (store *fakeStore) InsertStarter(_ context.Context, candidate domain.BaroCharacter) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.conflicts > 0 {
		store.conflicts--
		return domain.ErrCharacterConflict
	}
	if store.starter != nil {
		return domain.ErrCharacterConflict
	}
	store.starter = &candidate
	return nil
}

type zeroPicker struct{}

func (zeroPicker) Intn(int) (int, error) { return 0, nil }

func TestRevealStarterIsIdempotent(t *testing.T) {
	store := &fakeStore{exists: true}
	service := NewService(store, zeroPicker{})
	owner := primitive.NewObjectID().Hex()
	first, err := service.RevealStarter(context.Background(), owner)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.RevealStarter(context.Background(), owner)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID || first.Fingerprint != second.Fingerprint || !first.IsStarter {
		t.Fatalf("starter changed between requests: %+v %+v", first, second)
	}
	collection, err := service.Collection(context.Background(), owner)
	if err != nil || len(collection) != 1 {
		t.Fatalf("collection = %+v, %v", collection, err)
	}
}

func TestRevealStarterRetriesIdentityCollision(t *testing.T) {
	store := &fakeStore{exists: true, conflicts: 1}
	service := NewService(store, zeroPicker{})
	character, err := service.RevealStarter(context.Background(), primitive.NewObjectID().Hex())
	if err != nil || character == nil {
		t.Fatalf("character = %+v, %v", character, err)
	}
}

func TestRevealStarterConcurrentRequestsReturnSameCharacter(t *testing.T) {
	store := &fakeStore{exists: true}
	service := NewService(store, zeroPicker{})
	owner := primitive.NewObjectID().Hex()
	results := make([]*domain.BaroCharacter, 12)
	errorsFound := make([]error, len(results))
	var group sync.WaitGroup
	for index := range results {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			results[index], errorsFound[index] = service.RevealStarter(context.Background(), owner)
		}(index)
	}
	group.Wait()
	for index, result := range results {
		if errorsFound[index] != nil || result == nil || result.ID != results[0].ID {
			t.Fatalf("request %d = %+v, %v", index, result, errorsFound[index])
		}
	}
}

func TestRevealStarterRejectsMissingAccount(t *testing.T) {
	service := NewService(&fakeStore{}, zeroPicker{})
	_, err := service.RevealStarter(context.Background(), primitive.NewObjectID().Hex())
	if !errors.Is(err, ErrAccountNotFound) {
		t.Fatalf("expected missing account, got %v", err)
	}
}
