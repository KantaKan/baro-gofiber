package startupstory

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"time"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Store interface {
	FindStudio(ctx context.Context, ownerID primitive.ObjectID) (*domain.StartupStudio, error)
	InsertStudio(ctx context.Context, studio domain.StartupStudio) error
	FindActiveRun(ctx context.Context, ownerID primitive.ObjectID) (*domain.StartupRun, error)
	InsertRun(ctx context.Context, run *domain.StartupRun) error
	SaveRun(ctx context.Context, run *domain.StartupRun, expectedVersion int) error
	AddDiscoveredCombo(ctx context.Context, ownerID primitive.ObjectID, key string) error
	SettleRun(ctx context.Context, ownerID primitive.ObjectID, entry domain.StartupHallEntry, fameGain int, founders, items []string, skin string) error
	CountRankedRuns(ctx context.Context, ownerID primitive.ObjectID, weekKey string) (int, error)
	LeaderboardWeekly(ctx context.Context, cohort int, weekKey string, limit int) ([]domain.StartupLeaderboardEntry, error)
	LeaderboardFame(ctx context.Context, cohort int, limit int) ([]domain.StartupLeaderboardEntry, error)
	ListGenmates(ctx context.Context, cohort int, excludeID primitive.ObjectID) ([]domain.StartupGenmate, error)
	SetStartupOptOut(ctx context.Context, userID primitive.ObjectID, optOut bool) error
	FindStartupOptOut(ctx context.Context, userID primitive.ObjectID) (bool, error)
}

type Player struct {
	ID     string
	Cohort int
	Role   string
}

type Overview struct {
	Studio     *domain.StartupStudio `json:"studio"`
	Run        *domain.StartupRun    `json:"run"`
	RankedAttemptsLeft int           `json:"ranked_attempts_left"`
	WeekKey    string                `json:"week_key"`
	ServerTime time.Time             `json:"server_time"`
	Types      []string              `json:"types"`
	Themes     []string              `json:"themes"`
	Items      []StartupItem         `json:"items"`
	Unlocks    []UnlockInfo          `json:"unlocks"`
	Roles              []RoleInfo            `json:"roles"`
	OptOut     bool                  `json:"opt_out"`
}

type Service struct {
	store Store
	now   func() time.Time
}

func NewService(store Store) *Service {
	return &Service{store: store, now: func() time.Time { return time.Now().UTC() }}
}

func ownerID(player Player) (primitive.ObjectID, error) {
	id, err := primitive.ObjectIDFromHex(player.ID)
	if err != nil {
		return primitive.NilObjectID, errors.New("invalid account ID")
	}
	return id, nil
}

func (s *Service) ensureStudio(ctx context.Context, owner primitive.ObjectID, cohort int) (*domain.StartupStudio, error) {
	studio, err := s.store.FindStudio(ctx, owner)
	if err != nil {
		return nil, err
	}
	if studio == nil {
		now := s.now()
		fresh := domain.StartupStudio{ID: primitive.NewObjectID(), OwnerID: owner, Cohort: cohort, CreatedAt: now, UpdatedAt: now}
		if err := s.store.InsertStudio(ctx, fresh); err != nil {
			return nil, err
		}
		if studio, err = s.store.FindStudio(ctx, owner); err != nil {
			return nil, err
		}
	}
	return studio, nil
}

func (s *Service) Overview(ctx context.Context, player Player) (*Overview, error) {
	owner, err := ownerID(player)
	if err != nil {
		return nil, err
	}
	studio, err := s.ensureStudio(ctx, owner, player.Cohort)
	if err != nil {
		return nil, err
	}
	run, err := s.store.FindActiveRun(ctx, owner)
	if err != nil {
		return nil, err
	}
	weekKey := weekKeyFor(s.now())
	used, err := s.store.CountRankedRuns(ctx, owner, weekKey)
	if err != nil {
		return nil, err
	}
	left := MaxRankedAttempts - used
	if left < 0 {
		left = 0
	}
	optOut, err := s.store.FindStartupOptOut(ctx, owner)
	if err != nil {
		return nil, err
	}
	typeNames := make([]string, len(productTypes))
	for i, t := range productTypes {
		typeNames[i] = t.Name
	}
	items := append(append([]StartupItem{}, itemCatalog...), unlockableItems...)
	return &Overview{Studio: studio, Run: run, RankedAttemptsLeft: left, WeekKey: weekKey, ServerTime: s.now(), Types: typeNames, Themes: themes, Items: items, Unlocks: unlockTable, Roles: roles, OptOut: optOut}, nil
}

func (s *Service) StartRun(ctx context.Context, player Player, mode string) (*domain.StartupRun, error) {
	owner, err := ownerID(player)
	if err != nil {
		return nil, err
	}
	if mode != domain.StartupModeFree && mode != domain.StartupModeRanked {
		return nil, domain.ErrStartupInvalidChoice
	}
	weekKey := ""
	var seedValue uint64
	if mode == domain.StartupModeRanked {
		weekKey = weekKeyFor(s.now())
		used, err := s.store.CountRankedRuns(ctx, owner, weekKey)
		if err != nil {
			return nil, err
		}
		if used >= MaxRankedAttempts {
			return nil, domain.ErrStartupNoAttempts
		}
		seedValue = WeekSeed(weekKey)
	} else {
		var seed [8]byte
		if _, err := rand.Read(seed[:]); err != nil {
			return nil, err
		}
		seedValue = binary.LittleEndian.Uint64(seed[:])
	}
	studio, err := s.ensureStudio(ctx, owner, player.Cohort)
	if err != nil {
		return nil, err
	}
	run := newRunWithPool(owner, player.Cohort, player.Role, mode, seedValue, s.now(), founderPool(studio.UnlockedFounders), studio.UnlockedItems)
	run.WeekKey = weekKey
	genmates, err := s.store.ListGenmates(ctx, player.Cohort, owner)
	if err != nil {
		return nil, err
	}
	run.GenmatePool = genmates
	run.ID = primitive.NewObjectID()
	if err := s.store.InsertRun(ctx, run); err != nil {
		return nil, err
	}
	return run, nil
}

