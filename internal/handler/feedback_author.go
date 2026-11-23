package handler

import (
	"time"

	"gofiber-baro/internal/domain"
)

// feedbackAuthor records who replied to a reflection: their id and the name learners know them by
// (Zoom name, which the app turns into a nickname, else first name).
func feedbackAuthor(u *domain.User, now time.Time) domain.FeedbackAuthor {
	a := domain.FeedbackAuthor{At: now}
	if u == nil {
		return a
	}
	a.ID = u.ID.Hex()
	a.Avatar = u.Avatar
	a.Name = u.ZoomName
	if a.Name == "" {
		a.Name = u.FirstName
	}
	return a
}
