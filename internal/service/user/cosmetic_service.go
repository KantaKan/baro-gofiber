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
	RevokeCosmetic(ctx interface{}, userID primitive.ObjectID, cosmeticID, slot, legacyValue string) (bool, error)
	EquipCosmetic(ctx interface{}, userID primitive.ObjectID, cosmeticID, slot string, requiresOwnership bool) error
	UnequipCosmetic(ctx interface{}, userID primitive.ObjectID, slot string) error
}

type cosmeticNotifier interface {
	CreateUserNotification(userID primitive.ObjectID, title, message, link, linkText string) error
}

type CosmeticService struct {
	users    cosmeticUserStore
	notifier cosmeticNotifier
}

func NewCosmeticService(users cosmeticUserStore, notifier ...cosmeticNotifier) *CosmeticService {
	service := &CosmeticService{users: users}
	if len(notifier) > 0 {
		service.notifier = notifier[0]
	}
	return service
}

func (s *CosmeticService) Grant(userID, cosmeticID, message string) (bool, error) {
	return s.grant(userID, cosmeticID, message, false)
}

func (s *CosmeticService) GrantCharacter(userID, cosmeticID, message string) (bool, error) {
	return s.grant(userID, cosmeticID, message, true)
}

func (s *CosmeticService) grant(userID, cosmeticID, message string, character bool) (bool, error) {
	id, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return false, errors.New("invalid user ID")
	}
	item, found := findCosmetic(cosmeticID)
	if !found || item.Starter || isCharacterSlot(item.Slot) != character {
		return false, errors.New("cosmetic is not eligible for an admin grant")
	}
	granted, err := s.users.GrantCosmetic(context.Background(), id, cosmeticID)
	if err != nil || !granted {
		return granted, err
	}
	if s.notifier != nil {
		body := message
		if body == "" {
			body = "A teacher added " + item.Name + " to your permanent collection."
		}
		link := "/learner/dashboard"
		if character {
			link = "/character"
		}
		if err := s.notifier.CreateUserNotification(id, "A new gift for you", body, link, "View collection"); err != nil {
			return true, err
		}
	}
	return true, nil
}

func (s *CosmeticService) Revoke(userID, cosmeticID string) (bool, error) {
	id, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return false, errors.New("invalid user ID")
	}
	item, found := findCosmetic(cosmeticID)
	if !found || item.Starter || isCharacterSlot(item.Slot) {
		return false, errors.New("cosmetic is not eligible for revocation")
	}
	return s.users.RevokeCosmetic(context.Background(), id, cosmeticID, item.Slot, item.PreviewValue)
}

func (s *CosmeticService) Equip(userID, slot, cosmeticID string) error {
	return s.equip(userID, slot, cosmeticID, false)
}

func (s *CosmeticService) EquipCharacter(userID, slot, cosmeticID string) error {
	return s.equip(userID, slot, cosmeticID, true)
}

func (s *CosmeticService) equip(userID, slot, cosmeticID string, character bool) error {
	id, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return errors.New("invalid user ID")
	}
	item, found := findCosmetic(cosmeticID)
	if !found {
		return errors.New("unknown cosmetic")
	}
	if item.Slot != slot || isCharacterSlot(slot) != character {
		return errors.New("cosmetic does not belong to this slot")
	}
	learner, err := s.users.FindByID(context.Background(), id)
	if err != nil {
		return err
	}
	owned := item.Starter
	for _, ownedID := range learner.OwnedCosmeticIDs {
		if ownedID == item.ID {
			owned = true
			break
		}
	}
	if !owned {
		return errors.New("cosmetic is not owned")
	}
	if !character && !cosmeticCompatible(item, learner.SelectedSpecies) {
		return errors.New("cosmetic is not compatible with this plant")
	}
	return s.users.EquipCosmetic(context.Background(), id, cosmeticID, item.Slot, !item.Starter)
}

func (s *CosmeticService) Unequip(userID, slot string) error {
	return s.unequip(userID, slot, false)
}

func (s *CosmeticService) UnequipCharacter(userID, slot string) error {
	return s.unequip(userID, slot, true)
}

func (s *CosmeticService) unequip(userID, slot string, character bool) error {
	id, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return errors.New("invalid user ID")
	}
	if !validCosmeticSlot(slot) || isCharacterSlot(slot) != character {
		return errors.New("unknown cosmetic slot")
	}
	return s.users.UnequipCosmetic(context.Background(), id, slot)
}

