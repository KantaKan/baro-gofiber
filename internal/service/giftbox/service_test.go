package giftbox

import (
	"context"
	"sync"
	"testing"

	"gofiber-baro/internal/domain"
	"gofiber-baro/internal/service/reward"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type fakeStore struct {
	mu    sync.Mutex
	boxes map[primitive.ObjectID]domain.TeacherGiftBox
}

func (s *fakeStore) Create(_ context.Context, box domain.TeacherGiftBox) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.boxes[box.ID] = box
	return nil
}

func (s *fakeStore) ListForUser(_ context.Context, userID primitive.ObjectID) ([]domain.TeacherGiftBox, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	boxes := []domain.TeacherGiftBox{}
	for _, box := range s.boxes {
		if box.UserID == userID {
			boxes = append(boxes, box)
		}
	}
	return boxes, nil
}

type fakeDrawer struct {
	mu      sync.Mutex
	results map[string]*reward.DrawResult
	calls   int
}

func (d *fakeDrawer) Open(_ context.Context, request reward.DrawRequest) (*reward.DrawResult, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if result := d.results[request.IdempotencyKey]; result != nil {
		return result, nil
	}
	d.calls++
	result := &reward.DrawResult{IdempotencyKey: request.IdempotencyKey, UserID: request.UserID, Item: domain.CosmeticCatalogItem{ID: "pot:starlight", Name: "Starlight Pot"}}
	d.results[request.IdempotencyKey] = result
	return result, nil
}

func TestGiftBoxesAreVisibleOnlyToTheirLearner(t *testing.T) {
	store := &fakeStore{boxes: map[primitive.ObjectID]domain.TeacherGiftBox{}}
	service := NewService(store, &fakeDrawer{results: map[string]*reward.DrawResult{}})
	learnerA := primitive.NewObjectID()
	learnerB := primitive.NewObjectID()
	admin := primitive.NewObjectID()

	if _, err := service.Grant(context.Background(), learnerA.Hex(), admin.Hex(), "Rare", "For your steady practice"); err != nil {
		t.Fatalf("Grant returned an error: %v", err)
	}
	if _, err := service.Grant(context.Background(), learnerB.Hex(), admin.Hex(), "Epic", "For helping your genmates"); err != nil {
		t.Fatalf("Grant returned an error: %v", err)
	}
	boxes, err := service.List(context.Background(), learnerA.Hex())
	if err != nil || len(boxes) != 1 || boxes[0].Message != "For your steady practice" {
		t.Fatalf("learner saw the wrong boxes: boxes=%+v err=%v", boxes, err)
	}
}

func TestOpeningTheSameBoxConcurrentlyReturnsOneReward(t *testing.T) {
	learner := primitive.NewObjectID()
	boxID := primitive.NewObjectID()
	store := &fakeStore{boxes: map[primitive.ObjectID]domain.TeacherGiftBox{
		boxID: {ID: boxID, UserID: learner, MinimumRarity: "Rare", Status: "unopened"},
	}}
	drawer := &fakeDrawer{results: map[string]*reward.DrawResult{}}
	service := NewService(store, drawer)

	const attempts = 12
	results := make(chan *reward.DrawResult, attempts)
	var group sync.WaitGroup
	for range attempts {
		group.Add(1)
		go func() {
			defer group.Done()
			result, err := service.Open(context.Background(), learner.Hex(), boxID.Hex())
			if err != nil {
				t.Errorf("Open returned an error: %v", err)
			}
			results <- result
		}()
	}
	group.Wait()
	close(results)
	for result := range results {
		if result == nil || result.Item.ID != "pot:starlight" {
			t.Fatalf("Open returned the wrong reward: %+v", result)
		}
	}
	if drawer.calls != 1 {
		t.Fatalf("box drew %d rewards, want 1", drawer.calls)
	}
}
