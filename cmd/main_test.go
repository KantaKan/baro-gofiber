package main

import (
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestFiberConfigReadsLeftmostForwardedIP(t *testing.T) {
	app := fiber.New(fiberConfig("X-Forwarded-For"))
	app.Get("/", func(c *fiber.Ctx) error { return c.SendString(c.IP()) })
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.5, 10.0.0.2")
	res, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	if string(body) != "203.0.113.5" {
		t.Errorf("got %q want 203.0.113.5", body)
	}
}
