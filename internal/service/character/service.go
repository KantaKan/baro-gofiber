package character

import (
	"context"
	"errors"
	"strings"
	"time"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

var ErrAccountNotFound = errors.New("account not found")
var ErrIssueRetriesExhausted = errors.New("could not create a unique character")

type Store interface {
	AccountExists(ctx context.Context, ownerID primitive.ObjectID) (bool, error)
	FindStarter(ctx context.Context, ownerID primitive.ObjectID) (*domain.BaroCharacter, error)
	ListForOwner(ctx context.Context, ownerID primitive.ObjectID) ([]domain.BaroCharacter, error)
	InsertStarter(ctx context.Context, character domain.BaroCharacter) error
}

type Service struct {
	store  Store
	picker Picker
}

func NewService(store Store, picker Picker) *Service {
	return &Service{store: store, picker: picker}
}

func (s *Service) Collection(ctx context.Context, ownerHex string) ([]domain.BaroCharacter, error) {
	ownerID, err := primitive.ObjectIDFromHex(ownerHex)
	if err != nil {
		return nil, errors.New("invalid account ID")
	}
	exists, err := s.store.AccountExists(ctx, ownerID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrAccountNotFound
	}
	return s.store.ListForOwner(ctx, ownerID)
}

func (s *Service) RevealStarter(ctx context.Context, ownerHex string) (*domain.BaroCharacter, error) {
	ownerID, err := primitive.ObjectIDFromHex(ownerHex)
	if err != nil {
		return nil, errors.New("invalid account ID")
	}
	exists, err := s.store.AccountExists(ctx, ownerID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrAccountNotFound
	}
	starter, err := s.store.FindStarter(ctx, ownerID)
	if err != nil || starter != nil {
		return starter, err
	}
	for attempt := 0; attempt < 16; attempt++ {
		dna, err := GenerateDNA(s.picker)
		if err != nil {
			return nil, err
		}
		id := primitive.NewObjectID()
		candidate := domain.BaroCharacter{
			ID: id, OwnerID: ownerID, Serial: "B-" + strings.ToUpper(id.Hex()),
			DNA: dna, Fingerprint: dna.Fingerprint(), Source: "starter", IsStarter: true,
			CreatedAt: time.Now().UTC(),
		}
		if err := s.store.InsertStarter(ctx, candidate); err == nil {
			return &candidate, nil
		} else if !errors.Is(err, domain.ErrCharacterConflict) {
			return nil, err
		}
		starter, err = s.store.FindStarter(ctx, ownerID)
		if err != nil || starter != nil {
			return starter, err
		}
	}
	return nil, ErrIssueRetriesExhausted
}
