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
}

type Player struct {
	ID     string
	Cohort int
	Role   string
}

type Overview struct {
	Studio     *domain.StartupStudio `json:"studio"`
	Run        *domain.StartupRun    `json:"run"`
	ServerTime time.Time             `json:"server_time"`
	Types      []string              `json:"types"`
	Themes     []string              `json:"themes"`
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

func (s *Service) Overview(ctx context.Context, player Player) (*Overview, error) {
	owner, err := ownerID(player)
	if err != nil {
		return nil, err
	}
	studio, err := s.store.FindStudio(ctx, owner)
	if err != nil {
		return nil, err
	}
	if studio == nil {
		now := s.now()
		fresh := domain.StartupStudio{ID: primitive.NewObjectID(), OwnerID: owner, Cohort: player.Cohort, CreatedAt: now, UpdatedAt: now}
		if err := s.store.InsertStudio(ctx, fresh); err != nil {
			return nil, err
		}
		if studio, err = s.store.FindStudio(ctx, owner); err != nil {
			return nil, err
		}
	}
	run, err := s.store.FindActiveRun(ctx, owner)
	if err != nil {
		return nil, err
	}
	typeNames := make([]string, len(productTypes))
	for i, t := range productTypes {
		typeNames[i] = t.Name
	}
	return &Overview{Studio: studio, Run: run, ServerTime: s.now(), Types: typeNames, Themes: themes}, nil
}

func (s *Service) StartRun(ctx context.Context, player Player, mode string) (*domain.StartupRun, error) {
	owner, err := ownerID(player)
	if err != nil {
		return nil, err
	}
	if mode != domain.StartupModeFree {
		return nil, domain.ErrStartupInvalidChoice
	}
	var seed [8]byte
	if _, err := rand.Read(seed[:]); err != nil {
		return nil, err
	}
	run := NewRun(owner, player.Cohort, player.Role, mode, binary.LittleEndian.Uint64(seed[:]), s.now())
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

func (s *Service) Ship(ctx context.Context, player Player) (*domain.StartupRun, error) {
	return s.mutate(ctx, player, Ship)
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
