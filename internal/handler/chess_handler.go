package handler

import (
	"context"
	"errors"
	"regexp"
	"strconv"
	"time"

	"gofiber-baro/internal/domain"
	"gofiber-baro/internal/service/chess"
	"gofiber-baro/pkg/utils"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
)

type chessUsers interface {
	GetUserByIDWithFields(id string, projection bson.M) (*domain.User, error)
	ListCohortLearners(cohort int, projection bson.M) ([]domain.User, error)
}

var personFields = bson.M{"_id": 1, "first_name": 1, "zoom_name": 1, "genmate_group": 1, "cohort_number": 1, "role": 1}

var zoomNick = regexp.MustCompile(`\(([^)/]*)`)

// nickname is the English nickname inside a Zoom name like "03_Aphiradee(Black/แบล็ค)", else the first name.
func nickname(u domain.User) string {
	if m := zoomNick.FindStringSubmatch(u.ZoomName); len(m) == 2 && m[1] != "" {
		return m[1]
	}
	return u.FirstName
}

// ChessPeople adapts the user service to the chess service.
type ChessPeople struct{ Users chessUsers }

func (p ChessPeople) Get(id string) (*chess.Person, error) {
	u, err := p.Users.GetUserByIDWithFields(id, personFields)
	if err != nil || u == nil {
		return nil, errors.New("no user")
	}
	return &chess.Person{ID: u.ID.Hex(), Name: nickname(*u), Group: u.GenmateGroup, Cohort: u.CohortNumber, Staff: u.Role == "admin"}, nil
}

func (p ChessPeople) Learners(cohort int) ([]chess.Person, error) {
	us, err := p.Users.ListCohortLearners(cohort, personFields)
	if err != nil {
		return nil, err
	}
	out := make([]chess.Person, 0, len(us))
	for _, u := range us {
		if u.Role == "learner" {
			out = append(out, chess.Person{ID: u.ID.Hex(), Name: nickname(u), Group: u.GenmateGroup, Cohort: u.CohortNumber})
		}
	}
	return out, nil
}

type ChessHandler struct{ svc *chess.Service }

func NewChessHandler(svc *chess.Service) *ChessHandler { return &ChessHandler{svc: svc} }

func chessCtx(c *fiber.Ctx) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	return chess.WithCohort(ctx, c.QueryInt("cohort")), cancel
}

func chessError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, chess.ErrIllegal), errors.Is(err, chess.ErrConflict):
		return utils.SendError(c, fiber.StatusConflict, err.Error())
	case errors.Is(err, chess.ErrNotYourTurn), errors.Is(err, chess.ErrNotYourSide), errors.Is(err, chess.ErrNoAccess):
		return utils.SendError(c, fiber.StatusForbidden, err.Error())
	case errors.Is(err, chess.ErrNoGame):
		return utils.SendError(c, fiber.StatusNotFound, err.Error())
	case errors.Is(err, chess.ErrBadText):
		return utils.SendError(c, fiber.StatusBadRequest, err.Error())
	}
	return utils.SendError(c, fiber.StatusInternalServerError, "Chess is unavailable right now")
}

func uid(c *fiber.Ctx) string { s, _ := c.Locals("userID").(string); return s }

// @Summary  Current team chess game  @Tags chess  @Security BearerAuth  @Router /chess/current [get]
func (h *ChessHandler) Current(c *fiber.Ctx) error {
	ctx, cancel := chessCtx(c)
	defer cancel()
	v, err := h.svc.View(ctx, uid(c))
	if err != nil {
		return chessError(c, err)
	}
	return utils.SendResponse(c, fiber.StatusOK, "Chess", v)
}

// @Summary  Is it my turn  @Tags chess  @Security BearerAuth  @Router /chess/me [get]
func (h *ChessHandler) Me(c *fiber.Ctx) error {
	ctx, cancel := chessCtx(c)
	defer cancel()
	on, mine := h.svc.Me(ctx, uid(c))
	return utils.SendResponse(c, fiber.StatusOK, "Chess", fiber.Map{"enabled": on, "my_turn": mine})
}

// @Summary  Play my move  @Tags chess  @Security BearerAuth  @Router /chess/move [post]
func (h *ChessHandler) Move(c *fiber.Ctx) error {
	var b struct {
		UCI string `json:"uci"`
	}
	if err := c.BodyParser(&b); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}
	ctx, cancel := chessCtx(c)
	defer cancel()
	if err := h.svc.Move(ctx, uid(c), b.UCI); err != nil {
		return chessError(c, err)
	}
	return utils.SendResponse(c, fiber.StatusOK, "Moved", nil)
}

// @Summary  Propose a move to my team  @Tags chess  @Security BearerAuth  @Router /chess/proposals [post]
func (h *ChessHandler) Propose(c *fiber.Ctx) error {
	var b struct {
		UCI  string `json:"uci"`
		Note string `json:"note"`
	}
	if err := c.BodyParser(&b); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}
	ctx, cancel := chessCtx(c)
	defer cancel()
	if err := h.svc.Propose(ctx, uid(c), b.UCI, b.Note); err != nil {
		return chessError(c, err)
	}
	return utils.SendResponse(c, fiber.StatusOK, "Proposed", nil)
}

// @Summary  Agree or un-agree with a proposal  @Tags chess  @Security BearerAuth  @Router /chess/proposals/{id}/vote [post]
func (h *ChessHandler) Vote(c *fiber.Ctx) error {
	ctx, cancel := chessCtx(c)
	defer cancel()
	if err := h.svc.Vote(ctx, uid(c), c.Params("id")); err != nil {
		return chessError(c, err)
	}
	return utils.SendResponse(c, fiber.StatusOK, "Voted", nil)
}

// @Summary  Room messages  @Tags chess  @Security BearerAuth  @Router /chess/messages [get]
func (h *ChessHandler) Messages(c *fiber.Ctx) error {
	ctx, cancel := chessCtx(c)
	defer cancel()
	ms, err := h.svc.Messages(ctx, uid(c), c.Query("room"), c.Query("after"))
	if err != nil {
		return chessError(c, err)
	}
	return utils.SendResponse(c, fiber.StatusOK, "Messages", ms)
}

// @Summary  Post to a room  @Tags chess  @Security BearerAuth  @Router /chess/messages [post]
func (h *ChessHandler) Post(c *fiber.Ctx) error {
	var b struct {
		Room string `json:"room"`
		Text string `json:"text"`
	}
	if err := c.BodyParser(&b); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}
	ctx, cancel := chessCtx(c)
	defer cancel()
	m, err := h.svc.Post(ctx, uid(c), b.Room, b.Text)
	if err != nil {
		return chessError(c, err)
	}
	return utils.SendResponse(c, fiber.StatusOK, "Posted", m)
}

// @Summary  End a cohort's game  @Tags admin  @Security BearerAuth  @Router /admin/chess/{cohort}/end [post]
func (h *ChessHandler) End(c *fiber.Ctx) error {
	n, err := strconv.Atoi(c.Params("cohort"))
	if err != nil || n <= 0 {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid cohort")
	}
	ctx, cancel := chessCtx(c)
	defer cancel()
	if err := h.svc.End(ctx, n); err != nil {
		return chessError(c, err)
	}
	return utils.SendResponse(c, fiber.StatusOK, "Ended", nil)
}
