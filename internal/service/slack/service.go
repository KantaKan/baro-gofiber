package slack

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// PingCooldown: a learner who was pinged is skipped for this long.
const PingCooldown = 30 * time.Minute

// maxFailsInARow: stop a Ping after this many consecutive Slack failures (an outage or bad token).
const maxFailsInARow = 3

const (
	KindNewLeave = "new_leave"
	KindAnnounce = "announcement"
	KindPing     = "ping"
	KindTest     = "test"
)

var (
	ErrNotConnected = errors.New("Slack isn't connected yet")
	ErrNoChannel    = errors.New("pick a channel for this first")
)

// Config is the single stored Slack setup. TokenEnc is the sealed bot token; it is never sent to the browser.
type Config struct {
	TokenEnc      string `bson:"token_enc" json:"-"`
	Team          string `bson:"team" json:"team"`
	Bot           string `bson:"bot" json:"bot"`
	Alerts        string `bson:"alerts_channel" json:"alerts"`
	Report        string `bson:"report_channel" json:"report"`
	Announce      string `bson:"announce_channel" json:"announce"`
	AlertNewLeave bool   `bson:"alert_new_leave" json:"alert_new_leave"`
	QuietFrom     string `bson:"quiet_from" json:"quiet_from"`
	QuietTo       string `bson:"quiet_to" json:"quiet_to"`
}

type LogEntry struct {
	At     time.Time `bson:"at" json:"at"`
	Kind   string    `bson:"kind" json:"kind"`
	Target string    `bson:"target" json:"target"`
	OK     bool      `bson:"ok" json:"ok"`
	Error  string    `bson:"error,omitempty" json:"error,omitempty"`
}

type Learner struct {
	ID      primitive.ObjectID `json:"id"`
	Name    string             `json:"name"`
	Email   string             `json:"email"`
	SlackID string             `json:"-"`
}

type Store interface {
	Config() (*Config, error)
	SaveConfig(c *Config) error
	DeleteConfig() error
	AddLog(e LogEntry) error
	Log(limit int) ([]LogEntry, error)
	Learners(cohort int) ([]Learner, error)
	LearnersByIDs(ids []primitive.ObjectID) ([]Learner, error)
	SetSlackID(id primitive.ObjectID, slackID string) error
}

type Service struct {
	store  Store
	client *Client
	key    []byte
	now    func() time.Time
	appURL string

	mu       sync.Mutex
	lastPing map[string]time.Time // learner id -> last Ping, so nobody gets nagged repeatedly
}

func NewService(st Store, c *Client, key []byte) *Service {
	app := strings.TrimSpace(os.Getenv("APP_URL"))
	if app == "" {
		app = strings.TrimSpace(strings.Split(os.Getenv("CORS_ALLOWED_ORIGINS"), ",")[0])
	}
	return &Service{store: st, client: c, key: key, now: time.Now, appURL: strings.TrimRight(app, "/"), lastPing: map[string]time.Time{}}
}

// Status is what the settings page may see: no token, ever.
type Status struct {
	Connected     bool   `json:"connected"`
	Team          string `json:"team"`
	Bot           string `json:"bot"`
	Alerts        string `json:"alerts"`
	Report        string `json:"report"`
	Announce      string `json:"announce"`
	AlertNewLeave bool   `json:"alert_new_leave"`
	QuietFrom     string `json:"quiet_from"`
	QuietTo       string `json:"quiet_to"`
}

func (s *Service) Status() (Status, error) {
	c, err := s.store.Config()
	if err != nil || c == nil {
		return Status{}, err
	}
	return Status{true, c.Team, c.Bot, c.Alerts, c.Report, c.Announce, c.AlertNewLeave, c.QuietFrom, c.QuietTo}, nil
}

// Connected tells other code whether posting is possible.
func (s *Service) Connected() bool {
	st, err := s.Status()
	return err == nil && st.Connected
}

func (s *Service) token() (string, *Config, error) {
	c, err := s.store.Config()
	if err != nil {
		return "", nil, err
	}
	if c == nil {
		return "", nil, ErrNotConnected
	}
	t, err := Open(s.key, c.TokenEnc)
	return t, c, err
}

