package slack

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type memStore struct {
	cfg      *Config
	log      []LogEntry
	learners []Learner
	slackIDs map[primitive.ObjectID]string
}

func (m *memStore) Config() (*Config, error) { return m.cfg, nil }
func (m *memStore) SaveConfig(c *Config) error {
	cp := *c
	m.cfg = &cp
	return nil
}
func (m *memStore) DeleteConfig() error { m.cfg = nil; return nil }
func (m *memStore) AddLog(e LogEntry) error {
	m.log = append(m.log, e)
	return nil
}
func (m *memStore) Log(limit int) ([]LogEntry, error)      { return m.log, nil }
func (m *memStore) Learners(cohort int) ([]Learner, error) { return m.learners, nil }
func (m *memStore) LearnersByIDs(ids []primitive.ObjectID) ([]Learner, error) {
	var out []Learner
	for _, l := range m.learners {
		for _, id := range ids {
			if l.ID == id {
				out = append(out, l)
			}
		}
	}
	return out, nil
}
func (m *memStore) SetSlackID(id primitive.ObjectID, sid string) error {
	m.slackIDs[id] = sid
	for i := range m.learners {
		if m.learners[i].ID == id {
			m.learners[i].SlackID = sid
		}
	}
	return nil
}

func newSvc(t *testing.T, replies map[string]string) (*Service, *memStore, *[]call) {
	t.Helper()
	c, calls := fakeSlack(t, replies)
	st := &memStore{slackIDs: map[primitive.ObjectID]string{}}
	key := make([]byte, 32)
	key[0] = 7
	s := NewService(st, c, key)
	s.now = func() time.Time { return time.Date(2026, 11, 23, 12, 0, 0, 0, time.FixedZone("BKK", 7*3600)) }
	s.appURL = "https://baro.example"
	return s, st, calls
}

const okAuth = `{"ok":true,"team":"Generation","user":"baro"}`

func TestConnectAndDisconnect(t *testing.T) {
	s, st, _ := newSvc(t, map[string]string{"/auth.test": `{"ok":false,"error":"invalid_auth"}`})
	if err := s.Connect("xoxb-bad"); err == nil || st.cfg != nil || strings.Contains(err.Error(), "xoxb-bad") {
		t.Fatalf("bad token: %v cfg=%v", err, st.cfg)
	}
	s, st, _ = newSvc(t, map[string]string{"/auth.test": okAuth})
	if err := s.Connect("xoxb-good"); err != nil {
		t.Fatal(err)
	}
	if st.cfg == nil || st.cfg.TokenEnc == "" || strings.Contains(st.cfg.TokenEnc, "xoxb") || st.cfg.Team != "Generation" {
		t.Fatalf("stored: %+v", st.cfg)
	}
	status, _ := s.Status()
	b, _ := json.Marshal(status)
	if !status.Connected || status.Team != "Generation" || strings.Contains(string(b), "xoxb") || strings.Contains(string(b), st.cfg.TokenEnc) {
		t.Fatalf("status leaks or is wrong: %s", b)
	}
	if err := s.Disconnect(); err != nil || st.cfg != nil {
		t.Fatalf("disconnect: %v", err)
	}
	if st2, _ := s.Status(); st2.Connected {
		t.Error("still connected")
	}
}

func TestSettingsValidation(t *testing.T) {
	s, _, _ := newSvc(t, map[string]string{"/auth.test": okAuth})
	if err := s.SaveSettings(Settings{QuietFrom: "21:00"}); err != ErrNotConnected {
		t.Fatalf("not connected: %v", err)
	}
	_ = s.Connect("xoxb-1")
	if err := s.SaveSettings(Settings{QuietFrom: "9pm", QuietTo: "08:00"}); err == nil {
		t.Error("bad quiet time accepted")
	}
	if err := s.SaveSettings(Settings{Alerts: "C1", Report: "C2", Announce: "C3", AlertNewLeave: true, QuietFrom: "22:00", QuietTo: "07:30"}); err != nil {
		t.Fatal(err)
	}
	st, _ := s.Status()
	if st.Alerts != "C1" || !st.AlertNewLeave || st.QuietTo != "07:30" || !st.Connected {
		t.Fatalf("settings: %+v", st)
	}
}

