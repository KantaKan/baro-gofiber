package character

import (
	"context"
	"errors"
	"strings"
	"time"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

var ErrCharacterNotOwned = errors.New("character is not owned by this account")

type OwnershipStore interface {
	AccountExists(ctx context.Context, ownerID primitive.ObjectID) (bool, error)
	FindStarter(ctx context.Context, ownerID primitive.ObjectID) (*domain.BaroCharacter, error)
	FindOwned(ctx context.Context, ownerID, characterID primitive.ObjectID) (*domain.BaroCharacter, error)
	InsertCharacter(ctx context.Context, character domain.BaroCharacter) error
	ReadSelection(ctx context.Context, ownerID primitive.ObjectID) (domain.CharacterSelection, error)
	SetEquipped(ctx context.Context, ownerID, characterID primitive.ObjectID) error
	SetPinned(ctx context.Context, ownerID primitive.ObjectID, characterID *primitive.ObjectID) error
}

type OwnershipService struct {
	store  OwnershipStore
	picker Picker
}

func NewOwnershipService(store OwnershipStore, picker Picker) *OwnershipService {
	return &OwnershipService{store: store, picker: picker}
}

func (s *OwnershipService) owner(ctx context.Context, ownerHex string) (primitive.ObjectID, error) {
	ownerID, err := primitive.ObjectIDFromHex(ownerHex)
	if err != nil {
		return primitive.NilObjectID, errors.New("invalid account ID")
	}
	exists, err := s.store.AccountExists(ctx, ownerID)
	if err != nil {
		return primitive.NilObjectID, err
	}
	if !exists {
		return primitive.NilObjectID, ErrAccountNotFound
	}
	return ownerID, nil
}

func (s *OwnershipService) Grant(ctx context.Context, ownerHex string) (*domain.BaroCharacter, error) {
	ownerID, err := s.owner(ctx, ownerHex)
	if err != nil {
		return nil, err
	}
	for attempt := 0; attempt < 16; attempt++ {
		dna, err := GenerateDNA(s.picker)
		if err != nil {
			return nil, err
		}
		id := primitive.NewObjectID()
		candidate := domain.BaroCharacter{
			ID: id, OwnerID: ownerID, Serial: "B-" + strings.ToUpper(id.Hex()),
			DNA: dna, Fingerprint: dna.Fingerprint(), Source: "admin_grant", IsStarter: false,
			CreatedAt: time.Now().UTC(),
		}
		if err := s.store.InsertCharacter(ctx, candidate); err == nil {
			return &candidate, nil
		} else if !errors.Is(err, domain.ErrCharacterConflict) {
			return nil, err
		}
	}
	return nil, ErrIssueRetriesExhausted
}

func (s *OwnershipService) Selection(ctx context.Context, ownerHex string) (domain.CharacterSelection, error) {
	ownerID, err := s.owner(ctx, ownerHex)
	if err != nil {
		return domain.CharacterSelection{}, err
	}
	selection, err := s.store.ReadSelection(ctx, ownerID)
	if err != nil {
		return selection, err
	}
	if selection.EquippedID == "" {
		starter, err := s.store.FindStarter(ctx, ownerID)
		if err != nil {
			return selection, err
		}
		if starter != nil {
			selection.EquippedID = starter.ID.Hex()
		}
	}
	return selection, nil
}

func (s *OwnershipService) owned(ctx context.Context, ownerID primitive.ObjectID, characterHex string) (primitive.ObjectID, error) {
	characterID, err := primitive.ObjectIDFromHex(characterHex)
	if err != nil {
		return primitive.NilObjectID, ErrCharacterNotOwned
	}
	item, err := s.store.FindOwned(ctx, ownerID, characterID)
	if err != nil {
		return primitive.NilObjectID, err
	}
	if item == nil {
		return primitive.NilObjectID, ErrCharacterNotOwned
	}
	return characterID, nil
}

func (s *OwnershipService) Equip(ctx context.Context, ownerHex, characterHex string) (domain.CharacterSelection, error) {
	ownerID, err := s.owner(ctx, ownerHex)
	if err != nil {
		return domain.CharacterSelection{}, err
	}
	characterID, err := s.owned(ctx, ownerID, characterHex)
	if err != nil {
		return domain.CharacterSelection{}, err
	}
	if err := s.store.SetEquipped(ctx, ownerID, characterID); err != nil {
		return domain.CharacterSelection{}, err
	}
	return s.Selection(ctx, ownerHex)
}

func (s *OwnershipService) Pin(ctx context.Context, ownerHex, characterHex string) (domain.CharacterSelection, error) {
	ownerID, err := s.owner(ctx, ownerHex)
	if err != nil {
		return domain.CharacterSelection{}, err
	}
	var characterID *primitive.ObjectID
	if characterHex != "" {
		id, err := s.owned(ctx, ownerID, characterHex)
		if err != nil {
			return domain.CharacterSelection{}, err
		}
		characterID = &id
	}
	if err := s.store.SetPinned(ctx, ownerID, characterID); err != nil {
		return domain.CharacterSelection{}, err
	}
	return s.Selection(ctx, ownerHex)
}
