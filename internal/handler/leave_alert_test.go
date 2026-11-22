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