func validCosmeticSlot(slot string) bool {
	for _, candidate := range []string{"palette", "pot", "aura", "particle", "accessory", "mutation", "card_background", "character_prop"} {
		if candidate == slot {
			return true
		}
	}
	return false
}

func isCharacterSlot(slot string) bool {
	return slot == "card_background" || slot == "character_prop"
}

func cosmeticCompatible(item domain.CosmeticCatalogItem, species string) bool {
	if len(item.CompatibleSpecies) == 0 {
		return true
	}
	for _, candidate := range item.CompatibleSpecies {
		if candidate == species {
			return true
		}
	}
	return false
}

func findCosmetic(cosmeticID string) (domain.CosmeticCatalogItem, bool) {
	for _, item := range cosmeticCatalog {
		if item.ID == cosmeticID {
			return item, true
		}
	}
	return domain.CosmeticCatalogItem{}, false
}

var cosmeticCatalog = []domain.CosmeticCatalogItem{
	{ID: "palette:forest", Name: "Forest", Slot: "palette", Rarity: "Common", PreviewValue: "Forest", SourceHint: "Starter collection", RewardPools: []string{"starter"}, Starter: true},
	{ID: "palette:sunset", Name: "Sunset", Slot: "palette", Rarity: "Common", PreviewValue: "Sunset", SourceHint: "Existing plant collection", RewardPools: []string{"legacy"}},
	{ID: "palette:ocean", Name: "Ocean", Slot: "palette", Rarity: "Rare", PreviewValue: "Ocean", SourceHint: "Reflection rewards", RewardPools: []string{"achievement", "reflection", "teacher-box"}},
	{ID: "palette:desert", Name: "Desert", Slot: "palette", Rarity: "Common", PreviewValue: "Desert", SourceHint: "Existing plant collection", RewardPools: []string{"legacy"}},
	{ID: "palette:rose", Name: "Rose", Slot: "palette", Rarity: "Common", PreviewValue: "Rose", SourceHint: "Existing plant collection", RewardPools: []string{"legacy"}},
	{ID: "palette:lavender", Name: "Lavender", Slot: "palette", Rarity: "Common", PreviewValue: "Lavender", SourceHint: "Existing plant collection", RewardPools: []string{"legacy"}},
	{ID: "palette:sunshine", Name: "Sunshine", Slot: "palette", Rarity: "Common", PreviewValue: "Sunshine", SourceHint: "Existing plant collection", RewardPools: []string{"legacy"}},
	{ID: "palette:mint", Name: "Mint", Slot: "palette", Rarity: "Common", PreviewValue: "Mint", SourceHint: "Existing plant collection", RewardPools: []string{"legacy"}},
	{ID: "palette:coral", Name: "Coral", Slot: "palette", Rarity: "Common", PreviewValue: "Coral", SourceHint: "Existing plant collection", RewardPools: []string{"legacy"}},
	{ID: "palette:autumn", Name: "Autumn", Slot: "palette", Rarity: "Common", PreviewValue: "Autumn", SourceHint: "Existing plant collection", RewardPools: []string{"legacy"}},
	{ID: "palette:jade", Name: "Jade", Slot: "palette", Rarity: "Common", PreviewValue: "Jade", SourceHint: "Existing plant collection", RewardPools: []string{"legacy"}},
	{ID: "palette:berry", Name: "Berry", Slot: "palette", Rarity: "Common", PreviewValue: "Berry", SourceHint: "Existing plant collection", RewardPools: []string{"legacy"}},
	{ID: "palette:citrus", Name: "Citrus", Slot: "palette", Rarity: "Common", PreviewValue: "Citrus", SourceHint: "Existing plant collection", RewardPools: []string{"legacy"}},
	{ID: "palette:slate", Name: "Slate", Slot: "palette", Rarity: "Common", PreviewValue: "Slate", SourceHint: "Existing plant collection", RewardPools: []string{"legacy"}},
	{ID: "palette:blush", Name: "Blush", Slot: "palette", Rarity: "Common", PreviewValue: "Blush", SourceHint: "Existing plant collection", RewardPools: []string{"legacy"}},
	{ID: "palette:midnight", Name: "Midnight", Slot: "palette", Rarity: "Epic", PreviewValue: "Midnight", SourceHint: "Achievement rewards", RewardPools: []string{"achievement", "reflection", "teacher-box"}},
	{ID: "pot:round", Name: "Cozy Round Pot", Slot: "pot", Rarity: "Common", PreviewValue: "round", SourceHint: "Starter collection", RewardPools: []string{"starter"}, Starter: true},
	{ID: "pot:square", Name: "Square Pot", Slot: "pot", Rarity: "Common", PreviewValue: "square", SourceHint: "Existing plant collection", RewardPools: []string{"legacy"}},
	{ID: "pot:tall", Name: "Tall Pot", Slot: "pot", Rarity: "Common", PreviewValue: "tall", SourceHint: "Existing plant collection", RewardPools: []string{"legacy"}},
	{ID: "pot:bowl", Name: "Bowl Pot", Slot: "pot", Rarity: "Common", PreviewValue: "bowl", SourceHint: "Existing plant collection", RewardPools: []string{"legacy"}},
	{ID: "pot:trophy", Name: "Golden Trophy Pot", Slot: "pot", Rarity: "Rare", PreviewValue: "trophy", SourceHint: "Existing teacher reward", RewardPools: []string{"legacy"}},
	{ID: "pot:starlight", Name: "Starlight Pot", Slot: "pot", Rarity: "Rare", PreviewValue: "starlight", SourceHint: "Teacher Gift Boxes", RewardPools: []string{"achievement", "teacher-box"}},
	{ID: "pot:rainbow", Name: "Rainbow Pot", Slot: "pot", Rarity: "Rare", PreviewValue: "rainbow", SourceHint: "Existing teacher reward", RewardPools: []string{"legacy"}},
	{ID: "pot:crystal", Name: "Crystal Pot", Slot: "pot", Rarity: "Legendary", PreviewValue: "crystal", SourceHint: "Special achievements", RewardPools: []string{"achievement", "reflection", "teacher-box"}},
	{ID: "pot:sweetheart", Name: "Sweetheart Pot", Slot: "pot", Rarity: "Rare", PreviewValue: "sweetheart", SourceHint: "Existing teacher reward", RewardPools: []string{"legacy"}},
	{ID: "pot:laurel", Name: "Laurel Medal Pot", Slot: "pot", Rarity: "Rare", PreviewValue: "laurel", SourceHint: "Existing teacher reward", RewardPools: []string{"legacy"}},
	{ID: "pot:constellation", Name: "Constellation Pot", Slot: "pot", Rarity: "Epic", PreviewValue: "constellation", SourceHint: "Existing teacher reward", RewardPools: []string{"legacy"}},
	{ID: "pot:mosaic", Name: "Mosaic Pot", Slot: "pot", Rarity: "Rare", PreviewValue: "mosaic", SourceHint: "Existing teacher reward", RewardPools: []string{"legacy"}},
	{ID: "pot:royal", Name: "Royal Pot", Slot: "pot", Rarity: "Epic", PreviewValue: "royal", SourceHint: "Existing teacher reward", RewardPools: []string{"legacy"}},
	{ID: "pot:firework", Name: "Firework Pot", Slot: "pot", Rarity: "Epic", PreviewValue: "firework", SourceHint: "Existing teacher reward", RewardPools: []string{"legacy"}},
	{ID: "aura:morning-mist", Name: "Morning Mist", Slot: "aura", Rarity: "Common", PreviewValue: "morning-mist", SourceHint: "Starter collection", RewardPools: []string{"starter"}, Starter: true},
	{ID: "aura:firefly", Name: "Firefly Glow", Slot: "aura", Rarity: "Rare", PreviewValue: "firefly", SourceHint: "Consistency rewards", RewardPools: []string{"achievement", "reflection", "teacher-box"}},
	{ID: "aura:aurora", Name: "Aurora Halo", Slot: "aura", Rarity: "Legendary", PreviewValue: "aurora", SourceHint: "Secret achievements", RewardPools: []string{"achievement", "reflection"}},
	{ID: "particle:pollen", Name: "Gentle Pollen", Slot: "particle", Rarity: "Common", PreviewValue: "pollen", SourceHint: "Starter collection", RewardPools: []string{"starter"}, Starter: true},
	{ID: "particle:petals", Name: "Falling Petals", Slot: "particle", Rarity: "Rare", PreviewValue: "petals", SourceHint: "Reflection rewards", RewardPools: []string{"achievement", "reflection", "teacher-box"}},
	{ID: "particle:stars", Name: "Tiny Stars", Slot: "particle", Rarity: "Epic", PreviewValue: "stars", SourceHint: "Social achievements", RewardPools: []string{"achievement", "reflection", "teacher-box"}},
	{ID: "accessory:ladybug", Name: "Ladybug Friend", Slot: "accessory", Rarity: "Common", PreviewValue: "ladybug", SourceHint: "Starter collection", RewardPools: []string{"starter"}, Starter: true},
	{ID: "accessory:ribbon", Name: "Garden Ribbon", Slot: "accessory", Rarity: "Rare", PreviewValue: "ribbon", SourceHint: "Teacher Gift Boxes", RewardPools: []string{"achievement", "teacher-box"}},
	{ID: "accessory:crown", Name: "Little Crown", Slot: "accessory", Rarity: "Legendary", PreviewValue: "crown", SourceHint: "Special achievements", RewardPools: []string{"achievement", "reflection"}},
	{ID: "mutation:speckled", Name: "Speckled Leaves", Slot: "mutation", Rarity: "Common", PreviewValue: "speckled", SourceHint: "Starter collection", RewardPools: []string{"starter"}, Starter: true},
	{ID: "mutation:variegated", Name: "Variegated Leaves", Slot: "mutation", Rarity: "Epic", PreviewValue: "variegated", SourceHint: "Comeback achievements", RewardPools: []string{"achievement", "reflection", "teacher-box"}},
	{ID: "mutation:crystal", Name: "Crystal Growth", Slot: "mutation", Rarity: "Legendary", PreviewValue: "crystal", SourceHint: "Secret achievements", RewardPools: []string{"achievement", "reflection"}},
	{ID: "card_background:meadow", Name: "Cozy Meadow", Slot: "card_background", Rarity: "Common", PreviewValue: "meadow", SourceHint: "Starter collection", RewardPools: []string{"starter"}, Starter: true},
	{ID: "card_background:sunset", Name: "Peach Sunset", Slot: "card_background", Rarity: "Rare", PreviewValue: "sunset", SourceHint: "Teacher Gift Boxes", RewardPools: []string{"character-box"}},
	{ID: "card_background:night", Name: "Starry Night", Slot: "card_background", Rarity: "Epic", PreviewValue: "night", SourceHint: "Teacher Gift Boxes", RewardPools: []string{"character-box"}},
	{ID: "card_background:rainbow", Name: "Rainbow Picnic", Slot: "card_background", Rarity: "Legendary", PreviewValue: "rainbow", SourceHint: "Teacher Gift Boxes", RewardPools: []string{"character-box"}},
	{ID: "character_prop:flower", Name: "Little Flower", Slot: "character_prop", Rarity: "Common", PreviewValue: "flower", SourceHint: "Teacher Gift Boxes", RewardPools: []string{"character-box"}},
	{ID: "character_prop:cat-ears", Name: "Cat Ear Headband", Slot: "character_prop", Rarity: "Rare", PreviewValue: "cat-ears", SourceHint: "Teacher Gift Boxes", RewardPools: []string{"character-box"}},
	{ID: "character_prop:egg", Name: "Fried Egg Pin", Slot: "character_prop", Rarity: "Epic", PreviewValue: "egg", SourceHint: "Teacher Gift Boxes", RewardPools: []string{"character-box"}},
	{ID: "character_prop:halo", Name: "Tiny Halo", Slot: "character_prop", Rarity: "Legendary", PreviewValue: "halo", SourceHint: "Teacher Gift Boxes", RewardPools: []string{"character-box"}},
}

func (s *CosmeticService) Catalog() []domain.CosmeticCatalogItem {
	return filteredCosmeticCatalog(false)
}

func (s *CosmeticService) CharacterCatalog() []domain.CosmeticCatalogItem {
	return filteredCosmeticCatalog(true)
}

func filteredCosmeticCatalog(character bool) []domain.CosmeticCatalogItem {
	items := make([]domain.CosmeticCatalogItem, 0, len(cosmeticCatalog))
	for _, item := range cosmeticCatalog {
		if isCharacterSlot(item.Slot) == character {
			items = append(items, item)
		}
	}
	return items
}

func (s *CosmeticService) Collection(userID string) (*domain.CosmeticCollection, error) {
	return s.collection(userID, false)
}

func (s *CosmeticService) CharacterCollection(userID string) (*domain.CosmeticCollection, error) {
	return s.collection(userID, true)
}

func (s *CosmeticService) collection(userID string, character bool) (*domain.CosmeticCollection, error) {
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
	for _, catalogItem := range filteredCosmeticCatalog(character) {
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