func TestTestMessageAndLog(t *testing.T) {
	s, st, calls := newSvc(t, map[string]string{"/auth.test": okAuth, "/chat.postMessage": `{"ok":true}`})
	_ = s.Connect("xoxb-1")
	_ = s.SaveSettings(Settings{Alerts: "C1"})
	if err := s.Test("alerts"); err != nil {
		t.Fatal(err)
	}
	last := (*calls)[len(*calls)-1]
	if last.path != "/chat.postMessage" || last.body["channel"] != "C1" || last.auth != "Bearer xoxb-1" {
		t.Fatalf("call: %+v", last)
	}
	if err := s.Test("report"); err != ErrNoChannel {
		t.Errorf("unset channel: %v", err)
	}
	if len(st.log) != 1 || !st.log[0].OK || st.log[0].Kind != "test" {
		t.Fatalf("log: %+v", st.log)
	}
	s2, st2, _ := newSvc(t, map[string]string{"/auth.test": okAuth, "/chat.postMessage": `{"ok":false,"error":"not_in_channel"}`})
	_ = s2.Connect("xoxb-1")
	_ = s2.SaveSettings(Settings{Alerts: "C1"})
	if err := s2.Test("alerts"); err == nil || len(st2.log) != 1 || st2.log[0].OK || !strings.Contains(st2.log[0].Error, "not_in_channel") {
		t.Fatalf("failure not logged: %v %+v", err, st2.log)
	}
}

func TestMatchByEmail(t *testing.T) {
	a, b := primitive.NewObjectID(), primitive.NewObjectID()
	s, st, _ := newSvc(t, map[string]string{"/auth.test": okAuth, "/users.lookupByEmail": `{"ok":false,"error":"users_not_found"}`})
	_ = s.Connect("xoxb-1")
	st.learners = []Learner{{ID: a, Name: "Mint", Email: "mint@x.co"}, {ID: b, Name: "Fah", Email: "fah@x.co", SlackID: "UOLD"}}
	res, err := s.Match(13)
	if err != nil || res.Matched != 0 || len(res.Missing) != 2 || res.Missing[0].Name != "Mint" {
		t.Fatalf("match with nobody found: %v %+v", err, res)
	}
	s2, st2, _ := newSvc(t, map[string]string{"/auth.test": okAuth, "/users.lookupByEmail": `{"ok":true,"user":{"id":"U42"}}`})
	_ = s2.Connect("xoxb-1")
	st2.learners = []Learner{{ID: a, Name: "Mint", Email: "mint@x.co"}}
	res, err = s2.Match(13)
	if err != nil || res.Matched != 1 || len(res.Missing) != 0 || st2.slackIDs[a] != "U42" {
		t.Fatalf("match: %v %+v %v", err, res, st2.slackIDs)
	}
}

func TestPing(t *testing.T) {
	a, b := primitive.NewObjectID(), primitive.NewObjectID()
	s, st, calls := newSvc(t, map[string]string{"/auth.test": okAuth, "/chat.postMessage": `{"ok":true}`})
	_ = s.Connect("xoxb-1")
	st.learners = []Learner{{ID: a, Name: "Mint", SlackID: "U1"}, {ID: b, Name: "Fah"}}
	before := len(*calls)
	res, err := s.Ping([]primitive.ObjectID{a, b})
	if err != nil || res.Sent != 1 || len(res.NoSlack) != 1 || res.NoSlack[0] != b.Hex() || res.Quiet {
		t.Fatalf("ping: %v %+v", err, res)
	}
	dm := (*calls)[before]
	if dm.body["channel"] != "U1" || !strings.Contains(dm.body["text"], "https://baro.example") {
		t.Fatalf("dm: %+v", dm)
	}
	s.now = func() time.Time { return time.Date(2026, 11, 23, 22, 0, 0, 0, time.FixedZone("BKK", 7*3600)) }
	before = len(*calls)
	res, _ = s.Ping([]primitive.ObjectID{a})
	if !res.Quiet || res.Sent != 0 || len(*calls) != before {
		t.Fatalf("quiet hours must send nothing: %+v", res)
	}
}

