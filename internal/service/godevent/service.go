package godevent

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

var ErrAccountNotFound = errors.New("account not found")
var ErrAdminRequired = errors.New("admin access required")
var ErrAudienceNotFound = errors.New("cohort not found")

var presets = map[string]bool{"star_rain": true, "god_entrance": true, "character_parade": true}

type Viewer struct {
	Role   string
	Cohort int
}

type Store interface {
	Viewer(ctx context.Context, id primitive.ObjectID) (*Viewer, error)
	CohortExists(ctx context.Context, cohort int) (bool, error)
	AdminCharacter(ctx context.Context, adminID primitive.ObjectID) (*domain.BaroCharacter, error)
	Create(ctx context.Context, event domain.GodEvent) error
	ListVisible(ctx context.Context, viewer Viewer) ([]domain.GodEvent, error)
}

type EventView struct {
	domain.GodEvent
	Active bool `json:"active"`
}

type Service struct {
	store Store
	now   func() time.Time
}

func NewService(store Store) *Service { return &Service{store: store, now: time.Now} }

func (s *Service) Cast(ctx context.Context, adminHex, preset, caption string, cohort int) (*EventView, error) {
	adminID, err := primitive.ObjectIDFromHex(adminHex)
	if err != nil {
		return nil, ErrAdminRequired
	}
	viewer, err := s.store.Viewer(ctx, adminID)
	if err != nil {
		return nil, err
	}
	if viewer == nil || viewer.Role != "admin" {
		return nil, ErrAdminRequired
	}
	if !presets[preset] {
		return nil, errors.New("invalid GOD event preset")
	}
	caption = strings.TrimSpace(caption)
	if caption == "" || utf8.RuneCountInString(caption) > 160 || strings.ContainsAny(caption, "<>") || strings.IndexFunc(caption, unicode.IsControl) >= 0 {
		return nil, errors.New("caption must be plain text of 1 to 160 characters")
	}
	if cohort < 0 {
		return nil, errors.New("invalid cohort")
	}
	if cohort > 0 {
		exists, err := s.store.CohortExists(ctx, cohort)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, ErrAudienceNotFound
		}
	}
	character, err := s.store.AdminCharacter(ctx, adminID)
	if err != nil {
		return nil, err
	}
	now := s.now().UTC()
	event := domain.GodEvent{ID: primitive.NewObjectID(), Preset: preset, Caption: caption, Cohort: cohort, CastBy: adminID, Character: character, CreatedAt: now, ActiveUntil: now.Add(24 * time.Hour)}
	if err := s.store.Create(ctx, event); err != nil {
		return nil, err
	}
	return &EventView{GodEvent: event, Active: true}, nil
}

func (s *Service) List(ctx context.Context, viewerHex string) ([]EventView, error) {
	viewerID, err := primitive.ObjectIDFromHex(viewerHex)
	if err != nil {
		return nil, ErrAccountNotFound
	}
	viewer, err := s.store.Viewer(ctx, viewerID)
	if err != nil {
		return nil, err
	}
	if viewer == nil {
		return nil, ErrAccountNotFound
	}
	if viewer.Role != "admin" && viewer.Cohort <= 0 {
		return nil, ErrAudienceNotFound
	}
	events, err := s.store.ListVisible(ctx, *viewer)
	if err != nil {
		return nil, err
	}
	now := s.now()
	result := make([]EventView, 0, len(events))
	for _, event := range events {
		result = append(result, EventView{GodEvent: event, Active: now.Before(event.ActiveUntil)})
	}
	return result, nil
}
