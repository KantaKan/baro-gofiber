package character

import (
	"context"
	"errors"
	"strings"
	"time"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type EggStore interface {
	FindCharacterEgg(ctx context.Context, ownerID, eggID primitive.ObjectID) (*domain.BaroCharacter, error)
	CommitCharacterEgg(ctx context.Context, ownerID, eggID primitive.ObjectID, candidate domain.BaroCharacter) (*domain.BaroCharacter, error)
}

type EggService struct {
	store  EggStore
	picker Picker
}

func NewEggService(store EggStore, picker Picker) *EggService {
	return &EggService{store: store, picker: picker}
}

func (s *EggService) Hatch(ctx context.Context, ownerHex, eggHex string) (*domain.BaroCharacter, error) {
	ownerID, err := primitive.ObjectIDFromHex(ownerHex)
	if err != nil {
		return nil, errors.New("invalid account ID")
	}
	eggID, err := primitive.ObjectIDFromHex(eggHex)
	if err != nil {
		return nil, errors.New("invalid character egg ID")
	}
	existing, err := s.store.FindCharacterEgg(ctx, ownerID, eggID)
	if err != nil || existing != nil {
		return existing, err
	}
	for attempt := 0; attempt < 16; attempt++ {
		dna, err := GenerateDNA(s.picker)
		if err != nil {
			return nil, err
		}
		id := primitive.NewObjectID()
		candidate := domain.BaroCharacter{
			ID: id, OwnerID: ownerID, Serial: "B-" + strings.ToUpper(id.Hex()),
			DNA: dna, Fingerprint: dna.Fingerprint(), Source: "character_egg",
			OriginKey: "character-egg:" + eggID.Hex(), IsStarter: false, CreatedAt: time.Now().UTC(),
		}
		character, err := s.store.CommitCharacterEgg(ctx, ownerID, eggID, candidate)
		if err == nil {
			return character, nil
		}
		if !errors.Is(err, domain.ErrCharacterConflict) {
			return nil, err
		}
	}
	return nil, ErrIssueRetriesExhausted
}
