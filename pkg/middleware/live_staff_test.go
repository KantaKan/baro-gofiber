package middleware

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
)

func tokenFor(t *testing.T, role string) string {
	t.Helper()
	t.Setenv("JWT_SECRET_KEY", "test-secret")
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, &Claims{UserID: "u1", Role: role, RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}}).SignedString([]byte("test-secret"))
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func call(t *testing.T, tok string) int {
	t.Helper()
	app := fiber.New()
	app.Get("/x", AuthMiddleware, func(c *fiber.Ctx) error { return c.SendStatus(200) })
	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	res, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	return res.StatusCode
}

func TestStaffTokenIsCheckedAgainstTheDatabase(t *testing.T) {
	defer func() { StaffStillActive = nil }()
	calls := 0
	StaffStillActive = func(id string) (bool, error) { calls++; return false, nil }
	if c := call(t, tokenFor(t, "admin")); c != 403 {
		t.Errorf("removed staff with a valid token got %d", c)
	}
	StaffStillActive = func(string) (bool, error) { return true, nil }
	if c := call(t, tokenFor(t, "admin")); c != 200 {
		t.Errorf("active staff got %d", c)
	}
	calls = 0
	StaffStillActive = func(string) (bool, error) { calls++; return false, nil }
	if c := call(t, tokenFor(t, "learner")); c != 200 || calls != 0 {
		t.Errorf("learners must not cost a staff lookup (status %d, lookups %d)", c, calls)
	}
}
