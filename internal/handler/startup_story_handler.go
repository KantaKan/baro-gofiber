package handler

import (
	"errors"
	"log"

	"gofiber-baro/internal/domain"
	"gofiber-baro/internal/service/startupstory"
	"gofiber-baro/pkg/middleware"
	"gofiber-baro/pkg/utils"

	"github.com/gofiber/fiber/v2"
)

type StartupStoryHandler struct {
	service *startupstory.Service
}

func NewStartupStoryHandler(service *startupstory.Service) *StartupStoryHandler {
	return &StartupStoryHandler{service: service}
}

func startupPlayer(c *fiber.Ctx) (startupstory.Player, bool) {
	claims, ok := c.Locals("user").(*middleware.Claims)
	if !ok {
		return startupstory.Player{}, false
	}
	return startupstory.Player{ID: claims.UserID, Cohort: claims.Cohort, Role: claims.Role}, true
}

func startupError(c *fiber.Ctx, err error) error {
	status := fiber.StatusInternalServerError
	switch {
	case errors.Is(err, domain.ErrStartupRunActive), errors.Is(err, domain.ErrStartupRunConflict):
		status = fiber.StatusConflict
	case errors.Is(err, domain.ErrStartupTooEarly):
		status = fiber.StatusTooEarly
	case errors.Is(err, domain.ErrStartupNoActiveRun):
		status = fiber.StatusNotFound
	case errors.Is(err, domain.ErrStartupNoAttempts):
		status = fiber.StatusForbidden
	case errors.Is(err, domain.ErrStartupWrongStage), errors.Is(err, domain.ErrStartupInvalidChoice), errors.Is(err, domain.ErrStartupTeamFull), errors.Is(err, domain.ErrStartupDeskLimit), errors.Is(err, domain.ErrStartupNoFunds):
		status = fiber.StatusBadRequest
	default:
		log.Printf("startup story: %v", err)
		return utils.SendError(c, status, "Something went wrong with your startup, please try again")
	}
	return utils.SendError(c, status, err.Error())
}

func (h *StartupStoryHandler) Overview(c *fiber.Ctx) error {
	player, ok := startupPlayer(c)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}
	overview, err := h.service.Overview(c.UserContext(), player)
	if err != nil {
		return startupError(c, err)
	}
	return utils.SendResponse(c, fiber.StatusOK, "Startup Story retrieved", overview)
}

func (h *StartupStoryHandler) Leaderboard(c *fiber.Ctx) error {
	player, ok := startupPlayer(c)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}
	entries, err := h.service.Leaderboard(c.UserContext(), player, c.Query("tab"))
	if err != nil {
		return startupError(c, err)
	}
	return utils.SendResponse(c, fiber.StatusOK, "Leaderboard retrieved", entries)
}

func (h *StartupStoryHandler) OptOut(c *fiber.Ctx) error {
	player, ok := startupPlayer(c)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}
	var body struct {
		OptOut bool `json:"opt_out"`
	}
	if err := c.BodyParser(&body); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}
	out, err := h.service.OptOut(c.UserContext(), player, body.OptOut)
	if err != nil {
		return startupError(c, err)
	}
	return utils.SendResponse(c, fiber.StatusOK, "Preference saved", fiber.Map{"opt_out": out})
}

func (h *StartupStoryHandler) StartRun(c *fiber.Ctx) error {
	player, ok := startupPlayer(c)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}
	var body struct {
		Mode string `json:"mode"`
	}
	if err := c.BodyParser(&body); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}
	run, err := h.service.StartRun(c.UserContext(), player, body.Mode)
	if err != nil {
		return startupError(c, err)
	}
	return utils.SendResponse(c, fiber.StatusCreated, "Startup founded", run)
}

func (h *StartupStoryHandler) PickFounder(c *fiber.Ctx) error {
	player, ok := startupPlayer(c)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}
	var body struct {
		Index int `json:"index"`
	}
	if err := c.BodyParser(&body); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}
	run, err := h.service.PickFounder(c.UserContext(), player, body.Index)
	if err != nil {
		return startupError(c, err)
	}
	return utils.SendResponse(c, fiber.StatusOK, "Founder picked", run)
}

func (h *StartupStoryHandler) StartProject(c *fiber.Ctx) error {
	player, ok := startupPlayer(c)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}
	var body struct {
		Type       string   `json:"type"`
		Theme      string   `json:"theme"`
		StaffIDs   []string `json:"staff_ids"`
		PitchIndex *int     `json:"pitch_index"`
	}
	if err := c.BodyParser(&body); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}
	var run *domain.StartupRun
	var err error
	if body.PitchIndex != nil {
		run, err = h.service.StartProjectPitch(c.UserContext(), player, *body.PitchIndex, body.StaffIDs)
	} else {
		run, err = h.service.StartProject(c.UserContext(), player, body.Type, body.Theme, body.StaffIDs)
	}
	if err != nil {
		return startupError(c, err)
	}
	return utils.SendResponse(c, fiber.StatusOK, "Project started", run)
}

