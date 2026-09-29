package reward

import (
	"context"
	"errors"
	"strings"
	"time"

	"gofiber-baro/internal/domain"
)

var ErrIdempotencyKeyRequired = errors.New("idempotency key is required")
var ErrInventoryChanged = errors.New("reward inventory changed")

type LearnerRewardState struct {
	Species  string
	OwnedIDs map[string]bool
}

type DrawResult struct {
	IdempotencyKey string                     `json:"idempotency_key" bson:"idempotency_key"`
	UserID         string                     `json:"user_id" bson:"user_id"`
	Pool           string                     `json:"pool" bson:"pool"`
	MinimumRarity  string                     `json:"minimum_rarity" bson:"minimum_rarity"`
	Item           domain.CosmeticCatalogItem `json:"item" bson:"item"`
	CreatedAt      time.Time                  `json:"created_at" bson:"created_at"`
}

type DrawRepository interface {
	FindDraw(ctx context.Context, idempotencyKey string) (*DrawResult, error)
	LoadLearnerState(ctx context.Context, userID string) (LearnerRewardState, error)
	CommitDraw(ctx context.Context, result DrawResult) (*DrawResult, error)
}

type DrawRequest struct {
	IdempotencyKey string
	UserID         string
	Pool           string
	MinimumRarity  string
}

type Service struct {
	repository DrawRepository
	selector   *Selector
	catalog    []domain.CosmeticCatalogItem
	now        func() time.Time
}

func NewService(repository DrawRepository, selector *Selector, catalog []domain.CosmeticCatalogItem) *Service {
	return &Service{repository: repository, selector: selector, catalog: catalog, now: time.Now}
}

func (s *Service) Odds(ctx context.Context, request DrawRequest) (Eligibility, error) {
	state, err := s.repository.LoadLearnerState(ctx, request.UserID)
	if err != nil {
		return Eligibility{}, err
	}
	return s.selector.Eligibility(SelectionInput{
		Catalog: s.catalog, Pool: request.Pool, MinimumRarity: request.MinimumRarity,
		Species: state.Species, OwnedIDs: state.OwnedIDs,
	}), nil
}

func (s *Service) Open(ctx context.Context, request DrawRequest) (*DrawResult, error) {
	request.IdempotencyKey = strings.TrimSpace(request.IdempotencyKey)
	if request.IdempotencyKey == "" {
		return nil, ErrIdempotencyKeyRequired
	}
	existing, err := s.repository.FindDraw(ctx, request.IdempotencyKey)
	if err != nil || existing != nil {
		return existing, err
	}
	for attempt := 0; attempt < 4; attempt++ {
		state, stateErr := s.repository.LoadLearnerState(ctx, request.UserID)
		if stateErr != nil {
			return nil, stateErr
		}
		item, selectErr := s.selector.Select(SelectionInput{
			Catalog: s.catalog, Pool: request.Pool, MinimumRarity: request.MinimumRarity,
			Species: state.Species, OwnedIDs: state.OwnedIDs,
		})
		if selectErr != nil {
			existing, findErr := s.repository.FindDraw(ctx, request.IdempotencyKey)
			if findErr != nil || existing != nil {
				return existing, findErr
			}
			return nil, selectErr
		}
		result, commitErr := s.repository.CommitDraw(ctx, DrawResult{
			IdempotencyKey: request.IdempotencyKey, UserID: request.UserID, Pool: request.Pool,
			MinimumRarity: request.MinimumRarity, Item: item, CreatedAt: s.now(),
		})
		if !errors.Is(commitErr, ErrInventoryChanged) {
			return result, commitErr
		}
	}
	return nil, ErrInventoryChanged
}
