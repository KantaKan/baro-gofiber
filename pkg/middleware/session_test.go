package middleware

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
)

func sessionApp(t *testing.T) (*fiber.App, string) {
	t.Helper()
	t.Setenv("JWT_SECRET_KEY", "test-secret")
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://generation-barometer.vercel.app")
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, &Claims{UserID: "u1", Role: "learner", RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}}).SignedString([]byte("test-secret"))
	if err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	ok := func(c *fiber.Ctx) error { return c.SendString("ok") }
	app.Get("/me", AuthMiddleware, ok)
	app.Post("/write", AuthMiddleware, ok)
	app.Post("/login", func(c *fiber.Ctx) error { SetSession(c, tok); return ok(c) })
	return app, tok
}

func TestSessionCookieIsHttpOnlyAndWorks(t *testing.T) {
	app, tok := sessionApp(t)
	res, _ := app.Test(httptest.NewRequest("POST", "/login", nil))
	set := strings.ToLower(res.Header.Get("Set-Cookie"))
	for _, want := range []string{SessionCookie + "=", "httponly", "secure", "samesite=lax", "path=/"} {
		if !strings.Contains(set, want) {
			t.Fatalf("cookie missing %q: %s", want, set)
		}
	}
	req := httptest.NewRequest("GET", "/me", nil)
	req.Header.Set("Cookie", SessionCookie+"="+tok)
	if res, _ := app.Test(req); res.StatusCode != 200 {
		t.Fatalf("cookie session should be accepted, got %d", res.StatusCode)
	}
}

func TestCookieWritesMustComeFromOurOwnPages(t *testing.T) {
	app, tok := sessionApp(t)
	try := func(origin string) int {
		req := httptest.NewRequest("POST", "/write", nil)
		req.Header.Set("Cookie", SessionCookie+"="+tok)
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		res, _ := app.Test(req)
		return res.StatusCode
	}
	if got := try("https://generation-barometer.vercel.app"); got != 200 {
		t.Errorf("our own page should be able to write, got %d", got)
	}
	if got := try("https://evil.example"); got != 403 {
		t.Errorf("another site must not ride the cookie, got %d", got)
	}
	if got := try(""); got != 403 {
		t.Errorf("a write with no Origin must not ride the cookie, got %d", got)
	}
	req := httptest.NewRequest("POST", "/write", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	if res, _ := app.Test(req); res.StatusCode != 200 {
		t.Errorf("a Bearer token is not a cookie and needs no Origin, got %d", res.StatusCode)
	}
}

func TestClearSessionExpiresTheCookie(t *testing.T) {
	app := fiber.New()
	app.Post("/logout", func(c *fiber.Ctx) error { ClearSession(c); return nil })
	res, _ := app.Test(httptest.NewRequest("POST", "/logout", nil))
	if set := res.Header.Get("Set-Cookie"); !strings.Contains(set, SessionCookie+"=;") || !strings.Contains(set, "max-age=-1") && !strings.Contains(strings.ToLower(set), "expires=thu, 01 jan 1970") {
		t.Fatalf("logout should expire the cookie: %s", set)
	}
}
