package showcase

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

var ErrAccountNotFound = errors.New("account not found")
var ErrCharacterNotOwned = errors.New("character is not owned by this account")
var ErrCohortForbidden = errors.New("this cohort is not available to your account")
var ErrEntryNotFound = errors.New("showcase entry not found")
var ErrAdminRequired = errors.New("admin access required")

var ErrInvalidMood = errors.New("choose one of the lawn moods")
var ErrInvalidEmote = errors.New("choose one of the lawn emotes")
var ErrInvalidEmoteTarget = errors.New("choose a visible friend from your cohort to visit")

var reactionEmojis = map[string]bool{"❤️": true, "✨": true, "😂": true, "🙌": true}

var lawnMoods = map[string]bool{"greeting": true, "relaxing": true, "meal": true, "playful": true, "quiet": true, "surprise": true}

const MoodDuration = 24 * time.Hour

var lawnEmotes = map[string]bool{"wave": true, "dance": true, "jump": true, "heart": true, "nap": true, "visit": true}

const EmoteDuration = 30 * time.Minute

type Viewer struct {
	Cohort int
	Role   string
}

type Entry struct {
	OwnerID   string               `json:"owner_id"`
	Name      string               `json:"name"`
	Cohort    int                  `json:"cohort"`
	Team      string               `json:"team"`
	Character domain.BaroCharacter `json:"character"`
	Prop      string               `json:"prop,omitempty"`
	Message   string               `json:"message"`
	UpdatedAt time.Time            `json:"updated_at"`
	Hidden    bool                 `json:"hidden,omitempty"`
	Reactions []ReactionSummary    `json:"reactions"`
	Mood        string               `json:"mood,omitempty"`
	MoodUntil   *time.Time           `json:"mood_until,omitempty"`
	Emote       string               `json:"emote,omitempty"`
	EmoteTarget string               `json:"emote_target,omitempty"`
	EmoteUntil  *time.Time           `json:"emote_until,omitempty"`
}

type EmoteState struct {
	Emote  string     `json:"emote"`
	Target string     `json:"target,omitempty"`
	Until  *time.Time `json:"until,omitempty"`
}

type MoodState struct {
	Mood  string     `json:"mood"`
	Until *time.Time `json:"until,omitempty"`
}

type ReactionSummary struct {
	Emoji   string `json:"emoji"`
	Count   int    `json:"count"`
	Reacted bool   `json:"reacted"`
}

type Target struct {
	Cohort int
	Hidden bool
}

type Store interface {
	Viewer(ctx context.Context, id primitive.ObjectID) (*Viewer, error)
	FindOwned(ctx context.Context, ownerID, characterID primitive.ObjectID) (*domain.BaroCharacter, error)
	Save(ctx context.Context, ownerID primitive.ObjectID, characterID *primitive.ObjectID, message string, at time.Time) error
	List(ctx context.Context, cohort int, team string, includeHidden bool, viewerID primitive.ObjectID) ([]Entry, error)
	Own(ctx context.Context, ownerID primitive.ObjectID) (*Entry, error)
	ReactionTarget(ctx context.Context, ownerID primitive.ObjectID) (*Target, error)
	ToggleReaction(ctx context.Context, ownerID, actorID primitive.ObjectID, emoji string) (bool, error)
	Moderate(ctx context.Context, ownerID, adminID primitive.ObjectID, hidden bool, reason string, at time.Time) error
	SetMood(ctx context.Context, ownerID primitive.ObjectID, mood string, until time.Time) error
	SetEmote(ctx context.Context, ownerID primitive.ObjectID, emote, target string, until time.Time) error
}

type Service struct {
	store Store
	now   func() time.Time
}

func NewService(store Store) *Service { return &Service{store: store, now: time.Now} }

func (s *Service) withActiveMoods(entries []Entry) []Entry {
	now := s.now()
	for index := range entries {
		if entries[index].MoodUntil == nil || !entries[index].MoodUntil.After(now) {
			entries[index].Mood = ""
			entries[index].MoodUntil = nil
		}
		if entries[index].EmoteUntil == nil || !entries[index].EmoteUntil.After(now) {
			entries[index].Emote = ""
			entries[index].EmoteTarget = ""
			entries[index].EmoteUntil = nil
		}
	}
	return entries
}

func (s *Service) SetEmote(ctx context.Context, ownerHex, emote, targetHex string) (*EmoteState, error) {
	ownerID, err := primitive.ObjectIDFromHex(ownerHex)
	if err != nil {
		return nil, ErrAccountNotFound
	}
	emote = strings.TrimSpace(emote)
	if emote != "" && !lawnEmotes[emote] {
		return nil, ErrInvalidEmote
	}
	if emote != "visit" {
		targetHex = ""
	}
	if emote == "visit" {
		targetID, err := primitive.ObjectIDFromHex(targetHex)
		if err != nil || targetID == ownerID {
			return nil, ErrInvalidEmoteTarget
		}
		viewer, err := s.store.Viewer(ctx, ownerID)
		if err != nil {
			return nil, err
		}
		target, err := s.store.ReactionTarget(ctx, targetID)
		if errors.Is(err, ErrEntryNotFound) || (err == nil && (target == nil || viewer == nil || target.Hidden || (viewer.Role != "admin" && target.Cohort != viewer.Cohort))) {
			return nil, ErrInvalidEmoteTarget
		}
		if err != nil {
			return nil, err
		}
	}
	until := s.now().UTC().Add(EmoteDuration)
	if err := s.store.SetEmote(ctx, ownerID, emote, targetHex, until); err != nil {
		return nil, err
	}
	if emote == "" {
		return &EmoteState{}, nil
	}
	return &EmoteState{Emote: emote, Target: targetHex, Until: &until}, nil
}