func (h *StartupStoryHandler) Ship(c *fiber.Ctx) error {
	player, ok := startupPlayer(c)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}
	run, err := h.service.Ship(c.UserContext(), player)
	if err != nil {
		return startupError(c, err)
	}
	return utils.SendResponse(c, fiber.StatusOK, "Shipped", run)
}

func (h *StartupStoryHandler) Hire(c *fiber.Ctx) error {
	player, ok := startupPlayer(c)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}
	var body struct {
		CandidateID string `json:"candidate_id"`
	}
	if err := c.BodyParser(&body); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}
	run, err := h.service.Hire(c.UserContext(), player, body.CandidateID)
	if err != nil {
		return startupError(c, err)
	}
	return utils.SendResponse(c, fiber.StatusOK, "Hired", run)
}

func (h *StartupStoryHandler) BuyDesk(c *fiber.Ctx) error {
	player, ok := startupPlayer(c)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}
	run, err := h.service.BuyDesk(c.UserContext(), player)
	if err != nil {
		return startupError(c, err)
	}
	return utils.SendResponse(c, fiber.StatusOK, "Desk bought", run)
}

func (h *StartupStoryHandler) UpgradeDesk(c *fiber.Ctx) error {
	player, ok := startupPlayer(c)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}
	var body struct {
		Index int `json:"index"`
	}
	if err := c.BodyParser(&body); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}
	run, err := h.service.UpgradeDesk(c.UserContext(), player, body.Index)
	if err != nil {
		return startupError(c, err)
	}
	return utils.SendResponse(c, fiber.StatusOK, "Desk upgraded", run)
}

func (h *StartupStoryHandler) Infra(c *fiber.Ctx) error {
	player, ok := startupPlayer(c)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}
	var body struct {
		Action string `json:"action"`
		Index  int    `json:"index"`
		ID     string `json:"id"`
	}
	if err := c.BodyParser(&body); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}
	run, err := h.service.Infra(c.UserContext(), player, body.Action, body.Index, body.ID)
	if err != nil {
		return startupError(c, err)
	}
	return utils.SendResponse(c, fiber.StatusOK, "Infra updated", run)
}

func (h *StartupStoryHandler) PickItem(c *fiber.Ctx) error {
	player, ok := startupPlayer(c)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}
	var body struct {
		Index int `json:"index"`
	}
	if err := c.BodyParser(&body); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}
	run, err := h.service.PickItem(c.UserContext(), player, body.Index)
	if err != nil {
		return startupError(c, err)
	}
	return utils.SendResponse(c, fiber.StatusOK, "Item drafted", run)
}

func (h *StartupStoryHandler) PickPerk(c *fiber.Ctx) error {
	player, ok := startupPlayer(c)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}
	var body struct {
		Index int `json:"index"`
	}
	if err := c.BodyParser(&body); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}
	run, err := h.service.PickPerk(c.UserContext(), player, body.Index)
	if err != nil {
		return startupError(c, err)
	}
	return utils.SendResponse(c, fiber.StatusOK, "Perk picked", run)
}

func (h *StartupStoryHandler) PickEvent(c *fiber.Ctx) error {
	player, ok := startupPlayer(c)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}
	var body struct {
		Index int `json:"index"`
	}
	if err := c.BodyParser(&body); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}
	run, err := h.service.PickEvent(c.UserContext(), player, body.Index)
	if err != nil {
		return startupError(c, err)
	}
	return utils.SendResponse(c, fiber.StatusOK, "Event resolved", run)
}

func (h *StartupStoryHandler) IPOChoice(c *fiber.Ctx) error {
	player, ok := startupPlayer(c)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}
	var body struct {
		KeepGoing bool `json:"keep_going"`
	}
	if err := c.BodyParser(&body); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}
	run, err := h.service.IPOChoice(c.UserContext(), player, body.KeepGoing)
	if err != nil {
		return startupError(c, err)
	}
	return utils.SendResponse(c, fiber.StatusOK, "IPO choice made", run)
}

func (h *StartupStoryHandler) Abandon(c *fiber.Ctx) error {
	player, ok := startupPlayer(c)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}
	run, err := h.service.Abandon(c.UserContext(), player)
	if err != nil {
		return startupError(c, err)
	}
	return utils.SendResponse(c, fiber.StatusOK, "Startup pivoted", run)
}

func (h *StartupStoryHandler) Dismiss(c *fiber.Ctx) error {
	player, ok := startupPlayer(c)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}
	run, err := h.service.Dismiss(c.UserContext(), player, c.Params("id"))
	if err != nil {
		return startupError(c, err)
	}
	return utils.SendResponse(c, fiber.StatusOK, "Dismissed", run)
}
