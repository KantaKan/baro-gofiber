package user

import (
	"testing"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type cosmeticUserReaderStub struct {
	user  *domain.User
	owned map[string]bool
}

func (s cosmeticUserReaderStub) GrantCosmetic(_ interface{}, _ primitive.ObjectID, cosmeticID string) (bool, error) {
	if s.owned[cosmeticID] {
		return false, nil
	}
	s.owned[cosmeticID] = true
	return true, nil
}

func (s cosmeticUserReaderStub) RevokeCosmetic(_ interface{}, _ primitive.ObjectID, cosmeticID, _ string) (bool, error) {
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
	notifier := &cosmeticNotifierStub{}
	service := NewCosmeticService(cosmeticUserReaderStub{owned: owned}, notifier)

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