func (s *Service) SetMood(ctx context.Context, ownerHex, mood string) (*MoodState, error) {
	ownerID, err := primitive.ObjectIDFromHex(ownerHex)
	if err != nil {
		return nil, ErrAccountNotFound
	}
	mood = strings.TrimSpace(mood)
	if mood != "" && !lawnMoods[mood] {
		return nil, ErrInvalidMood
	}
	until := s.now().UTC().Add(MoodDuration)
	if err := s.store.SetMood(ctx, ownerID, mood, until); err != nil {
		return nil, err
	}
	if mood == "" {
		return &MoodState{}, nil
	}
	return &MoodState{Mood: mood, Until: &until}, nil
}

func (s *Service) Save(ctx context.Context, ownerHex, characterHex, message string) error {
	ownerID, err := primitive.ObjectIDFromHex(ownerHex)
	if err != nil {
		return ErrAccountNotFound
	}
	message = strings.TrimSpace(message)
	if utf8.RuneCountInString(message) > 160 {
		return errors.New("message must be 160 characters or fewer")
	}
	if characterHex == "" {
		return errors.New("choose a character to pin")
	}
	characterID, err := primitive.ObjectIDFromHex(characterHex)
	if err != nil {
		return ErrCharacterNotOwned
	}
	viewer, err := s.store.Viewer(ctx, ownerID)
	if err != nil {
		return err
	}
	if viewer == nil {
		return ErrAccountNotFound
	}
	owned, err := s.store.FindOwned(ctx, ownerID, characterID)
	if err != nil {
		return err
	}
	if owned == nil {
		return ErrCharacterNotOwned
	}
	return s.store.Save(ctx, ownerID, &characterID, message, time.Now().UTC())
}

func (s *Service) Remove(ctx context.Context, ownerHex string) error {
	ownerID, err := primitive.ObjectIDFromHex(ownerHex)
	if err != nil {
		return ErrAccountNotFound
	}
	return s.store.Save(ctx, ownerID, nil, "", time.Now().UTC())
}

func (s *Service) Mine(ctx context.Context, ownerHex string) (*Entry, error) {
	ownerID, err := primitive.ObjectIDFromHex(ownerHex)
	if err != nil {
		return nil, ErrAccountNotFound
	}
	viewer, err := s.store.Viewer(ctx, ownerID)
	if err != nil {
		return nil, err
	}
	if viewer == nil {
		return nil, ErrAccountNotFound
	}
	entry, err := s.store.Own(ctx, ownerID)
	if err != nil || entry == nil {
		return entry, err
	}
	return &s.withActiveMoods([]Entry{*entry})[0], nil
}

func (s *Service) List(ctx context.Context, viewerHex string, isAdmin bool, cohort int, team string, includeHidden bool) ([]Entry, error) {
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
	if !isAdmin || viewer.Role != "admin" {
		if includeHidden {
			return nil, ErrAdminRequired
		}
		if viewer.Cohort <= 0 || (cohort != 0 && cohort != viewer.Cohort) {
			return nil, ErrCohortForbidden
		}
		cohort = viewer.Cohort
	}
	team = strings.TrimSpace(team)
	if cohort < 0 || utf8.RuneCountInString(team) > 100 {
		return nil, errors.New("invalid cohort or team filter")
	}
	entries, err := s.store.List(ctx, cohort, team, includeHidden, viewerID)
	if err != nil {
		return nil, err
	}
	return s.withActiveMoods(entries), nil
}

func (s *Service) React(ctx context.Context, actorHex, ownerHex, emoji string) (bool, error) {
	if !reactionEmojis[emoji] {
		return false, errors.New("unsupported reaction")
	}
	actorID, err := primitive.ObjectIDFromHex(actorHex)
	if err != nil {
		return false, ErrAccountNotFound
	}
	ownerID, err := primitive.ObjectIDFromHex(ownerHex)
	if err != nil {
		return false, ErrEntryNotFound
	}
	viewer, err := s.store.Viewer(ctx, actorID)
	if err != nil {
		return false, err
	}
	if viewer == nil {
		return false, ErrAccountNotFound
	}
	target, err := s.store.ReactionTarget(ctx, ownerID)
	if err != nil {
		return false, err
	}
	if target == nil || target.Hidden || (viewer.Role != "admin" && viewer.Cohort != target.Cohort) {
		return false, ErrEntryNotFound
	}
	return s.store.ToggleReaction(ctx, ownerID, actorID, emoji)
}

func (s *Service) Moderate(ctx context.Context, adminHex, ownerHex string, hidden bool, reason string) error {
	adminID, err := primitive.ObjectIDFromHex(adminHex)
	if err != nil {
		return ErrAdminRequired
	}
	viewer, err := s.store.Viewer(ctx, adminID)
	if err != nil {
		return err
	}
	if viewer == nil || viewer.Role != "admin" {
		return ErrAdminRequired
	}
	ownerID, err := primitive.ObjectIDFromHex(ownerHex)
	if err != nil {
		return ErrEntryNotFound
	}
	reason = strings.TrimSpace(reason)
	if utf8.RuneCountInString(reason) > 200 {
		return errors.New("reason must be 200 characters or fewer")
	}
	return s.store.Moderate(ctx, ownerID, adminID, hidden, reason, time.Now().UTC())
}
