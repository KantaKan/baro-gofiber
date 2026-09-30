package giftbox

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"gofiber-baro/internal/domain"
	"gofiber-baro/internal/service/reward"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type fakeStore struct {
	mu         sync.Mutex
	boxes      map[primitive.ObjectID]domain.TeacherGiftBox
	cohorts    map[int][]primitive.ObjectID
	teams      map[string][]primitive.ObjectID
	grantKeys  map[string]bool
	failUser   primitive.ObjectID
	recipients map[primitive.ObjectID]Recipient
}

func (s *fakeStore) SearchRecipients(_ context.Context, query string, exclude primitive.ObjectID) ([]Recipient, error) {
	items := []Recipient{}
	for id, recipient := range s.recipients {
		if id != exclude && strings.Contains(strings.ToLower(recipient.DisplayName), strings.ToLower(query)) {
			items = append(items, recipient)
		}
	}
	return items, nil
}

func (s *fakeStore) IsLearner(_ context.Context, userID primitive.ObjectID) (bool, error) {
	recipient, exists := s.recipients[userID]
	return exists && recipient.Role == "learner", nil
}

func (s *fakeStore) Transfer(_ context.Context, boxID, fromID, toID primitive.ObjectID) (*domain.TeacherGiftBox, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	box, exists := s.boxes[boxID]
	if !exists || box.UserID != fromID || box.Status != "unopened" {
		return nil, ErrBoxNotFound
	}
	if _, exists := s.recipients[toID]; !exists {
		return nil, ErrRecipientNotFound
	}
	box.UserID = toID
	box.TransferHistory = append(box.TransferHistory, domain.GiftBoxTransfer{FromID: fromID, ToID: toID, TransferredAt: time.Now()})
	s.boxes[boxID] = box
	return &box, nil
}

func (s *fakeStore) ListCohortLearners(_ context.Context, cohort int) ([]primitive.ObjectID, error) {
	return s.cohorts[cohort], nil
}

func (s *fakeStore) ListTeamLearners(_ context.Context, cohort int, team string) ([]primitive.ObjectID, error) {
	return s.teams[strconv.Itoa(cohort)+":"+team], nil
}

func (s *fakeStore) CreateOnce(ctx context.Context, box domain.TeacherGiftBox) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if box.UserID == s.failUser {
		s.failUser = primitive.NilObjectID
		return false, errors.New("temporary write failure")
	}
	if s.grantKeys[box.GrantKey] {
		return false, nil
	}
	s.grantKeys[box.GrantKey] = true
	s.boxes[box.ID] = box
	return true, nil
}

func (s *fakeStore) Create(_ context.Context, box domain.TeacherGiftBox) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.boxes[box.ID] = box
	return nil
}

func TestCohortGrantReportsPartialFailuresAndRetriesWithoutDuplicates(t *testing.T) {
	learners := []primitive.ObjectID{primitive.NewObjectID(), primitive.NewObjectID(), primitive.NewObjectID()}
	store := &fakeStore{
		boxes:     map[primitive.ObjectID]domain.TeacherGiftBox{},
		cohorts:   map[int][]primitive.ObjectID{16: learners},
		grantKeys: map[string]bool{},
		failUser:  learners[1],
	}
	service := NewService(store, &fakeDrawer{results: map[string]*reward.DrawResult{}})
	admin := primitive.NewObjectID()

	first, err := service.GrantCohort(context.Background(), 16, admin.Hex(), "Epic", "For a strong sprint", "sprint-16")
	if err != nil || first.Created != 2 || len(first.Failures) != 1 {
		t.Fatalf("first cohort grant = %+v, err=%v", first, err)
	}
	second, err := service.GrantCohort(context.Background(), 16, admin.Hex(), "Epic", "For a strong sprint", "sprint-16")
	if err != nil || second.Created != 1 || second.Existing != 2 || len(second.Failures) != 0 {
		t.Fatalf("retry cohort grant = %+v, err=%v", second, err)
	}
	if len(store.boxes) != 3 {
		t.Fatalf("retry created duplicates: %d boxes", len(store.boxes))
	}
}

