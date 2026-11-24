package utils

import (
	"errors"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestErrorHandlerHidesInternalsButKeepsMeantMessages(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: ErrorHandler})
	app.Get("/raw", func(c *fiber.Ctx) error { return errors.New("mongo: connection refused at 10.0.0.5:27017") })
	app.Get("/meant", func(c *fiber.Ctx) error { return fiber.NewError(fiber.StatusUnauthorized, "Unauthorized: Invalid token") })

	res, _ := app.Test(httptest.NewRequest("GET", "/raw", nil))
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != 500 || strings.Contains(string(body), "mongo") || !strings.Contains(string(body), "Something went wrong") {
		t.Fatalf("raw error leaked: %d %s", res.StatusCode, body)
	}

	res, _ = app.Test(httptest.NewRequest("GET", "/meant", nil))
	body, _ = io.ReadAll(res.Body)
	if res.StatusCode != 401 || !strings.Contains(string(body), `"message":"Unauthorized: Invalid token"`) || !strings.Contains(string(body), `"success":false`) {
		t.Fatalf("fiber.NewError should keep its status and message as JSON: %d %s", res.StatusCode, body)
	}
}
