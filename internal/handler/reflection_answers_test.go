package handler

import (
	"testing"

	"gofiber-baro/internal/domain"
)

func TestReflectionAnswersComplete(t *testing.T) {
	full := domain.ReflectionContent{
		TechSessions:    domain.SessionDetails{Happy: "a", Improve: "b"},
		NonTechSessions: domain.SessionDetails{Happy: "c", Improve: "d"},
	}
	blank := func(mod func(*domain.ReflectionContent)) domain.ReflectionContent {
		c := full
		mod(&c)
		return c
	}
	cases := []struct {
		name string
		in   domain.ReflectionContent
		want bool
	}{
		{"all four answered", full, true},
		{"tech went well empty", blank(func(c *domain.ReflectionContent) { c.TechSessions.Happy = "" }), false},
		{"tech improve only spaces", blank(func(c *domain.ReflectionContent) { c.TechSessions.Improve = "  \n\t" }), false},
		{"non-tech went well empty", blank(func(c *domain.ReflectionContent) { c.NonTechSessions.Happy = "" }), false},
		{"non-tech improve empty", blank(func(c *domain.ReflectionContent) { c.NonTechSessions.Improve = "" }), false},
		{"nothing answered", domain.ReflectionContent{}, false},
	}
	for _, tc := range cases {
		if got := reflectionAnswersComplete(tc.in); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}