func TestTeamGrantPreviewsScopeAndRetriesCharacterBoxes(t *testing.T) {
	first := primitive.NewObjectID()
	second := primitive.NewObjectID()
	outside := primitive.NewObjectID()
	store := &fakeStore{
		boxes:     map[primitive.ObjectID]domain.TeacherGiftBox{},
		cohorts:   map[int][]primitive.ObjectID{16: {first, second, outside}},
		teams:     map[string][]primitive.ObjectID{"16:Garden Alpha": {first, second}},
		grantKeys: map[string]bool{}, failUser: second,
	}
	service := NewService(store, &fakeDrawer{results: map[string]*reward.DrawResult{}})
	preview, err := service.PreviewAudience(context.Background(), 16, " Garden Alpha ")
	if err != nil || preview.Total != 2 || preview.Team != "Garden Alpha" {
		t.Fatalf("team preview = %+v, err=%v", preview, err)
	}
	all, err := service.PreviewAudience(context.Background(), 16, "")
	if err != nil || all.Total != 3 {
		t.Fatalf("cohort preview = %+v, err=%v", all, err)
	}
	admin := primitive.NewObjectID().Hex()
	firstGrant, err := service.GrantAudience(context.Background(), 16, "Garden Alpha", admin, "Rare", "Team surprise", "team-batch", "character-box")
	if err != nil || firstGrant.Total != 2 || firstGrant.Created != 1 || len(firstGrant.Failures) != 1 {
		t.Fatalf("first team grant = %+v, err=%v", firstGrant, err)
	}
	retry, err := service.GrantAudience(context.Background(), 16, "Garden Alpha", admin, "Rare", "Team surprise", "team-batch", "character-box")
	if err != nil || retry.Created != 1 || retry.Existing != 1 || len(retry.Failures) != 0 {
		t.Fatalf("retry team grant = %+v, err=%v", retry, err)
	}
	if len(store.boxes) != 2 {
		t.Fatalf("wrong recipient scope or duplicate boxes: %+v", store.boxes)
	}
	for _, box := range store.boxes {
		if box.UserID == outside || box.RewardPool != "character-box" {
			t.Fatalf("wrong grant = %+v", box)
		}
	}
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
	mu       sync.Mutex
	results  map[string]*reward.DrawResult
	calls    int
	requests []reward.DrawRequest
}

func (d *fakeDrawer) Open(_ context.Context, request reward.DrawRequest) (*reward.DrawResult, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if result := d.results[request.IdempotencyKey]; result != nil {
		return result, nil
	}
	d.calls++
	d.requests = append(d.requests, request)
	result := &reward.DrawResult{IdempotencyKey: request.IdempotencyKey, UserID: request.UserID, Item: domain.CosmeticCatalogItem{ID: "pot:starlight", Name: "Starlight Pot"}}
	d.results[request.IdempotencyKey] = result
	return result, nil
}

func (d *fakeDrawer) Odds(_ context.Context, request reward.DrawRequest) (reward.Eligibility, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.requests = append(d.requests, request)
	return reward.Eligibility{EligibleCount: 1, Odds: map[string]float64{"Rare": 1}}, nil
}

func TestCharacterBoxUsesSeparatePoolAndKeepsLegacyBox(t *testing.T) {
	learner := primitive.NewObjectID()
	admin := primitive.NewObjectID()
	store := &fakeStore{boxes: map[primitive.ObjectID]domain.TeacherGiftBox{}}
	drawer := &fakeDrawer{results: map[string]*reward.DrawResult{}}
	service := NewService(store, drawer)
	characterBox, err := service.GrantWithPool(context.Background(), learner.Hex(), admin.Hex(), "Rare", "A little surprise", "character-box")
	if err != nil || characterBox.RewardPool != "character-box" {
		t.Fatalf("character box = %+v, err=%v", characterBox, err)
	}
	legacyBox, err := service.Grant(context.Background(), learner.Hex(), admin.Hex(), "Rare", "A garden gift")
	if err != nil || legacyBox.RewardPool != "" {
		t.Fatalf("legacy box = %+v, err=%v", legacyBox, err)
	}
	if _, err := service.Odds(context.Background(), learner.Hex(), characterBox.ID.Hex()); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Open(context.Background(), learner.Hex(), legacyBox.ID.Hex()); err != nil {
		t.Fatal(err)
	}
	if len(drawer.requests) != 2 || drawer.requests[0].Pool != "character-box" || drawer.requests[1].Pool != "teacher-box" {
		t.Fatalf("draw pools = %+v", drawer.requests)
	}
}

