package handler

import (
	"strings"

	"gofiber-baro/internal/domain"
)

// reflectionAnswersComplete reports whether all four reflection answers
// (tech went well / to improve, non-tech went well / to improve) have text.
func reflectionAnswersComplete(c domain.ReflectionContent) bool {
	for _, s := range []string{c.TechSessions.Happy, c.TechSessions.Improve, c.NonTechSessions.Happy, c.NonTechSessions.Improve} {
		if strings.TrimSpace(s) == "" {
			return false
		}
	}
	return true
}
