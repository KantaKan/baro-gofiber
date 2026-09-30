package giftbox

import (
	"context"
	"errors"
	"strings"
	"time"

	"gofiber-baro/internal/domain"
	"gofiber-baro/internal/service/reward"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

var ErrBoxNotFound = errors.New("gift box not found")
var ErrSameRecipient = errors.New("cannot send a gift box to yourself")
var ErrRecipientNotFound = errors.New("recipient not found")
var ErrCharacterEggNotReady = errors.New("character egg hatching is not available yet")
var ErrInvalidCharacterEggTier = errors.New("character eggs must use the Standard tier")

const CharacterEggPool = "character-egg"

type Recipient struct {
	ID           string `json:"id"`
	DisplayName  string `json:"display_name"`
	CohortNumber int    `json:"cohort_number"`
	Group        string `json:"group"`
	Role         string `json:"role"`
}

type Store interface {
	Create(ctx context.Context, box domain.TeacherGiftBox) error
	CreateOnce(ctx context.Context, box domain.TeacherGiftBox) (bool, error)
	ListForUser(ctx context.Context, userID primitive.ObjectID) ([]domain.TeacherGiftBox, error)
	ListCohortLearners(ctx context.Context, cohort int) ([]primitive.ObjectID, error)
	ListTeamLearners(ctx context.Context, cohort int, team string) ([]primitive.ObjectID, error)
	SearchRecipients(ctx context.Context, query string, exclude primitive.ObjectID) ([]Recipient, error)
	IsLearner(ctx context.Context, userID primitive.ObjectID) (bool, error)
	Transfer(ctx context.Context, boxID, fromID, toID primitive.ObjectID) (*domain.TeacherGiftBox, error)
}

type CohortGrantFailure struct {
	UserID string `json:"user_id"`
	Error  string `json:"error"`
}

type CohortGrantResult struct {
	Total    int                  `json:"total"`
	Created  int                  `json:"created"`
	Existing int                  `json:"existing"`
	Failures []CohortGrantFailure `json:"failures"`
}

type AudiencePreview struct {
	Cohort int    `json:"cohort"`
	Team   string `json:"team,omitempty"`
	Total  int    `json:"total"`
}

type Drawer interface {
	Open(ctx context.Context, request reward.DrawRequest) (*reward.DrawResult, error)
	Odds(ctx context.Context, request reward.DrawRequest) (reward.Eligibility, error)
}

type EggHatcher interface {
	Hatch(ctx context.Context, ownerID, eggID string) (*domain.BaroCharacter, error)
}

type OpenResult struct {
	Kind string `json:"kind"`
	*reward.DrawResult
	Character *domain.BaroCharacter `json:"character,omitempty"`
}

type Service struct {
	store   Store
	drawer  Drawer
	hatcher EggHatcher
}

func NewService(store Store, drawer Drawer, hatchers ...EggHatcher) *Service {
	service := &Service{store: store, drawer: drawer}
	if len(hatchers) > 0 {
		service.hatcher = hatchers[0]
	}
	return service
}

func (s *Service) Grant(ctx context.Context, userID, adminID, minimumRarity, message string) (*domain.TeacherGiftBox, error) {
	return s.GrantWithPool(ctx, userID, adminID, minimumRarity, message, "")
}

func (s *Service) GrantWithPool(ctx context.Context, userID, adminID, minimumRarity, message, rewardPool string) (*domain.TeacherGiftBox, error) {
	if rewardPool != "" && rewardPool != "character-box" && rewardPool != CharacterEggPool {
		return nil, errors.New("invalid reward pool")
	}
	if rewardPool == CharacterEggPool && minimumRarity != "Common" {
		return nil, ErrInvalidCharacterEggTier
	}
	learner, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, errors.New("invalid learner ID")
	}
	admin, err := primitive.ObjectIDFromHex(adminID)
	if err != nil {
		return nil, errors.New("invalid admin ID")
	}
	if rewardPool == CharacterEggPool {
		eligible, err := s.store.IsLearner(ctx, learner)
		if err != nil {
			return nil, err
		}
		if !eligible {
			return nil, ErrRecipientNotFound
		}
	}
	box := domain.TeacherGiftBox{
		ID:            primitive.NewObjectID(),
		UserID:        learner,
		MinimumRarity: minimumRarity,
		Message:       strings.TrimSpace(message),
		GrantedBy:     admin,
		Status:        "unopened",
		CreatedAt:     time.Now(),
		RewardPool:    rewardPool,
	}
	if err := s.store.Create(ctx, box); err != nil {
		return nil, err
	}
	return &box, nil
}

func (s *Service) GrantCohort(ctx context.Context, cohort int, adminID, minimumRarity, message, idempotencyKey string) (*CohortGrantResult, error) {
	return s.GrantAudience(ctx, cohort, "", adminID, minimumRarity, message, idempotencyKey, "")
}

func (s *Service) PreviewAudience(ctx context.Context, cohort int, team string) (*AudiencePreview, error) {
	learners, err := s.audienceLearners(ctx, cohort, team)
	if err != nil {
		return nil, err
	}
	return &AudiencePreview{Cohort: cohort, Team: strings.TrimSpace(team), Total: len(learners)}, nil
}

func (s *Service) audienceLearners(ctx context.Context, cohort int, team string) ([]primitive.ObjectID, error) {
	team = strings.TrimSpace(team)
	if cohort <= 0 || len([]rune(team)) > 100 {
		return nil, errors.New("valid cohort and team are required")
	}
	if team != "" {
		return s.store.ListTeamLearners(ctx, cohort, team)
	}
	return s.store.ListCohortLearners(ctx, cohort)
}

