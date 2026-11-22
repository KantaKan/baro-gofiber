package slack

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type call struct {
	path, auth string
	body       map[string]string
}

func fakeSlack(t *testing.T, replies map[string]string) (*Client, *[]call) {
	t.Helper()
	var calls []call
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		b := map[string]string{}
		if r.Header.Get("Content-Type") == "application/json" {
			var raw map[string]any
			_ = json.NewDecoder(r.Body).Decode(&raw)
			for k, v := range raw {
				if s, ok := v.(string); ok {
					b[k] = s
				}
			}
		}
		for k := range r.Form {
			b[k] = r.Form.Get(k)
		}
		calls = append(calls, call{r.URL.Path, r.Header.Get("Authorization"), b})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(replies[r.URL.Path]))
	}))
	t.Cleanup(srv.Close)
	return &Client{BaseURL: srv.URL, HTTP: srv.Client()}, &calls
}

func TestAuthTest(t *testing.T) {
	c, calls := fakeSlack(t, map[string]string{"/auth.test": `{"ok":true,"team":"Generation","user":"baro","user_id":"U1"}`})
	team, bot, err := c.AuthTest("xoxb-1")
	if err != nil || team != "Generation" || bot != "baro" || (*calls)[0].auth != "Bearer xoxb-1" {
		t.Fatalf("auth: %v %s %s %+v", err, team, bot, *calls)
	}
	bad, _ := fakeSlack(t, map[string]string{"/auth.test": `{"ok":false,"error":"invalid_auth"}`})
	if _, _, err := bad.AuthTest("nope"); err == nil || !strings.Contains(err.Error(), "invalid_auth") || strings.Contains(err.Error(), "nope") {
		t.Fatalf("bad token error: %v", err)
	}
}

func TestPostMessage(t *testing.T) {
	c, calls := fakeSlack(t, map[string]string{"/chat.postMessage": `{"ok":true}`})
	if err := c.PostMessage("xoxb-1", "C123", "hello"); err != nil {
		t.Fatal(err)
	}
	got := (*calls)[0]
	if got.auth != "Bearer xoxb-1" || got.body["channel"] != "C123" || got.body["text"] != "hello" {
		t.Fatalf("post: %+v", got)
	}
	bad, _ := fakeSlack(t, map[string]string{"/chat.postMessage": `{"ok":false,"error":"channel_not_found"}`})
	if err := bad.PostMessage("t", "C", "x"); err == nil || !strings.Contains(err.Error(), "channel_not_found") {
		t.Fatalf("error: %v", err)
	}
}

func TestLookupByEmail(t *testing.T) {
	c, _ := fakeSlack(t, map[string]string{"/users.lookupByEmail": `{"ok":true,"user":{"id":"U9"}}`})
	if id, found, err := c.LookupByEmail("t", "a@x.co"); err != nil || !found || id != "U9" {
		t.Fatalf("found: %v %v %s", err, found, id)
	}
	miss, _ := fakeSlack(t, map[string]string{"/users.lookupByEmail": `{"ok":false,"error":"users_not_found"}`})
	if _, found, err := miss.LookupByEmail("t", "a@x.co"); err != nil || found {
		t.Fatalf("not found must not be an error: %v %v", err, found)
	}
}

func TestChannels(t *testing.T) {
	c, _ := fakeSlack(t, map[string]string{"/conversations.list": `{"ok":true,"channels":[{"id":"C1","name":"alerts","is_private":true},{"id":"C2","name":"general","is_private":false}]}`})
	chs, err := c.Channels("t")
	if err != nil || len(chs) != 2 || chs[0].Name != "alerts" || !chs[0].Private {
		t.Fatalf("channels: %v %+v", err, chs)
	}
}

func TestSeal(t *testing.T) {
	key := make([]byte, 32)
	key[0] = 1
	a, err := Seal(key, "xoxb-secret")
	if err != nil || strings.Contains(a, "xoxb") {
		t.Fatalf("seal: %v %s", err, a)
	}
	b, _ := Seal(key, "xoxb-secret")
	if a == b {
		t.Error("two seals of the same token must differ")
	}
	if got, err := Open(key, a); err != nil || got != "xoxb-secret" {
		t.Fatalf("open: %v %q", err, got)
	}
	other := make([]byte, 32)
	if _, err := Open(other, a); err == nil {
		t.Error("wrong key opened it")
	}
	if _, err := Open(key, "not-base64!!"); err == nil {
		t.Error("garbage opened")
	}
}

func TestInQuiet(t *testing.T) {
	bkk := time.FixedZone("BKK", 7*3600)
	at := func(h, m int) time.Time { return time.Date(2026, 11, 23, h, m, 0, 0, bkk) }
	cases := []struct {
		now      time.Time
		from, to string
		want     bool
	}{
		{at(21, 0), "21:00", "08:00", true},
		{at(23, 59), "21:00", "08:00", true},
		{at(3, 0), "21:00", "08:00", true},
		{at(7, 59), "21:00", "08:00", true},
		{at(8, 0), "21:00", "08:00", false},
		{at(12, 0), "21:00", "08:00", false},
		{at(12, 0), "12:00", "13:00", true},
		{at(13, 0), "12:00", "13:00", false},
		{at(12, 0), "", "", true}, // unset = defaults 21:00–08:00 → noon is not quiet
	}
	cases[len(cases)-1].want = false
	for _, c := range cases {
		if got := InQuiet(c.now, c.from, c.to); got != c.want {
			t.Errorf("%s in %s–%s = %v, want %v", c.now.Format("15:04"), c.from, c.to, got, c.want)
		}
	}
	if !InQuiet(at(22, 0), "", "") {
		t.Error("empty window must use the 21:00–08:00 default")
	}
}
