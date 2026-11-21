package user

import (
	"errors"
	"testing"
	"time"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type careEnergyRepoStub struct {
	domain.UserRepository
	user *domain.User
}

func (s *careEnergyRepoStub) FindByID(_ interface{}, _ primitive.ObjectID) (*domain.User, error) {
	return s.user, nil
}

func (s *careEnergyRepoStub) UseCareEnergyCharacter(_ interface{}, _ primitive.ObjectID, effect string) error {
	if s.user.CareEnergyBalance < 1 {
		return domain.ErrInsufficientCareEnergy
	}
	s.user.CareEnergyBalance--
	s.user.CharacterCareCount++
	now := time.Now().UTC()
	s.user.LastCaredAt = &now
	s.user.CareEnergyLog = append(s.user.CareEnergyLog, domain.CareEnergyLogEntry{Kind: "character-care", Amount: 1, Note: effect, CreatedAt: now})
	return nil
}

func TestCareEnergyReadsCanonicalBalance(t *testing.T) {
	userID := primitive.NewObjectID()
	log := []domain.CareEnergyLogEntry{{Kind: "grant", Amount: 4}}
	repo := &careEnergyRepoStub{user: &domain.User{ID: userID, CareEnergyBalance: 4, CareEnergyLog: log}}
	service := NewCareEnergyService(repo, nil)

	state, err := service.CareEnergyState(userID)
	if err != nil {
		t.Fatal(err)
	}
	if state.Balance != 4 {
		t.Fatalf("unexpected balance: %+v", state)
	}
	if len(state.Log) != 1 || state.Log[0].Kind != "grant" {
		t.Fatalf("log was not exposed: %+v", state)
	}
}

func TestUserResponseKeepsCareEnergyFields(t *testing.T) {
	userID := primitive.NewObjectID()
	log := []domain.CareEnergyLogEntry{{Kind: "grant", Amount: 2}}
	repo := &careEnergyRepoStub{user: &domain.User{ID: userID, CareEnergyBalance: 2, CareEnergyLog: log}}
	service := NewService(repo)

	result, err := service.GetUserByID(userID.Hex())
	if err != nil {
		t.Fatal(err)
	}
	if result.CareEnergyBalance != 2 || len(result.CareEnergyLog) != 1 {
		t.Fatalf("Care Energy fields were lost: %+v", result)
	}
	safe := result.ToSafe()
	if safe.CareEnergyBalance != 2 || len(safe.CareEnergyLog) != 1 {
		t.Fatalf("safe response lost Care Energy fields: %+v", safe)
	}
}

func TestCareCharacterSpendsOneWithoutChangingGrowthOrRewards(t *testing.T) {
	userID := primitive.NewObjectID()
	boxID := primitive.NewObjectID()
	repo := &careEnergyRepoStub{user: &domain.User{
		ID:                userID,
		CareEnergyBalance: 3,
		GrowthPoints:      70,
		GiftBoxes:         []domain.TeacherGiftBox{{ID: boxID}},
	}}
	service := NewCareEnergyService(repo, nil)

	result, err := service.CareCharacter(userID)
	if err != nil {
		t.Fatal(err)
	}
	if result.Effect != "happy-hop" || result.State.Balance != 2 {
		t.Fatalf("unexpected care result: %+v", result)
	}
	if repo.user.GrowthPoints != 70 {
		t.Fatalf("character care changed main growth: %d", repo.user.GrowthPoints)
	}
	if len(repo.user.GiftBoxes) != 1 || repo.user.GiftBoxes[0].ID != boxID {
		t.Fatalf("character care changed reward boxes: %+v", repo.user.GiftBoxes)
	}
	if repo.user.CharacterCareCount != 1 || len(repo.user.CareEnergyLog) != 1 || repo.user.CareEnergyLog[0].Kind != "character-care" {
		t.Fatalf("character care was not recorded: %+v", repo.user)
	}
}

func TestCareCharacterCannotSpendEmptyBalance(t *testing.T) {
	repo := &careEnergyRepoStub{user: &domain.User{ID: primitive.NewObjectID()}}
	service := NewCareEnergyService(repo, nil)

	_, err := service.CareCharacter(repo.user.ID)
	if !errors.Is(err, domain.ErrInsufficientCareEnergy) {
		t.Fatalf("expected insufficient balance, got %v", err)
	}
	if repo.user.CharacterCareCount != 0 || len(repo.user.CareEnergyLog) != 0 {
		t.Fatalf("failed care mutated the account: %+v", repo.user)
	}
}
