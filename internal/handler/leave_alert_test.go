package handler

import (
	"strings"
	"testing"
)

func TestLeaveAlertText(t *testing.T) {
	got := leaveAlertText("Mint", "personal", "2026-11-26", 5)
	for _, want := range []string{"Mint", "2026-11-26", "personal", "5 of 8"} {
		if !strings.Contains(got, want) {
			t.Errorf("%q missing %q", got, want)
		}
	}
	if strings.Contains(got, "Final warning") {
		t.Error("5 of 8 is not a final warning")
	}
	if !strings.Contains(leaveAlertText("Mint", "sick", "2026-11-26", 7), "Final warning") {
		t.Error("7 of 8 must warn")
	}
	if !strings.Contains(leaveAlertText("Mint", "sick", "2026-11-26", 8), "Final warning") {
		t.Error("8 of 8 must warn")
	}
	if strings.Contains(leaveAlertText("Mint", "sick", "2026-11-26", 0), "of 8") {
		t.Error("unknown count must not print a number")
	}
}

func TestLeaveAlertTextCannotPingOrLink(t *testing.T) {
	got := leaveAlertText("<!here>", "<!channel>", "<https://evil.example|click>", 3)
	for _, bad := range []string{"<!here>", "<!channel>", "<https://"} {
		if strings.Contains(got, bad) {
			t.Errorf("%q must be escaped in %q", bad, got)
		}
	}
	if !strings.Contains(got, "&lt;!here&gt;") {
		t.Errorf("expected escaped text, got %q", got)
	}
}

func TestValidLeaveInput(t *testing.T) {
	for _, ok := range []string{"late", "half_day", "full_day"} {
		if !validLeaveInput(ok, "2026-11-26") {
			t.Errorf("%s should be valid", ok)
		}
	}
	for _, c := range [][2]string{{"<!channel>", "2026-11-26"}, {"", "2026-11-26"}, {"late", "tomorrow"}, {"late", "2026-13-40"}, {"late", ""}} {
		if validLeaveInput(c[0], c[1]) {
			t.Errorf("%v should be refused", c)
		}
	}
}

func TestSlackPostRules(t *testing.T) {
	if err := checkSlackPost(false, "ta", "groups"); err != nil {
		t.Error("not asking for Slack is always fine")
	}
	if checkSlackPost(true, "admin", "everyone") != nil || checkSlackPost(true, "owner", "everyone") != nil {
		t.Error("an admin may post a whole-cohort announcement to Slack")
	}
	for _, c := range [][2]string{{"admin", "groups"}, {"admin", "people"}, {"admin", ""}, {"ta", "everyone"}, {"viewer", "everyone"}} {
		if checkSlackPost(true, c[0], c[1]) == nil {
			t.Errorf("role %s audience %q must not post to Slack", c[0], c[1])
		}
	}
}
