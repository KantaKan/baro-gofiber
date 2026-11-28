package user

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"gofiber-baro/internal/domain"
	"gofiber-baro/pkg/utils"
)

func TestTemporaryPassword(t *testing.T) {
	p, err := TemporaryPassword(bytes.NewReader(bytes.Repeat([]byte{0, 1, 2, 250}, 50)))
	if err != nil || len(p) != 10 {
		t.Fatalf("got %q, %v", p, err)
	}
	for _, r := range p {
		if !strings.ContainsRune(tempAlphabet, r) {
			t.Errorf("char %q outside alphabet", r)
		}
	}
}

func TestCanResetPassword(t *testing.T) {
	if err := CanResetPassword(&domain.User{Role: "learner"}); err != nil {
		t.Errorf("learner: %v", err)
	}
	for _, role := range []string{"admin", "disabled", ""} {
		if err := CanResetPassword(&domain.User{Role: role}); !errors.Is(err, ErrNotLearner) {
			t.Errorf("role %q: got %v", role, err)
		}
	}
}

func TestCheckNewPassword(t *testing.T) {
	hash, _ := utils.HashPassword("oldpass12")
	cases := []struct {
		cur, next string
		want      error
	}{
		{"wrong", "newpass12", ErrWrongCurrent},
		{"oldpass12", "short", ErrTooShort},
		{"oldpass12", "oldpass12", ErrSamePassword},
		{"oldpass12", "        ", nil},
		{"oldpass12", "newpass12", nil},
	}
	for _, c := range cases {
		if err := CheckNewPassword(hash, c.cur, c.next); !errors.Is(err, c.want) && !(err == nil && c.want == nil) {
			t.Errorf("%q→%q: got %v want %v", c.cur, c.next, err, c.want)
		}
	}
}
