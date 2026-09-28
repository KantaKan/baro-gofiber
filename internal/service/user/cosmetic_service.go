package user

import (
	"context"
	"errors"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type cosmeticUserStore interface {
	FindByID(ctx interface{}, id primitive.ObjectID) (*domain.User, error)
	GrantCosmetic(ctx interface{}, userID primitive.ObjectID, cosmeticID string) (bool, error)
}

type CosmeticService struct {
	users cosmeticUserStore
}

func NewCosmeticService(users cosmeticUserStore) *CosmeticService {
	return &CosmeticService{users: users}
}

func (s *CosmeticService) Grant(userID string, cosmeticID string) (bool, error) {
	id, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return false, errors.New("invalid user ID")
	}
	found := false
	for _, item := range cosmeticCatalog {
		if item.ID == cosmeticID {
			found = true
			break
		}
	}
	if !found {
		return false, errors.New("unknown cosmetic")
	}
	return s.users.GrantCosmetic(context.Background(), id, cosmeticID)
}

var cosmeticCatalog = []domain.CosmeticCatalogItem{
	{ID: "palette:forest", Name: "Forest", Slot: "palette", Rarity: "Common", PreviewValue: "Forest", SourceHint: "Starter collection", RewardPools: []string{"starter"}, Starter: true},
	{ID: "palette:ocean", Name: "Ocean", Slot: "palette", Rarity: "Rare", PreviewValue: "Ocean", SourceHint: "Reflection rewards", RewardPools: []string{"reflection", "teacher-box"}},
	{ID: "palette:midnight", Name: "Midnight", Slot: "palette", Rarity: "Epic", PreviewValue: "Midnight", SourceHint: "Achievement rewards", RewardPools: []string{"achievement", "teacher-box"}},
	{ID: "pot:round", Name: "Cozy Round Pot", Slot: "pot", Rarity: "Common", PreviewValue: "round", SourceHint: "Starter collection", RewardPools: []string{"starter"}, Starter: true},
	{ID: "pot:starlight", Name: "Starlight Pot", Slot: "pot", Rarity: "Rare", PreviewValue: "starlight", SourceHint: "Teacher Gift Boxes", RewardPools: []string{"teacher-box"}},
	{ID: "pot:crystal", Name: "Crystal Pot", Slot: "pot", Rarity: "Legendary", PreviewValue: "crystal", SourceHint: "Special achievements", RewardPools: []string{"achievement", "teacher-box"}},
	{ID: "aura:morning-mist", Name: "Morning Mist", Slot: "aura", Rarity: "Common", PreviewValue: "morning-mist", SourceHint: "Starter collection", RewardPools: []string{"starter"}, Starter: true},
	{ID: "aura:firefly", Name: "Firefly Glow", Slot: "aura", Rarity: "Rare", PreviewValue: "firefly", SourceHint: "Consistency rewards", RewardPools: []string{"reflection", "teacher-box"}},
	{ID: "aura:aurora", Name: "Aurora Halo", Slot: "aura", Rarity: "Legendary", PreviewValue: "aurora", SourceHint: "Secret achievements", RewardPools: []string{"achievement"}},
	{ID: "particle:pollen", Name: "Gentle Pollen", Slot: "particle", Rarity: "Common", PreviewValue: "pollen", SourceHint: "Starter collection", RewardPools: []string{"starter"}, Starter: true},
	{ID: "particle:petals", Name: "Falling Petals", Slot: "particle", Rarity: "Rare", PreviewValue: "petals", SourceHint: "Reflection rewards", RewardPools: []string{"reflection", "teacher-box"}},
	{ID: "particle:stars", Name: "Tiny Stars", Slot: "particle", Rarity: "Epic", PreviewValue: "stars", SourceHint: "Social achievements", RewardPools: []string{"achievement", "teacher-box"}},
	{ID: "accessory:ladybug", Name: "Ladybug Friend", Slot: "accessory", Rarity: "Common", PreviewValue: "ladybug", SourceHint: "Starter collection", RewardPools: []string{"starter"}, Starter: true},
	{ID: "accessory:ribbon", Name: "Garden Ribbon", Slot: "accessory", Rarity: "Rare", PreviewValue: "ribbon", SourceHint: "Teacher Gift Boxes", RewardPools: []string{"teacher-box"}},
	{ID: "accessory:crown", Name: "Little Crown", Slot: "accessory", Rarity: "Legendary", PreviewValue: "crown", SourceHint: "Special achievements", RewardPools: []string{"achievement"}},
	{ID: "mutation:speckled", Name: "Speckled Leaves", Slot: "mutation", Rarity: "Common", PreviewValue: "speckled", SourceHint: "Starter collection", RewardPools: []string{"starter"}, Starter: true},
	{ID: "mutation:variegated", Name: "Variegated Leaves", Slot: "mutation", Rarity: "Epic", PreviewValue: "variegated", SourceHint: "Comeback achievements", RewardPools: []string{"achievement", "teacher-box"}},
	{ID: "mutation:crystal", Name: "Crystal Growth", Slot: "mutation", Rarity: "Legendary", PreviewValue: "crystal", SourceHint: "Secret achievements", RewardPools: []string{"achievement"}},
}

func (s *CosmeticService) Catalog() []domain.CosmeticCatalogItem {
	return append([]domain.CosmeticCatalogItem(nil), cosmeticCatalog...)
}

func (s *CosmeticService) Collection(userID string) (*domain.CosmeticCollection, error) {
	id, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, errors.New("invalid user ID")
	}
	learner, err := s.users.FindByID(context.Background(), id)
	if err != nil {
		return nil, err
	}
	owned := make(map[string]bool, len(learner.OwnedCosmeticIDs))
	for _, id := range learner.OwnedCosmeticIDs {
		owned[id] = true
	}
	newItems := make(map[string]bool, len(learner.NewCosmeticIDs))
	for _, id := range learner.NewCosmeticIDs {
		newItems[id] = true
	}
	items := make([]domain.CosmeticCollectionItem, 0, len(cosmeticCatalog))
	for _, catalogItem := range cosmeticCatalog {
		isOwned := catalogItem.Starter || owned[catalogItem.ID]
		items = append(items, domain.CosmeticCollectionItem{
			CosmeticCatalogItem: catalogItem,
			Owned:               isOwned,
			New:                 isOwned && newItems[catalogItem.ID],
			Equipped:            learner.EquippedCosmetics[catalogItem.Slot] == catalogItem.ID,
			Locked:              !isOwned,
		})
	}
	return &domain.CosmeticCollection{Items: items}, nil
}
