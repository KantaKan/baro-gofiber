package reward

import (
	"context"
	"sync"
	"testing"

	"gofiber-baro/internal/domain"
)

type constantRandom float64

func (r constantRandom) Float64() float64 { return float64(r) }

type fakeDrawRepository struct {
	mu          sync.Mutex
	draws       map[string]DrawResult
	owned       map[string]bool
	commitCount int
}

func (r *fakeDrawRepository) FindDraw(_ context.Context, key string) (*DrawResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	result, exists := r.draws[key]
	if !exists {
		return nil, nil
	}
	copy := result
	return &copy, nil
}

func (r *fakeDrawRepository) LoadLearnerState(_ context.Context, _ string) (LearnerRewardState, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	owned := make(map[string]bool, len(r.owned))
	for id, value := range r.owned {
		owned[id] = value
	}
	return LearnerRewardState{Species: "Lotus", OwnedIDs: owned}, nil
}

func (r *fakeDrawRepository) CommitDraw(_ context.Context, result DrawResult) (*DrawResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, exists := r.draws[result.IdempotencyKey]; exists {
		copy := existing
		return &copy, nil
	}
	r.draws[result.IdempotencyKey] = result
	r.owned[result.Item.ID] = true
	r.commitCount++
	copy := result
	return &copy, nil
}

func newRewardService(repository DrawRepository) *Service {
	catalog := []domain.CosmeticCatalogItem{{ID: "rare", Name: "Rare", Rarity: "Rare", RewardPools: []string{"teacher-box"}}}
	return NewService(repository, NewSelector(constantRandom(0)), catalog)
}

func TestServiceReturnsTheStoredResultForRepeatedIdempotencyKey(t *testing.T) {
	repository := &fakeDrawRepository{draws: map[string]DrawResult{}, owned: map[string]bool{}}
	service := newRewardService(repository)
	request := DrawRequest{IdempotencyKey: "box-1", UserID: "learner-1", Pool: "teacher-box", MinimumRarity: "Rare"}

	first, err := service.Open(context.Background(), request)
	if err != nil {
		t.Fatalf("first Open returned an error: %v", err)
	}
	second, err := service.Open(context.Background(), request)
	if err != nil {
		t.Fatalf("second Open returned an error: %v", err)
	}
	if first.Item.ID != second.Item.ID || repository.commitCount != 1 {
		t.Fatalf("repeated open was not idempotent: first=%+v second=%+v commits=%d", first, second, repository.commitCount)
	}
}

func TestConcurrentOpenCommitsOneReward(t *testing.T) {
	repository := &fakeDrawRepository{draws: map[string]DrawResult{}, owned: map[string]bool{}}
	service := newRewardService(repository)
	request := DrawRequest{IdempotencyKey: "box-concurrent", UserID: "learner-1", Pool: "teacher-box", MinimumRarity: "Rare"}

	const attempts = 16
	results := make(chan *DrawResult, attempts)
	errors := make(chan error, attempts)
	var group sync.WaitGroup
	for range attempts {
		group.Add(1)
		go func() {
			defer group.Done()
			result, err := service.Open(context.Background(), request)
			results <- result
			errors <- err
		}()
	}
	group.Wait()
	close(results)
	close(errors)

	for err := range errors {
		if err != nil {
			t.Fatalf("concurrent Open returned an error: %v", err)
		}
	}
	for result := range results {
		if result == nil || result.Item.ID != "rare" {
			t.Fatalf("concurrent Open returned the wrong result: %+v", result)
		}
	}
	if repository.commitCount != 1 || len(repository.owned) != 1 {
		t.Fatalf("concurrent Open committed %d times with inventory %v", repository.commitCount, repository.owned)
	}
}

func TestServiceRequiresAnIdempotencyKey(t *testing.T) {
	repository := &fakeDrawRepository{draws: map[string]DrawResult{}, owned: map[string]bool{}}
	_, err := newRewardService(repository).Open(context.Background(), DrawRequest{})
	if err != ErrIdempotencyKeyRequired {
		t.Fatalf("Open error = %v, want %v", err, ErrIdempotencyKeyRequired)
	}
}
