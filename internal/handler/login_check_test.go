package handler

import (
	"testing"

	"gofiber-baro/internal/domain"
	"gofiber-baro/pkg/utils"
)

func TestCanLogIn(t *testing.T) {
	h, _ := utils.HashPassword("secret123")
	if !canLogIn(&domain.User{Role: "admin", Password: h}, "secret123") {
		t.Error("staff should log in")
	}
	if canLogIn(&domain.User{Role: "disabled", Password: h}, "secret123") {
		t.Error("removed member logged in")
	}
	if canLogIn(&domain.User{Role: "learner", Password: h}, "nope") {
		t.Error("wrong password logged in")
	}
}
