package user

import (
	"errors"
	"testing"
	"time"

	"gofiber-baro/internal/domain"
)

func TestCanEditReflection(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Bangkok")
	now := time.Date(2026, 11, 28, 20, 0, 0, 0, loc)
	cases := []struct {
		name string
		r    domain.Reflection
		want error
	}{
		{"today, no reply", domain.Reflection{Day: "2026-11-28"}, nil},
		{"today by created time", domain.Reflection{CreatedAt: time.Date(2026, 11, 28, 10, 0, 0, 0, time.UTC)}, nil},
		{"created late last night UTC but today in Bangkok", domain.Reflection{CreatedAt: time.Date(2026, 11, 27, 18, 30, 0, 0, time.UTC)}, nil},
		{"yesterday", domain.Reflection{Day: "2026-11-27"}, ErrReflectionNotToday},
		{"replied", domain.Reflection{Day: "2026-11-28", AdminFeedback: "nice"}, ErrReflectionReplied},
	}
	for _, c := range cases {
		if err := CanEditReflection(c.r, now); !errors.Is(err, c.want) && !(err == nil && c.want == nil) {
			t.Errorf("%s: got %v want %v", c.name, err, c.want)
		}
	}
}
