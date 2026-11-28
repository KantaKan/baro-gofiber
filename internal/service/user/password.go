package user

import (
	"errors"
	"io"
	"unicode/utf8"

	"gofiber-baro/internal/domain"
	"gofiber-baro/pkg/utils"
)

const tempAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZabcdefghjkmnpqrstuvwxyz23456789"

var (
	ErrNotLearner   = errors.New("only learner passwords can be reset here")
	ErrWrongCurrent = errors.New("current password is wrong")
	ErrTooShort     = errors.New("new password must be at least 8 characters")
	ErrTooLong      = errors.New("new password must be at most 72 bytes")
	ErrSamePassword = errors.New("new password must be different from the current one")
)

// TemporaryPassword draws 10 characters without modulo bias from an alphabet without lookalikes.
func TemporaryPassword(r io.Reader) (string, error) {
	out := make([]byte, 0, 10)
	buf := make([]byte, 1)
	limit := 256 - 256%len(tempAlphabet)
	for len(out) < 10 {
		if _, err := io.ReadFull(r, buf); err != nil {
			return "", err
		}
		if int(buf[0]) < limit {
			out = append(out, tempAlphabet[int(buf[0])%len(tempAlphabet)])
		}
	}
	return string(out), nil
}

func CanResetPassword(u *domain.User) error {
	if u == nil || u.Role != "learner" {
		return ErrNotLearner
	}
	return nil
}

func CheckNewPassword(hash, current, next string) error {
	if !utils.CheckPasswordHash(current, hash) {
		return ErrWrongCurrent
	}
	if utf8.RuneCountInString(next) < 8 {
		return ErrTooShort
	}
	if len(next) > 72 {
		return ErrTooLong
	}
	if next == current {
		return ErrSamePassword
	}
	return nil
}
