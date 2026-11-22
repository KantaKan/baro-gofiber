package handler

import "fmt"

// alerter posts admin alerts to Slack without ever failing the caller.
type alerter interface {
	Alert(kind, text string)
}

// leaveAlertText: the Slack line for a new leave request. after is the leave count if approved (0 = unknown).
func leaveAlertText(name, leaveType, date string, after int) string {
	text := fmt.Sprintf("📝 %s asked for %s leave on %s", name, leaveType, date)
	if after > 0 {
		text += fmt.Sprintf(" (would be %d of 8)", after)
		if after >= 7 {
			text += " · Final warning"
		}
	}
	return text
}
