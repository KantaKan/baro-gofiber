package middleware

import (
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

// SessionCookie holds the login token. It is httpOnly, so page scripts (and anything injected
// into them) can never read it; the browser only sends it back to this site.
const SessionCookie = "baro_session"

const sessionTTL = 30 * 24 * time.Hour

// SetSession stores the token in the httpOnly session cookie.
func SetSession(c *fiber.Ctx, token string) {
	c.Cookie(&fiber.Cookie{
		Name:     SessionCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   int(sessionTTL.Seconds()),
		HTTPOnly: true,
		Secure:   true,
		SameSite: fiber.CookieSameSiteLaxMode,
	})
}

// ClearSession removes the session cookie.
func ClearSession(c *fiber.Ctx) {
	c.Cookie(&fiber.Cookie{Name: SessionCookie, Value: "", Path: "/", MaxAge: -1, Expires: time.Unix(0, 0), HTTPOnly: true, Secure: true, SameSite: fiber.CookieSameSiteLaxMode})
}

func allowedOrigins() []string {
	v := os.Getenv("CORS_ALLOWED_ORIGINS")
	if v == "" {
		v = "http://localhost:5173,https://generation-barometer.vercel.app"
	}
	return strings.Split(v, ",")
}

// sameSiteWrite blocks cross-site request forgery for cookie sessions: a write is only
// accepted when the browser says it came from one of our own pages.
func sameSiteWrite(c *fiber.Ctx) bool {
	switch c.Method() {
	case fiber.MethodGet, fiber.MethodHead, fiber.MethodOptions:
		return true
	}
	origin := c.Get("Origin")
	for _, o := range allowedOrigins() {
		if origin != "" && strings.TrimSpace(o) == origin {
			return true
		}
	}
	return false
}
