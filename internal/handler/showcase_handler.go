package handler

import (
	"errors"
	"strconv"

	"gofiber-baro/internal/service/showcase"
	"gofiber-baro/pkg/middleware"
	"gofiber-baro/pkg/utils"

	"github.com/gofiber/fiber/v2"
)

type ShowcaseHandler struct{ service *showcase.Service }

func NewShowcaseHandler(service *showcase.Service) *ShowcaseHandler {
	return &ShowcaseHandler{service: service}
}

type showcaseSaveRequest struct {
	CharacterID string `json:"character_id"`
	Message     string `json:"message"`
}

type showcaseReactionRequest struct {
	Emoji string `json:"emoji"`
}

type showcaseModerationRequest struct {
	Hidden *bool  `json:"hidden"`
	Reason string `json:"reason"`
}

func (h *ShowcaseHandler) Mine(c *fiber.Ctx) error {
	claims, ok := c.Locals("user").(*middleware.Claims)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}
	entry, err := h.service.Mine(c.UserContext(), claims.UserID)
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, err.Error())
	}
	return utils.SendResponse(c, fiber.StatusOK, "Own showcase entry retrieved", entry)
}

func (h *ShowcaseHandler) List(c *fiber.Ctx) error {
	claims, ok := c.Locals("user").(*middleware.Claims)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}
	cohort := 0
	if raw := c.Query("cohort"); raw != "" {
		var err error
		cohort, err = strconv.Atoi(raw)
		if err != nil || cohort <= 0 {
			return utils.SendError(c, fiber.StatusBadRequest, "Invalid cohort")
		}
	}
	includeHidden := c.Query("include_hidden") == "true"
	entries, err := h.service.List(c.UserContext(), claims.UserID, claims.Role == "admin", cohort, c.Query("team"), includeHidden)
	if errors.Is(err, showcase.ErrCohortForbidden) || errors.Is(err, showcase.ErrAdminRequired) {
		return utils.SendError(c, fiber.StatusForbidden, err.Error())
	}
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, err.Error())
	}
	return utils.SendResponse(c, fiber.StatusOK, "Showcase Lawn retrieved", entries)
}

func (h *ShowcaseHandler) React(c *fiber.Ctx) error {
	claims, ok := c.Locals("user").(*middleware.Claims)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}
	var body showcaseReactionRequest
	if err := c.BodyParser(&body); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}
	reacted, err := h.service.React(c.UserContext(), claims.UserID, c.Params("ownerId"), body.Emoji)
	if errors.Is(err, showcase.ErrEntryNotFound) {
		return utils.SendError(c, fiber.StatusNotFound, err.Error())
	}
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, err.Error())
	}
	return utils.SendResponse(c, fiber.StatusOK, "Reaction updated", fiber.Map{"reacted": reacted})
}

func (h *ShowcaseHandler) Moderate(c *fiber.Ctx) error {
	claims, ok := c.Locals("user").(*middleware.Claims)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}
	var body showcaseModerationRequest
	if err := c.BodyParser(&body); err != nil || body.Hidden == nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Hidden status is required")
	}
	err := h.service.Moderate(c.UserContext(), claims.UserID, c.Params("ownerId"), *body.Hidden, body.Reason)
	if errors.Is(err, showcase.ErrAdminRequired) {
		return utils.SendError(c, fiber.StatusForbidden, err.Error())
	}
	if errors.Is(err, showcase.ErrEntryNotFound) {
		return utils.SendError(c, fiber.StatusNotFound, err.Error())
	}
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, err.Error())
	}
	return utils.SendResponse(c, fiber.StatusOK, "Showcase moderation saved", fiber.Map{"hidden": *body.Hidden})
}

func (h *ShowcaseHandler) Save(c *fiber.Ctx) error {
	claims, ok := c.Locals("user").(*middleware.Claims)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}
	var body showcaseSaveRequest
	if err := c.BodyParser(&body); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}
	if err := h.service.Save(c.UserContext(), claims.UserID, body.CharacterID, body.Message); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, err.Error())
	}
	return utils.SendResponse(c, fiber.StatusOK, "Showcase pin saved", fiber.Map{"saved": true})
}

type showcaseMoodRequest struct {
	Mood string `json:"mood"`
}

func (h *ShowcaseHandler) SetMood(c *fiber.Ctx) error {
	claims, ok := c.Locals("user").(*middleware.Claims)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}
	var body showcaseMoodRequest
	if err := c.BodyParser(&body); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}
	state, err := h.service.SetMood(c.UserContext(), claims.UserID, body.Mood)
	if errors.Is(err, showcase.ErrEntryNotFound) {
		return utils.SendError(c, fiber.StatusNotFound, "Pin a character on the lawn before choosing a mood")
	}
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, err.Error())
	}
	return utils.SendResponse(c, fiber.StatusOK, "Lawn mood saved", state)
}

type showcaseEmoteRequest struct {
	Emote  string `json:"emote"`
	Target string `json:"target"`
}

func (h *ShowcaseHandler) SetEmote(c *fiber.Ctx) error {
	claims, ok := c.Locals("user").(*middleware.Claims)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}
	var body showcaseEmoteRequest
	if err := c.BodyParser(&body); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}
	state, err := h.service.SetEmote(c.UserContext(), claims.UserID, body.Emote, body.Target)
	if errors.Is(err, showcase.ErrEntryNotFound) {
		return utils.SendError(c, fiber.StatusNotFound, "Pin a character on the lawn before using an emote")
	}
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, err.Error())
	}
	return utils.SendResponse(c, fiber.StatusOK, "Lawn emote saved", state)
}

func (h *ShowcaseHandler) Remove(c *fiber.Ctx) error {
	claims, ok := c.Locals("user").(*middleware.Claims)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}
	if err := h.service.Remove(c.UserContext(), claims.UserID); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, err.Error())
	}
	return utils.SendResponse(c, fiber.StatusOK, "Showcase pin removed", fiber.Map{"removed": true})
}
