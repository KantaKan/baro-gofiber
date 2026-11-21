package user

import (
	"errors"
	"testing"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type cosmeticUserReaderStub struct {
	user               *domain.User
	owned              map[string]bool
	equipped           map[string]string
	revokedLegacyValue *string
}

func (s cosmeticUserReaderStub) EquipCosmetic(_ interface{}, _ primitive.ObjectID, cosmeticID, slot string, requiresOwnership bool) error {
	if requiresOwnership && !s.owned[cosmeticID] {
		return errors.New("cosmetic is not owned")
	}
	s.equipped[slot] = cosmeticID
	return nil
}

func (s cosmeticUserReaderStub) UnequipCosmetic(_ interface{}, _ primitive.ObjectID, slot string) error {
	delete(s.equipped, slot)
	return nil
}

func (s cosmeticUserReaderStub) GrantCosmetic(_ interface{}, _ primitive.ObjectID, cosmeticID string) (bool, error) {
	if s.owned[cosmeticID] {
		return false, nil
	}
	s.owned[cosmeticID] = true
	return true, nil
}

func (s cosmeticUserReaderStub) RevokeCosmetic(_ interface{}, _ primitive.ObjectID, cosmeticID, _, legacyValue string) (bool, error) {
	if s.revokedLegacyValue != nil {
		*s.revokedLegacyValue = legacyValue
	}
	if !s.owned[cosmeticID] {
		return false, nil
	}
	delete(s.owned, cosmeticID)
	return true, nil
}

type cosmeticNotifierStub struct {
	recipients []primitive.ObjectID
	messages   []string
}

func (s *cosmeticNotifierStub) CreateUserNotification(userID primitive.ObjectID, _, message, _, _ string) error {
	s.recipients = append(s.recipients, userID)
	s.messages = append(s.messages, message)
	return nil
}

func (s cosmeticUserReaderStub) FindByID(_ interface{}, _ primitive.ObjectID) (*domain.User, error) {
	return s.user, nil
}

func TestCosmeticOwnershipIsDuplicateSafe(t *testing.T) {
	userID := primitive.NewObjectID()
	owned := map[string]bool{}
	legacyValue := ""
	notifier := &cosmeticNotifierStub{}
	service := NewCosmeticService(cosmeticUserReaderStub{owned: owned, revokedLegacyValue: &legacyValue}, notifier)

	granted, err := service.Grant(userID.Hex(), "palette:ocean", "Your thoughtful reflection earned this color.")
	if err != nil || !granted {
		t.Fatalf("first grant failed: granted=%v err=%v", granted, err)
	}
	granted, err = service.Grant(userID.Hex(), "palette:ocean", "Duplicate")
	if err != nil || granted {
		t.Fatalf("duplicate grant was not ignored: granted=%v err=%v", granted, err)
	}
	if len(notifier.messages) != 1 || notifier.messages[0] != "Your thoughtful reflection earned this color." || notifier.recipients[0] != userID {
		t.Fatalf("grant notification was incorrect: recipients=%v messages=%v", notifier.recipients, notifier.messages)
	}

	revoked, err := service.Revoke(userID.Hex(), "palette:ocean")
	if err != nil || !revoked || owned["palette:ocean"] {
		t.Fatalf("revoke failed: revoked=%v err=%v owned=%v", revoked, err, owned)
	}
	if legacyValue != "Ocean" {
		t.Fatalf("revoke did not identify matching legacy selection: %q", legacyValue)
	}
}

func TestCosmeticGrantRejectsStarterAndUnknownItems(t *testing.T) {
	service := NewCosmeticService(cosmeticUserReaderStub{owned: map[string]bool{}})
	for _, cosmeticID := range []string{"pot:round", "unknown:item"} {
		if granted, err := service.Grant(primitive.NewObjectID().Hex(), cosmeticID, ""); err == nil || granted {
			t.Fatalf("ineligible item %q was granted", cosmeticID)
		}
	}
}

func TestCosmeticCollectionStates(t *testing.T) {
	userID := primitive.NewObjectID()
	service := NewCosmeticService(cosmeticUserReaderStub{user: &domain.User{
		ID:                userID,
		OwnedCosmeticIDs:  []string{"palette:ocean"},
		NewCosmeticIDs:    []string{"palette:ocean"},
		EquippedCosmetics: map[string]string{"palette": "palette:ocean"},
	}})

	collection, err := service.Collection(userID.Hex())
	if err != nil {
		t.Fatalf("Collection returned an error: %v", err)
	}

	states := make(map[string]domain.CosmeticCollectionItem, len(collection.Items))
	for _, item := range collection.Items {
		states[item.ID] = item
	}

	if item := states["palette:ocean"]; !item.Owned || !item.New || !item.Equipped || item.Locked {
		t.Fatalf("owned item state was incorrect: %+v", item)
	}
	if item := states["pot:round"]; !item.Owned || item.Locked {
		t.Fatalf("starter item state was incorrect: %+v", item)
	}
	if item := states["mutation:crystal"]; item.Owned || !item.Locked {
		t.Fatalf("locked item state was incorrect: %+v", item)
	}
}

func TestCosmeticCatalogCoversEverySlotAndRarity(t *testing.T) {
	service := NewCosmeticService(cosmeticUserReaderStub{})
	slots := map[string]bool{}
	rarities := map[string]bool{}
	for _, item := range service.Catalog() {
		slots[item.Slot] = true
		rarities[item.Rarity] = true
		if item.SourceHint == "" || len(item.RewardPools) == 0 {
			t.Fatalf("catalog item lacks disclosure metadata: %+v", item)
		}
	}
	for _, slot := range []string{"palette", "pot", "aura", "particle", "accessory", "mutation"} {
		if !slots[slot] {
			t.Fatalf("catalog is missing slot %s", slot)
		}
	}
	for _, rarity := range []string{"Common", "Rare", "Epic", "Legendary"} {
		if !rarities[rarity] {
			t.Fatalf("catalog is missing rarity %s", rarity)
		}
	}
}

func TestEquipValidatesEverySlotOwnershipAndCompatibility(t *testing.T) {
	userID := primitive.NewObjectID()
	owned := map[string]bool{}
	equipped := map[string]string{}
	for _, item := range cosmeticCatalog {
		if !item.Starter {
			owned[item.ID] = true
		}
	}
	service := NewCosmeticService(cosmeticUserReaderStub{
		user:     &domain.User{ID: userID, SelectedSpecies: "Lotus", OwnedCosmeticIDs: mapKeys(owned)},
		owned:    owned,
		equipped: equipped,
	})
	seen := map[string]bool{}
	for _, item := range service.Catalog() {
		if seen[item.Slot] {
			continue
		}
		seen[item.Slot] = true
		if err := service.Equip(userID.Hex(), item.Slot, item.ID); err != nil {
			t.Fatalf("Equip rejected valid %s item: %v", item.Slot, err)
		}
		if equipped[item.Slot] != item.ID {
			t.Fatalf("Equip did not save %s item", item.Slot)
		}
		if err := service.Equip(userID.Hex(), item.Slot, item.ID); err != nil {
			t.Fatalf("retrying Equip for %s failed: %v", item.Slot, err)
		}
	}
	if len(seen) != 6 {
		t.Fatalf("tested %d slots, want 6", len(seen))
	}
	if err := service.Equip(userID.Hex(), "pot", "palette:ocean"); err == nil {
		t.Fatal("Equip accepted a cosmetic in the wrong slot")
	}
	if err := service.Unequip(userID.Hex(), "not-a-slot"); err == nil {
		t.Fatal("Unequip accepted an unknown slot")
	}
}

func TestEquipRejectsUnownedAndIncompatibleCosmetics(t *testing.T) {
	userID := primitive.NewObjectID()
	service := NewCosmeticService(cosmeticUserReaderStub{
		user:     &domain.User{ID: userID, SelectedSpecies: "Lotus"},
		owned:    map[string]bool{},
		equipped: map[string]string{},
	})
	if err := service.Equip(userID.Hex(), "palette", "palette:ocean"); err == nil {
		t.Fatal("Equip accepted an unowned cosmetic")
	}

	incompatible := domain.CosmeticCatalogItem{ID: "test:species", Slot: "accessory", CompatibleSpecies: []string{"Mango"}}
	if cosmeticCompatible(incompatible, "Lotus") {
		t.Fatal("compatibility check accepted the wrong species")
	}
}

func TestCharacterCosmeticsStaySeparateAndPermanent(t *testing.T) {
	userID := primitive.NewObjectID()
	owned := map[string]bool{"palette:ocean": true}
	equipped := map[string]string{"palette": "palette:ocean"}
	learner := &domain.User{ID: userID, OwnedCosmeticIDs: []string{"palette:ocean"}, EquippedCosmetics: equipped}
	service := NewCosmeticService(cosmeticUserReaderStub{user: learner, owned: owned, equipped: equipped})

	granted, err := service.GrantCharacter(userID.Hex(), "character_prop:flower", "")
	if err != nil || !granted {
		t.Fatalf("character grant failed: granted=%v err=%v", granted, err)
	}
	learner.OwnedCosmeticIDs = append(learner.OwnedCosmeticIDs, "character_prop:flower")
	if err := service.EquipCharacter(userID.Hex(), "character_prop", "character_prop:flower"); err != nil {
		t.Fatalf("character equip failed: %v", err)
	}
	if equipped["palette"] != "palette:ocean" || equipped["character_prop"] != "character_prop:flower" || !owned["palette:ocean"] {
		t.Fatalf("character item erased plant item: equipped=%v owned=%v", equipped, owned)
	}
	characterCollection, err := service.CharacterCollection(userID.Hex())
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range characterCollection.Items {
		if !isCharacterSlot(item.Slot) {
			t.Fatalf("plant item leaked into character collection: %s", item.ID)
		}
		if item.ID == "character_prop:flower" && (!item.Owned || !item.Equipped) {
			t.Fatalf("character item state incorrect: %+v", item)
		}
	}
	plantCollection, err := service.Collection(userID.Hex())
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range plantCollection.Items {
		if isCharacterSlot(item.Slot) {
			t.Fatalf("character item leaked into plant collection: %s", item.ID)
		}
	}
}

func TestCharacterCosmeticSlotAndOwnershipChecks(t *testing.T) {
	userID := primitive.NewObjectID()
	service := NewCosmeticService(cosmeticUserReaderStub{
		user: &domain.User{ID: userID}, owned: map[string]bool{}, equipped: map[string]string{},
	})
	for _, test := range []struct{ slot, id string }{
		{"character_prop", "character_prop:flower"},
		{"card_background", "character_prop:flower"},
		{"palette", "character_prop:flower"},
	} {
		if err := service.EquipCharacter(userID.Hex(), test.slot, test.id); err == nil {
			t.Fatalf("unexpected equip: %s / %s", test.slot, test.id)
		}
	}
	if err := service.Equip(userID.Hex(), "character_prop", "character_prop:flower"); err == nil {
		t.Fatal("plant equip accepted a character slot")
	}
	if granted, err := service.Grant(userID.Hex(), "character_prop:flower", ""); err == nil || granted {
		t.Fatal("plant grant accepted a character item")
	}
	if granted, err := service.GrantCharacter(userID.Hex(), "palette:ocean", ""); err == nil || granted {
		t.Fatal("character grant accepted a plant item")
	}
	if revoked, err := service.Revoke(userID.Hex(), "character_prop:flower"); err == nil || revoked {
		t.Fatal("plant revoke accepted a permanent character item")
	}
	if err := service.UnequipCharacter(userID.Hex(), "palette"); err == nil {
		t.Fatal("character unequip accepted a plant slot")
	}
}

func mapKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}
