package handler

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"gofiber-baro/internal/domain"
)

// alerter posts admin alerts to Slack without ever failing the caller.
type alerter interface {
	Alert(kind, text string)
}

var slackEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")

// slackSafe makes learner-written text harmless in Slack: no <!channel>, @mentions via <@U..> or links.
func slackSafe(s string) string { return slackEscaper.Replace(s) }

// leaveAlertText: the Slack line for a new leave request. after is the leave count if approved (0 = unknown).
func leaveAlertText(name, leaveType, date string, after int) string {
	text := fmt.Sprintf("📝 %s asked for %s leave on %s", slackSafe(name), slackSafe(leaveType), slackSafe(date))
	if after > 0 {
		text += fmt.Sprintf(" (would be %d of 8)", after)
		if after >= 7 {
			text += " · Final warning"
		}
	}
	return text
}

// validLeaveInput: a known leave type and a real YYYY-MM-DD date.
func validLeaveInput(leaveType, date string) bool {
	switch domain.LeaveType(leaveType) {
	case domain.LeaveTypeLate, domain.LeaveTypeHalfDay, domain.LeaveTypeFullDay:
	default:
		return false
	}
	_, err := time.Parse("2006-01-02", date)
	return err == nil
}

// checkSlackPost: only an Owner/Admin may post, and only a whole-cohort announcement, so a note
// meant for a few learners never lands in a shared channel.
func checkSlackPost(wants bool, role, audience string) error {
	if !wants {
		return nil
	}
	if role != "owner" && role != "admin" {
		return errors.New("Only an Owner or Admin can post announcements to Slack")
	}
	if audience != "everyone" {
		return errors.New("Only announcements to everyone can also be posted to Slack")
	}
	return nil
}
