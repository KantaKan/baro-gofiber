package handler

import (
	"errors"
	"fmt"
	"net/http/httptest"
	"testing"

	"gofiber-baro/internal/domain"
	"gofiber-baro/internal/service/chess"

	"github.com/gofiber/fiber/v2"
)

func TestChessErrorStatus(t *testing.T) {
	cases := map[error]int{
		chess.ErrIllegal:                         409,
		chess.ErrConflict:                        409,
		chess.ErrNotYourTurn:                     403,
		chess.ErrNotYourSide:                     403,
		chess.ErrNoAccess:                        403,
		chess.ErrNoGame:                          404,
		chess.ErrBadText:                         400,
		fmt.Errorf("wrap: %w", chess.ErrIllegal): 409,
		errors.New("mongo down"):                 500,
	}
	for err, want := range cases {
		app := fiber.New()
		app.Get("/", func(c *fiber.Ctx) error { return chessError(c, err) })
		res, _ := app.Test(httptest.NewRequest("GET", "/", nil))
		if res.StatusCode != want {
			t.Errorf("%v: got %d want %d", err, res.StatusCode, want)
		}
	}
}

func TestChessNickname(t *testing.T) {
	if got := nickname(domain.User{FirstName: "Aphiradee", ZoomName: "03_Aphiradee(Black/แบล็ค)"}); got != "Black" {
		t.Errorf("zoom nickname: %q", got)
	}
	if got := nickname(domain.User{FirstName: "Somchai"}); got != "Somchai" {
		t.Errorf("first name fallback: %q", got)
	}
}
