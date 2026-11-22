package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"gofiber-baro/internal/service/slack"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type stubSlack struct {
	connectErr error
	connected  bool
	pinged     []primitive.ObjectID
	quiet      bool
	tested     string
}

func (s *stubSlack) Status() (slack.Status, error) {
	return slack.Status{Connected: s.connected, Team: "Generation", Bot: "baro"}, nil
}
func (s *stubSlack) Connect(token string) error {
	if s.connectErr != nil {
		return s.connectErr
	}
	s.connected = true
	return nil
}
func (s *stubSlack) Disconnect() error                 { s.connected = false; return nil }
func (s *stubSlack) SaveSettings(slack.Settings) error { return nil }
func (s *stubSlack) Channels() ([]slack.Channel, error) {
	return []slack.Channel{{ID: "C1", Name: "alerts"}}, nil
}
func (s *stubSlack) Test(which string) error { s.tested = which; return nil }
func (s *stubSlack) Match(int) (slack.MatchResult, error) {
	return slack.MatchResult{Missing: []slack.Learner{}}, nil
}
func (s *stubSlack) Log(int) ([]slack.LogEntry, error) { return []slack.LogEntry{}, nil }
func (s *stubSlack) Ping(ids []primitive.ObjectID) (slack.PingResult, error) {
	s.pinged = ids
	return slack.PingResult{Sent: len(ids), NoSlack: []string{}, Quiet: s.quiet}, nil
}

func slackApp(s *stubSlack) *fiber.App {
	app := fiber.New()
	h := NewSlackHandler(s)
	app.Get("/admin/settings/slack", h.Status)
	app.Post("/admin/settings/slack/connect", h.Connect)
	app.Delete("/admin/settings/slack", h.Disconnect)
	app.Put("/admin/settings/slack/settings", h.SaveSettings)
	app.Get("/admin/settings/slack/channels", h.Channels)
	app.Post("/admin/settings/slack/test", h.Test)
	app.Post("/admin/settings/slack/match", h.Match)
	app.Get("/admin/settings/slack/log", h.Log)
	app.Post("/admin/nudges/ping", h.Ping)
	return app
}

func slackCall(app *fiber.App, method, url, body string) (int, string) {
	req := httptest.NewRequest(method, url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res, _ := app.Test(req)
	b, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(b)
}

func TestSlackHandler(t *testing.T) {
	s := &stubSlack{}
	app := slackApp(s)
	code, body := slackCall(app, "GET", "/admin/settings/slack", "")
	var env struct{ Data map[string]any }
	_ = json.Unmarshal([]byte(body), &env)
	if code != 200 || env.Data["connected"] != false || !strings.Contains(body, "chat:write") {
		t.Errorf("status %d %s", code, body)
	}
	s.connectErr = errors.New("Slack didn't accept that token (slack: invalid_auth)")
	if code, body := slackCall(app, "POST", "/admin/settings/slack/connect", `{"token":"xoxb-secret-token"}`); code != 400 || strings.Contains(body, "xoxb-secret-token") {
		t.Errorf("bad token %d %s", code, body)
	}
	s.connectErr = nil
	if code, body := slackCall(app, "POST", "/admin/settings/slack/connect", `{"token":"xoxb-secret-token"}`); code != 200 || strings.Contains(body, "xoxb-secret-token") {
		t.Errorf("connect %d %s", code, body)
	}
	if code, _ := slackCall(app, "POST", "/admin/settings/slack/test", `{"channel":"alerts"}`); code != 200 || s.tested != "alerts" {
		t.Errorf("test %d %s", code, s.tested)
	}
	if code, _ := slackCall(app, "POST", "/admin/settings/slack/test", `{"channel":"nope"}`); code != 400 {
		t.Errorf("unknown channel %d", code)
	}
	if code, _ := slackCall(app, "POST", "/admin/settings/slack/match", `{"cohort":13}`); code != 200 {
		t.Errorf("match %d", code)
	}
}

func TestPingHandler(t *testing.T) {
	s := &stubSlack{}
	app := slackApp(s)
	id := primitive.NewObjectID().Hex()
	if code, _ := slackCall(app, "POST", "/admin/nudges/ping", `{"user_ids":["`+id+`"]}`); code != 200 || len(s.pinged) != 1 || s.pinged[0].Hex() != id {
		t.Errorf("ping %d %v", code, s.pinged)
	}
	if code, _ := slackCall(app, "POST", "/admin/nudges/ping", `{"user_ids":[]}`); code != 400 {
		t.Errorf("empty list %d", code)
	}
	if code, _ := slackCall(app, "POST", "/admin/nudges/ping", `{"user_ids":["not-an-id"]}`); code != 400 {
		t.Errorf("bad id %d", code)
	}
	tooMany := make([]string, 201)
	for i := range tooMany {
		tooMany[i] = primitive.NewObjectID().Hex()
	}
	b, _ := json.Marshal(map[string]any{"user_ids": tooMany})
	if code, _ := slackCall(app, "POST", "/admin/nudges/ping", string(b)); code != 400 {
		t.Errorf("too many %d", code)
	}
}