// Connect verifies the token with Slack and stores it sealed. Nothing is stored if Slack says no.
func (s *Service) Connect(token string) error {
	token = strings.TrimSpace(token)
	if !strings.HasPrefix(token, "xoxb-") {
		return errors.New("that doesn't look like a bot token (it starts with xoxb-)")
	}
	team, bot, err := s.client.AuthTest(token)
	if err != nil {
		return fmt.Errorf("Slack didn't accept that token (%v)", err)
	}
	sealed, err := Seal(s.key, token)
	if err != nil {
		return err
	}
	cfg, _ := s.store.Config()
	if cfg == nil {
		cfg = &Config{}
	}
	cfg.TokenEnc, cfg.Team, cfg.Bot = sealed, team, bot
	return s.store.SaveConfig(cfg)
}

func (s *Service) Disconnect() error { return s.store.DeleteConfig() }

type Settings struct {
	Alerts, Report, Announce string
	AlertNewLeave            bool
	QuietFrom, QuietTo       string
}

func validClock(v string) bool { return v == "" || minutes(v, -1) >= 0 }

func (s *Service) SaveSettings(in Settings) error {
	c, err := s.store.Config()
	if err != nil {
		return err
	}
	if c == nil {
		return ErrNotConnected
	}
	if !validClock(in.QuietFrom) || !validClock(in.QuietTo) {
		return errors.New("quiet hours must look like 21:00")
	}
	c.Alerts, c.Report, c.Announce = in.Alerts, in.Report, in.Announce
	c.AlertNewLeave, c.QuietFrom, c.QuietTo = in.AlertNewLeave, in.QuietFrom, in.QuietTo
	return s.store.SaveConfig(c)
}

func (s *Service) Channels() ([]Channel, error) {
	t, _, err := s.token()
	if err != nil {
		return nil, err
	}
	return s.client.Channels(t)
}

func (s *Service) Log(limit int) ([]LogEntry, error) { return s.store.Log(limit) }

func trim(msg string) string {
	if len(msg) > 200 {
		return msg[:200]
	}
	return msg
}

func (s *Service) record(kind, target string, err error) {
	e := LogEntry{At: s.now(), Kind: kind, Target: target, OK: err == nil}
	if err != nil {
		e.Error = trim(err.Error())
	}
	_ = s.store.AddLog(e)
}

func (s *Service) post(kind, channel, text string) error {
	t, _, err := s.token()
	if err != nil {
		return err
	}
	err = s.client.PostMessage(t, channel, text)
	s.record(kind, channel, err)
	return err
}

// Test posts a hello to one of the mapped channels ("alerts", "report", "announce").
func (s *Service) Test(which string) error {
	_, c, err := s.token()
	if err != nil {
		return err
	}
	ch := map[string]string{"alerts": c.Alerts, "report": c.Report, "announce": c.Announce}[which]
	if ch == "" {
		return ErrNoChannel
	}
	return s.post(KindTest, ch, "👋 Baro test message. If you can see this, Slack is connected.")
}

type MatchResult struct {
	Matched int       `json:"matched"`
	Missing []Learner `json:"missing"`
	Note    string    `json:"note,omitempty"`
}

// Match finds each active learner's Slack user by email and remembers the id.
func (s *Service) Match(cohort int) (MatchResult, error) {
	t, _, err := s.token()
	if err != nil {
		return MatchResult{}, err
	}
	ls, err := s.store.Learners(cohort)
	if err != nil {
		return MatchResult{}, err
	}
	res := MatchResult{Missing: []Learner{}}
	for _, l := range ls {
		if l.Email == "" {
			res.Missing = append(res.Missing, l)
			continue
		}
		id, found, err := s.client.LookupByEmail(t, l.Email)
		if err != nil {
			res.Note = "Stopped early: " + err.Error() + ". Run it again in a minute."
			return res, nil
		}
		if !found {
			res.Missing = append(res.Missing, l)
			if l.SlackID != "" { // they left the workspace or changed email: don't keep DMing an old id
				if err := s.store.SetSlackID(l.ID, ""); err != nil {
					return res, err
				}
			}
			continue
		}
		if id != l.SlackID {
			if err := s.store.SetSlackID(l.ID, id); err != nil {
				return res, err
			}
		}
		res.Matched++
	}
	return res, nil
}