func TestAchievementBoxesDrawFromAchievementPool(t *testing.T) {
	learner := primitive.NewObjectID()
	boxID := primitive.NewObjectID()
	store := &fakeStore{boxes: map[primitive.ObjectID]domain.TeacherGiftBox{
		boxID: {ID: boxID, UserID: learner, MinimumRarity: "Epic", Status: "unopened", Source: "achievement"},
	}}
	drawer := &fakeDrawer{results: map[string]*reward.DrawResult{}}
	service := NewService(store, drawer)

	if _, err := service.Open(context.Background(), learner.Hex(), boxID.Hex()); err != nil {
		t.Fatal(err)
	}
	if len(drawer.requests) != 1 || drawer.requests[0].Pool != "achievement" {
		t.Fatalf("draw request = %+v", drawer.requests)
	}
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

func TestGiftBoxTravelsAcrossAccountsAndPreservesOriginalGrant(t *testing.T) {
	first := primitive.NewObjectID()
	second := primitive.NewObjectID()
	admin := primitive.NewObjectID()
	boxID := primitive.NewObjectID()
	store := &fakeStore{
		boxes: map[primitive.ObjectID]domain.TeacherGiftBox{boxID: {
			ID: boxID, UserID: first, GrantedBy: admin, Status: "unopened", RewardPool: "character-box", Message: "Keep exploring",
		}},
		recipients: map[primitive.ObjectID]Recipient{
			first:  {ID: first.Hex(), DisplayName: "Mali", CohortNumber: 16, Role: "learner"},
			second: {ID: second.Hex(), DisplayName: "Pim", CohortNumber: 17, Role: "learner"},
			admin:  {ID: admin.Hex(), DisplayName: "Teacher", Role: "admin"},
		},
	}
	service := NewService(store, racingDrawer{store: store})
	recipients, err := service.SearchRecipients(context.Background(), first.Hex(), "Teacher")
	if err != nil || len(recipients) != 0 {
		t.Fatalf("admin recipient search = %+v, err=%v", recipients, err)
	}
	moved, err := service.Transfer(context.Background(), first.Hex(), boxID.Hex(), second.Hex())
	if err != nil || moved.UserID != second || moved.GrantedBy != admin || len(moved.TransferHistory) != 1 {
		t.Fatalf("first transfer = %+v, err=%v", moved, err)
	}
	if _, err := service.Open(context.Background(), first.Hex(), boxID.Hex()); !errors.Is(err, ErrBoxNotFound) {
		t.Fatalf("former owner opened box: %v", err)
	}
	if _, err := service.Transfer(context.Background(), second.Hex(), boxID.Hex(), admin.Hex()); !errors.Is(err, ErrRecipientNotFound) {
		t.Fatalf("transfer to admin error = %v", err)
	}
	if _, err := service.Open(context.Background(), second.Hex(), boxID.Hex()); err != nil {
		t.Fatalf("learner recipient could not open box: %v", err)
	}
	if _, err := service.Transfer(context.Background(), second.Hex(), boxID.Hex(), first.Hex()); !errors.Is(err, ErrBoxNotFound) {
		t.Fatalf("opened box transferred: %v", err)
	}
}

type racingDrawer struct {
	store *fakeStore
}

func (d racingDrawer) Odds(context.Context, reward.DrawRequest) (reward.Eligibility, error) {
	return reward.Eligibility{EligibleCount: 1, Odds: map[string]float64{"Rare": 1}}, nil
}
func (d racingDrawer) Open(_ context.Context, request reward.DrawRequest) (*reward.DrawResult, error) {
	boxID, _ := primitive.ObjectIDFromHex(request.IdempotencyKey)
	ownerID, _ := primitive.ObjectIDFromHex(request.UserID)
	d.store.mu.Lock()
	defer d.store.mu.Unlock()
	box := d.store.boxes[boxID]
	if box.UserID != ownerID || box.Status != "unopened" {
		return nil, ErrBoxNotFound
	}
	box.Status = "opened"
	d.store.boxes[boxID] = box
	return &reward.DrawResult{IdempotencyKey: request.IdempotencyKey, UserID: request.UserID, Item: domain.CosmeticCatalogItem{ID: "character_prop:halo"}}, nil
}

func TestTransferAndOpenCannotBothSucceed(t *testing.T) {
	for attempt := 0; attempt < 80; attempt++ {
		from := primitive.NewObjectID()
		to := primitive.NewObjectID()
		boxID := primitive.NewObjectID()
		store := &fakeStore{boxes: map[primitive.ObjectID]domain.TeacherGiftBox{boxID: {ID: boxID, UserID: from, Status: "unopened"}}, recipients: map[primitive.ObjectID]Recipient{to: {ID: to.Hex(), Role: "learner"}}}
		service := NewService(store, racingDrawer{store: store})
		start := make(chan struct{})
		results := make(chan error, 2)
		go func() { <-start; _, err := service.Open(context.Background(), from.Hex(), boxID.Hex()); results <- err }()
		go func() {
			<-start
			_, err := service.Transfer(context.Background(), from.Hex(), boxID.Hex(), to.Hex())
			results <- err
		}()
		close(start)
		firstErr := <-results
		secondErr := <-results
		if (firstErr == nil) == (secondErr == nil) {
			t.Fatalf("attempt %d: open/transfer results = %v, %v", attempt, firstErr, secondErr)
		}
		box := store.boxes[boxID]
		if (box.Status == "opened") == (box.UserID == to) {
			t.Fatalf("attempt %d: conflicting box state = %+v", attempt, box)
		}
	}
}
