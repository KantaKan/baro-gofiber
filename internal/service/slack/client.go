// Package slack talks to Slack's Web API with a bot token and keeps that token sealed at rest.
package slack

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client calls the Slack Web API. BaseURL is overridable so tests can use a fake server.
type Client struct {
	BaseURL string
	HTTP    *http.Client
}

func NewClient() *Client {
	return &Client{BaseURL: "https://slack.com/api", HTTP: &http.Client{Timeout: 10 * time.Second}}
}

type Channel struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Private bool   `json:"private"`
}

// SlackError carries Slack's own error code (e.g. invalid_auth, channel_not_found). It never
// contains the token.
type SlackError struct{ Code string }

func (e *SlackError) Error() string { return "slack: " + e.Code }

func (c *Client) do(method, path, token string, form url.Values, jsonBody any, out any) error {
	var body *bytes.Reader
	ctype := ""
	switch {
	case jsonBody != nil:
		b, _ := json.Marshal(jsonBody)
		body, ctype = bytes.NewReader(b), "application/json"
	case form != nil:
		body, ctype = bytes.NewReader([]byte(form.Encode())), "application/x-www-form-urlencoded"
	default:
		body = bytes.NewReader(nil)
	}
	req, err := http.NewRequest(method, strings.TrimRight(c.BaseURL, "/")+path, body)
	if err != nil {
		return errors.New("slack: bad request")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if ctype != "" {
		req.Header.Set("Content-Type", ctype)
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		// Don't wrap err: its text can include the URL but never the header, still keep it terse.
		return errors.New("slack: could not reach Slack")
	}
	defer res.Body.Close()
	var env struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	raw := new(bytes.Buffer)
	if _, err := raw.ReadFrom(res.Body); err != nil {
		return errors.New("slack: could not read the reply")
	}
	if err := json.Unmarshal(raw.Bytes(), &env); err != nil {
		return fmt.Errorf("slack: unexpected reply (HTTP %d)", res.StatusCode)
	}
	if !env.OK {
		code := env.Error
		if code == "" {
			code = "unknown_error"
		}
		return &SlackError{Code: code}
	}
	if out != nil {
		return json.Unmarshal(raw.Bytes(), out)
	}
	return nil
}

// AuthTest verifies a bot token and returns the workspace and bot names.
func (c *Client) AuthTest(token string) (team, bot string, err error) {
	var r struct {
		Team string `json:"team"`
		User string `json:"user"`
	}
	if err = c.do("POST", "/auth.test", token, nil, nil, &r); err != nil {
		return "", "", err
	}
	return r.Team, r.User, nil
}

// PostMessage posts to a channel id, or to a user id (which opens a DM).
func (c *Client) PostMessage(token, channel, text string) error {
	return c.do("POST", "/chat.postMessage", token, nil, map[string]string{"channel": channel, "text": text}, nil)
}

// LookupByEmail finds a Slack user id; found=false (not an error) when there is no such user.
func (c *Client) LookupByEmail(token, email string) (string, bool, error) {
	var r struct {
		User struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	err := c.do("POST", "/users.lookupByEmail", token, url.Values{"email": {email}}, nil, &r)
	var se *SlackError
	if errors.As(err, &se) && se.Code == "users_not_found" {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return r.User.ID, r.User.ID != "", nil
}

// Channels lists public and private channels the bot can see.
func (c *Client) Channels(token string) ([]Channel, error) {
	var r struct {
		Channels []struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			Private bool   `json:"is_private"`
		} `json:"channels"`
	}
	form := url.Values{"types": {"public_channel,private_channel"}, "exclude_archived": {"true"}, "limit": {"200"}}
	if err := c.do("POST", "/conversations.list", token, form, nil, &r); err != nil {
		return nil, err
	}
	out := make([]Channel, 0, len(r.Channels))
	for _, ch := range r.Channels {
		out = append(out, Channel{ID: ch.ID, Name: ch.Name, Private: ch.Private})
	}
	return out, nil
}