type PingResult struct {
	Sent    int      `json:"sent"`
	NoSlack []string `json:"no_slack"`
	Failed  int      `json:"failed"`
	Recent  int      `json:"recent"` // skipped: pinged within the cooldown
	Quiet   bool     `json:"quiet"`
}

// Ping DMs a check-in reminder to learners. Inside quiet hours nothing is sent.
func (s *Service) Ping(ids []primitive.ObjectID) (PingResult, error) {
	_, c, err := s.token()
	if err != nil {
		return PingResult{}, err
	}
	res := PingResult{NoSlack: []string{}}
	if InQuiet(s.now(), c.QuietFrom, c.QuietTo) {
		res.Quiet = true
		return res, nil
	}
	ls, err := s.store.LearnersByIDs(ids)
	if err != nil {
		return res, err
	}
	text := "Good morning! Don't forget to check in today."
	if s.appURL != "" {
		text += " " + s.appURL
	}
	fails := 0
	for i, l := range ls {
		if l.SlackID == "" {
			res.NoSlack = append(res.NoSlack, l.ID.Hex())
			continue
		}
		if s.recentlyPinged(l.ID.Hex()) {
			res.Recent++
			continue
		}
		if err := s.post(KindPing, l.SlackID, text); err != nil {
			res.Failed++
			if fails++; fails >= maxFailsInARow {
				for _, rest := range ls[i+1:] {
					if rest.SlackID != "" && !s.recentlyPinged(rest.ID.Hex()) {
						res.Failed++
					}
				}
				break
			}
			continue
		}
		fails = 0
		s.markPinged(l.ID.Hex())
		res.Sent++
	}
	return res, nil
}

func (s *Service) recentlyPinged(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.lastPing[id]
	return ok && s.now().Sub(t) < PingCooldown
}

func (s *Service) markPinged(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastPing[id] = s.now()
}

// alertNow posts an admin alert if Slack is connected, the channel is set and the toggle for kind is on.
func (s *Service) alertNow(kind, text string) error {
	c, err := s.store.Config()
	if err != nil || c == nil || c.Alerts == "" {
		return nil
	}
	if kind == KindNewLeave && !c.AlertNewLeave {
		return nil
	}
	return s.post(kind, c.Alerts, text)
}

// Alert is fire-and-forget: a Slack problem must never fail the request that triggered it.
func (s *Service) Alert(kind, text string) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Println("slack alert panic recovered")
			}
		}()
		_ = s.alertNow(kind, text)
	}()
}

// PostAnnouncement posts to the announcements channel (no-op when Slack isn't set up).
func (s *Service) PostAnnouncement(title, message string) error {
	c, err := s.store.Config()
	if err != nil || c == nil || c.Announce == "" {
		return nil
	}
	return s.post(KindAnnounce, c.Announce, "*"+title+"*\n"+message)
}

// AnnounceAsync is the fire-and-forget form used by the announcement handler.
func (s *Service) AnnounceAsync(title, message string) {
	go func() {
		defer func() { _ = recover() }()
		_ = s.PostAnnouncement(title, message)
	}()
}

// Manifest is the Slack app manifest (JSON) the Owner pastes when creating the app.
func Manifest() string {
	m := map[string]any{
		"display_information": map[string]any{"name": "Baro", "description": "Baro attendance and check-in helper"},
		"features":            map[string]any{"bot_user": map[string]any{"display_name": "Baro", "always_online": false}},
		"oauth_config": map[string]any{"scopes": map[string]any{"bot": []string{
			"chat:write", "chat:write.public", "im:write", "users:read", "users:read.email", "channels:read", "groups:read",
		}}},
		"settings": map[string]any{"org_deploy_enabled": false, "socket_mode_enabled": false, "token_rotation_enabled": false},
	}
	b, _ := json.MarshalIndent(m, "", "  ")
	return string(b)
}