func (s *Service) GrantAudience(ctx context.Context, cohort int, team, adminID, minimumRarity, message, idempotencyKey, rewardPool string) (*CohortGrantResult, error) {
	admin, err := primitive.ObjectIDFromHex(adminID)
	if err != nil {
		return nil, errors.New("invalid admin ID")
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	team = strings.TrimSpace(team)
	if idempotencyKey == "" || len(idempotencyKey) > 128 || (rewardPool != "" && rewardPool != "character-box") {
		return nil, errors.New("valid idempotency key and reward pool are required")
	}
	learners, err := s.audienceLearners(ctx, cohort, team)
	if err != nil {
		return nil, err
	}
	result := &CohortGrantResult{Total: len(learners), Failures: []CohortGrantFailure{}}
	for _, learner := range learners {
		box := domain.TeacherGiftBox{
			ID: primitive.NewObjectID(), UserID: learner, MinimumRarity: minimumRarity,
			Message: strings.TrimSpace(message), GrantedBy: admin, Status: "unopened",
			CreatedAt: time.Now(), GrantKey: idempotencyKey + ":" + learner.Hex(), RewardPool: rewardPool,
		}
		created, createErr := s.store.CreateOnce(ctx, box)
		if createErr != nil {
			result.Failures = append(result.Failures, CohortGrantFailure{UserID: learner.Hex(), Error: createErr.Error()})
		} else if created {
			result.Created++
		} else {
			result.Existing++
		}
	}
	return result, nil
}

func (s *Service) List(ctx context.Context, userID string) ([]domain.TeacherGiftBox, error) {
	id, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, errors.New("invalid learner ID")
	}
	return s.store.ListForUser(ctx, id)
}

func (s *Service) SearchRecipients(ctx context.Context, senderID, query string) ([]Recipient, error) {
	id, err := primitive.ObjectIDFromHex(senderID)
	if err != nil {
		return nil, errors.New("invalid sender ID")
	}
	query = strings.TrimSpace(query)
	if len([]rune(query)) < 2 || len([]rune(query)) > 80 {
		return nil, errors.New("search must be 2 to 80 characters")
	}
	recipients, err := s.store.SearchRecipients(ctx, query, id)
	if err != nil {
		return nil, err
	}
	learners := make([]Recipient, 0, len(recipients))
	for _, recipient := range recipients {
		if recipient.Role == "learner" {
			learners = append(learners, recipient)
		}
	}
	return learners, nil
}

func (s *Service) Transfer(ctx context.Context, senderID, boxID, recipientID string) (*domain.TeacherGiftBox, error) {
	from, err := primitive.ObjectIDFromHex(senderID)
	if err != nil {
		return nil, errors.New("invalid sender ID")
	}
	box, err := primitive.ObjectIDFromHex(boxID)
	if err != nil {
		return nil, errors.New("invalid gift box ID")
	}
	to, err := primitive.ObjectIDFromHex(recipientID)
	if err != nil {
		return nil, errors.New("invalid recipient ID")
	}
	if from == to {
		return nil, ErrSameRecipient
	}
	eligible, err := s.store.IsLearner(ctx, to)
	if err != nil {
		return nil, err
	}
	if !eligible {
		return nil, ErrRecipientNotFound
	}
	return s.store.Transfer(ctx, box, from, to)
}

func (s *Service) Open(ctx context.Context, userID, boxID string) (*OpenResult, error) {
	box, err := s.boxForUser(ctx, userID, boxID)
	if err != nil {
		return nil, err
	}
	if box.RewardPool == CharacterEggPool {
		if s.hatcher == nil {
			return nil, ErrCharacterEggNotReady
		}
		character, err := s.hatcher.Hatch(ctx, userID, boxID)
		if err != nil {
			return nil, err
		}
		return &OpenResult{Kind: "character", Character: character}, nil
	}
	result, err := s.drawer.Open(ctx, reward.DrawRequest{
		IdempotencyKey: box.ID.Hex(), UserID: userID,
		Pool: drawPool(box.Source, box.RewardPool), MinimumRarity: box.MinimumRarity,
	})
	if err != nil {
		return nil, err
	}
	if result == nil || result.UserID != userID {
		return nil, ErrBoxNotFound
	}
	return &OpenResult{Kind: "cosmetic", DrawResult: result}, nil
}

func (s *Service) Odds(ctx context.Context, userID, boxID string) (reward.Eligibility, error) {
	box, err := s.boxForUser(ctx, userID, boxID)
	if err != nil {
		return reward.Eligibility{}, err
	}
	if box.RewardPool == CharacterEggPool {
		return reward.Eligibility{
			EligibleCount: 1,
			Odds: map[string]float64{
				"Normal":    0.83,
				"Meme Rare": 0.15,
				"Legendary": 0.02,
			},
		}, nil
	}
	return s.drawer.Odds(ctx, reward.DrawRequest{
		IdempotencyKey: box.ID.Hex(), UserID: userID,
		Pool: drawPool(box.Source, box.RewardPool), MinimumRarity: box.MinimumRarity,
	})
}

func (s *Service) boxForUser(ctx context.Context, userID, boxID string) (*domain.TeacherGiftBox, error) {
	boxes, err := s.List(ctx, userID)
	if err != nil {
		return nil, err
	}
	var box *domain.TeacherGiftBox
	for index := range boxes {
		if boxes[index].ID.Hex() == boxID {
			box = &boxes[index]
			break
		}
	}
	if box == nil {
		return nil, ErrBoxNotFound
	}
	return box, nil
}

func drawPool(source, rewardPool string) string {
	if rewardPool == "character-box" {
		return rewardPool
	}
	switch source {
	case "achievement":
		return "achievement"
	case "reflection-milestone":
		return "reflection"
	default:
		return "teacher-box"
	}
}