func TestAlertsAndAnnouncements(t *testing.T) {
	s, st, calls := newSvc(t, map[string]string{"/auth.test": okAuth, "/chat.postMessage": `{"ok":true}`})
	if err := s.alertNow("leave", "x"); err != nil || len(*calls) != 0 {
		t.Fatalf("not connected must be a quiet no-op: %v", err)
	}
	_ = s.Connect("xoxb-1")
	_ = s.SaveSettings(Settings{Alerts: "CA", Announce: "CN", AlertNewLeave: false})
	n := len(*calls)
	_ = s.alertNow(KindNewLeave, "Mint asked for leave")
	if len(*calls) != n {
		t.Error("toggle off must send nothing")
	}
	_ = s.SaveSettings(Settings{Alerts: "CA", Announce: "CN", AlertNewLeave: true})
	_ = s.alertNow(KindNewLeave, "Mint asked for leave")
	if last := (*calls)[len(*calls)-1]; last.body["channel"] != "CA" || last.body["text"] != "Mint asked for leave" {
		t.Fatalf("alert: %+v", last)
	}
	if err := s.PostAnnouncement("Holiday", "No class Wednesday"); err != nil {
		t.Fatal(err)
	}
	if last := (*calls)[len(*calls)-1]; last.body["channel"] != "CN" || !strings.Contains(last.body["text"], "Holiday") || !strings.Contains(last.body["text"], "No class Wednesday") {
		t.Fatalf("announce: %+v", last)
	}
	for _, e := range st.log {
		if strings.Contains(e.Error, "xoxb") || strings.Contains(e.Target, "xoxb") {
			t.Fatalf("token in log: %+v", e)
		}
	}
}

func TestAsyncAlertNeverBlocksOrFails(t *testing.T) {
	s, st, _ := newSvc(t, map[string]string{"/auth.test": okAuth, "/chat.postMessage": `{"ok":false,"error":"is_archived"}`})
	_ = s.Connect("xoxb-1")
	_ = s.SaveSettings(Settings{Alerts: "CA", Announce: "CN", AlertNewLeave: true})
	s.Alert(KindNewLeave, "Mint asked for leave") // returns nothing; Slack is failing
	s.AnnounceAsync("Hi", "there")
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(st.log) >= 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(st.log) < 2 || st.log[0].OK || st.log[1].OK {
		t.Fatalf("failures should be logged, got %+v", st.log)
	}
}

func postCount(calls *[]call) int {
	n := 0
	for _, c := range *calls {
		if c.path == "/chat.postMessage" {
			n++
		}
	}
	return n
}

func TestPingCooldownAndFailureStop(t *testing.T) {
	a, b := primitive.NewObjectID(), primitive.NewObjectID()
	s, st, calls := newSvc(t, map[string]string{"/auth.test": okAuth, "/chat.postMessage": `{"ok":true}`})
	_ = s.Connect("xoxb-1")
	st.learners = []Learner{{ID: a, Name: "Mint", SlackID: "U1"}, {ID: b, Name: "Fah", SlackID: "U2"}}
	res, _ := s.Ping([]primitive.ObjectID{a, b})
	if res.Sent != 2 {
		t.Fatalf("first ping: %+v", res)
	}
	before := postCount(calls)
	res, _ = s.Ping([]primitive.ObjectID{a, b})
	if res.Sent != 0 || res.Recent != 2 || postCount(calls) != before {
		t.Fatalf("a second ping inside the cooldown must send nothing: %+v", res)
	}
	later := s.now().Add(PingCooldown + time.Minute)
	s.now = func() time.Time { return later }
	if res, _ = s.Ping([]primitive.ObjectID{a}); res.Sent != 1 || res.Recent != 0 {
		t.Fatalf("after the cooldown pings work again: %+v", res)
	}

	bad, bst, bcalls := newSvc(t, map[string]string{"/auth.test": okAuth, "/chat.postMessage": `{"ok":false,"error":"ratelimited"}`})
	_ = bad.Connect("xoxb-1")
	for i := 0; i < 6; i++ {
		bst.learners = append(bst.learners, Learner{ID: primitive.NewObjectID(), SlackID: "U" + string(rune('A'+i))})
	}
	ids := []primitive.ObjectID{}
	for _, l := range bst.learners {
		ids = append(ids, l.ID)
	}
	res, _ = bad.Ping(ids)
	if res.Sent != 0 || res.Failed != 6 || postCount(bcalls) != 3 {
		t.Fatalf("must give up after 3 failures in a row (posts=%d): %+v", postCount(bcalls), res)
	}
}

func TestMatchClearsAStaleSlackID(t *testing.T) {
	a := primitive.NewObjectID()
	s, st, _ := newSvc(t, map[string]string{"/auth.test": okAuth, "/users.lookupByEmail": `{"ok":false,"error":"users_not_found"}`})
	_ = s.Connect("xoxb-1")
	st.learners = []Learner{{ID: a, Name: "Mint", Email: "mint@x.co", SlackID: "UOLD"}}
	st.slackIDs[a] = "UOLD"
	if _, err := s.Match(13); err != nil {
		t.Fatal(err)
	}
	if st.slackIDs[a] != "" {
		t.Errorf("a learner who is no longer found must not keep a Slack id, has %q", st.slackIDs[a])
	}
}
