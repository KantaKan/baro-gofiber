package handler

import (
	"testing"
	"time"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestFeedbackAuthor(t *testing.T) {
	now := time.Date(2026, 11, 23, 18, 0, 0, 0, time.UTC)
	id := primitive.NewObjectID()
	cases := []struct {
		name string
		u    *domain.User
		want string
	}{
		{"zoom name when set", &domain.User{ID: id, ZoomName: "00_Kan(Kan/กัน)", FirstName: "Kantapon"}, "00_Kan(Kan/กัน)"},
		{"first name otherwise", &domain.User{ID: id, FirstName: "Neeti"}, "Neeti"},
		{"blank when nothing", &domain.User{ID: id}, ""},
	}
	for _, tc := range cases {
		got := feedbackAuthor(tc.u, now)
		if got.Name != tc.want || got.ID != id.Hex() || !got.At.Equal(now) {
			t.Errorf("%s: got %+v", tc.name, got)
		}
	}
	if got := feedbackAuthor(nil, now); got.ID != "" || got.Name != "" {
		t.Errorf("unknown replier: got %+v", got)
	}
}