func (s *Service) PickFounder(ctx context.Context, player Player, index int) (*domain.StartupRun, error) {
	return s.mutate(ctx, player, func(run *domain.StartupRun, _ time.Time) error {
		return PickFounder(run, index)
	})
}

func (s *Service) StartProject(ctx context.Context, player Player, typeName, theme string, staffIDs []string) (*domain.StartupRun, error) {
	return s.mutate(ctx, player, func(run *domain.StartupRun, now time.Time) error {
		return StartProject(run, typeName, theme, staffIDs, now)
	})
}

func hallEntryFor(run *domain.StartupRun) domain.StartupHallEntry {
	endedAt := run.UpdatedAt
	if run.EndedAt != nil {
		endedAt = *run.EndedAt
	}
	return domain.StartupHallEntry{
		RunID: run.ID, Mode: run.Mode, WeekKey: run.WeekKey,
		Score: run.Score, Outcome: run.Outcome, Founder: run.Founder, EndedAt: endedAt,
	}
}

func (s *Service) settleIfEnded(ctx context.Context, owner primitive.ObjectID, run *domain.StartupRun) error {
	if run.Status != domain.StartupStatusEnded {
		return nil
	}
	studio, err := s.ensureStudio(ctx, owner, run.Cohort)
	if err != nil {
		return err
	}
	gain := fameGainFor(run.Score)
	founders, items, skin := unlocksFor(studio.Fame + gain)
	return s.store.SettleRun(ctx, owner, hallEntryFor(run), gain, founders, items, skin)
}

func (s *Service) Ship(ctx context.Context, player Player) (*domain.StartupRun, error) {
	run, err := s.mutate(ctx, player, Ship)
	if err != nil {
		return nil, err
	}
	owner, err := ownerID(player)
	if err != nil {
		return nil, err
	}
	if run.LastResult != nil {
		if err := s.store.AddDiscoveredCombo(ctx, owner, ComboKey(run.LastResult.Type, run.LastResult.Theme)); err != nil {
			return nil, err
		}
	}
	if err := s.settleIfEnded(ctx, owner, run); err != nil {
		return nil, err
	}
	return run, nil
}

func (s *Service) Hire(ctx context.Context, player Player, candidateID string) (*domain.StartupRun, error) {
	return s.mutate(ctx, player, func(run *domain.StartupRun, _ time.Time) error {
		return Hire(run, candidateID)
	})
}

func (s *Service) Dismiss(ctx context.Context, player Player, staffID string) (*domain.StartupRun, error) {
	return s.mutate(ctx, player, func(run *domain.StartupRun, _ time.Time) error {
		return Dismiss(run, staffID)
	})
}

func (s *Service) PickItem(ctx context.Context, player Player, index int) (*domain.StartupRun, error) {
	return s.mutate(ctx, player, func(run *domain.StartupRun, _ time.Time) error {
		return PickItem(run, index)
	})
}

func (s *Service) OptOut(ctx context.Context, player Player, optOut bool) (bool, error) {
	owner, err := ownerID(player)
	if err != nil {
		return false, err
	}
	if err := s.store.SetStartupOptOut(ctx, owner, optOut); err != nil {
		return false, err
	}
	return optOut, nil
}

func (s *Service) Leaderboard(ctx context.Context, player Player, tab string) ([]domain.StartupLeaderboardEntry, error) {
	if tab == "" {
		tab = domain.StartupBoardWeekly
	}
	switch tab {
	case domain.StartupBoardFame:
		return s.store.LeaderboardFame(ctx, player.Cohort, domain.StartupLeaderboardLimit)
	case domain.StartupBoardWeekly:
		return s.store.LeaderboardWeekly(ctx, player.Cohort, weekKeyFor(s.now()), domain.StartupLeaderboardLimit)
	default:
		return nil, domain.ErrStartupInvalidChoice
	}
}

func (s *Service) Abandon(ctx context.Context, player Player) (*domain.StartupRun, error) {
	run, err := s.mutate(ctx, player, func(run *domain.StartupRun, now time.Time) error {
		return Abandon(run, now)
	})
	if err != nil {
		return nil, err
	}
	owner, err := ownerID(player)
	if err != nil {
		return nil, err
	}
	if err := s.settleIfEnded(ctx, owner, run); err != nil {
		return nil, err
	}
	return run, nil
}

func (s *Service) mutate(ctx context.Context, player Player, apply func(*domain.StartupRun, time.Time) error) (*domain.StartupRun, error) {
	owner, err := ownerID(player)
	if err != nil {
		return nil, err
	}
	run, err := s.store.FindActiveRun(ctx, owner)
	if err != nil {
		return nil, err
	}
	if run == nil {
		return nil, domain.ErrStartupNoActiveRun
	}
	now := s.now()
	expected := run.Version
	if err := apply(run, now); err != nil {
		return nil, err
	}
	run.Version = expected + 1
	run.UpdatedAt = now
	if err := s.store.SaveRun(ctx, run, expected); err != nil {
		return nil, err
	}
	return run, nil
}
