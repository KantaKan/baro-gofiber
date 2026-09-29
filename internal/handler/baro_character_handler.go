package handler

import (
	"context"
	"time"

	"gofiber-baro/internal/service/character"
	"gofiber-baro/pkg/middleware"
	"gofiber-baro/pkg/utils"

	"github.com/gofiber/fiber/v2"
)

type BaroCharacterHandler struct {
	service   *character.Service
	growth    *character.GrowthService
	ownership *character.OwnershipService
}

func NewBaroCharacterHandler(service *character.Service, growth *character.GrowthService, ownership *character.OwnershipService) *BaroCharacterHandler {
	return &BaroCharacterHandler{service: service, growth: growth, ownership: ownership}
}

type characterChoiceRequest struct {
	CharacterID string `json:"character_id"`
}

func (h *BaroCharacterHandler) Selection(c *fiber.Ctx) error {
	claims, ok := c.Locals("user").(*middleware.Claims)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}
	selection, err := h.ownership.Selection(c.UserContext(), claims.UserID)
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, err.Error())
	}
	return utils.SendResponse(c, fiber.StatusOK, "Character selection retrieved", selection)
}

func (h *BaroCharacterHandler) AdminSelection(c *fiber.Ctx) error {
	selection, err := h.ownership.Selection(c.UserContext(), c.Params("id"))
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, err.Error())
	}
	return utils.SendResponse(c, fiber.StatusOK, "Character selection retrieved", selection)
}

func (h *BaroCharacterHandler) AdminCollection(c *fiber.Ctx) error {
	collection, err := h.service.Collection(c.UserContext(), c.Params("id"))
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, err.Error())
	}
	return utils.SendResponse(c, fiber.StatusOK, "Character collection retrieved", collection)
}

func (h *BaroCharacterHandler) AdminGrant(c *fiber.Ctx) error {
	granted, err := h.ownership.Grant(c.UserContext(), c.Params("id"))
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, err.Error())
	}
	return utils.SendResponse(c, fiber.StatusOK, "Character granted", granted)
}

func (h *BaroCharacterHandler) choose(c *fiber.Ctx, ownerID, slot string) error {
	var body characterChoiceRequest
	if err := c.BodyParser(&body); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}
	if slot == "equipped" {
		selection, err := h.ownership.Equip(c.UserContext(), ownerID, body.CharacterID)
		if err != nil {
			return utils.SendError(c, fiber.StatusBadRequest, err.Error())
		}
		return utils.SendResponse(c, fiber.StatusOK, "Character equipped", selection)
	}
	selection, err := h.ownership.Pin(c.UserContext(), ownerID, body.CharacterID)
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, err.Error())
	}
	return utils.SendResponse(c, fiber.StatusOK, "Character pin updated", selection)
}

func (h *BaroCharacterHandler) Equip(c *fiber.Ctx) error {
	claims, ok := c.Locals("user").(*middleware.Claims)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}
	return h.choose(c, claims.UserID, "equipped")
}

func (h *BaroCharacterHandler) Pin(c *fiber.Ctx) error {
	claims, ok := c.Locals("user").(*middleware.Claims)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}
	return h.choose(c, claims.UserID, "pinned")
}

func (h *BaroCharacterHandler) AdminEquip(c *fiber.Ctx) error {
	return h.choose(c, c.Params("id"), "equipped")
}
func (h *BaroCharacterHandler) AdminPin(c *fiber.Ctx) error {
	return h.choose(c, c.Params("id"), "pinned")
}

func (h *BaroCharacterHandler) Growth(c *fiber.Ctx) error {
	claims, ok := c.Locals("user").(*middleware.Claims)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}
	snapshot, err := h.growth.Snapshot(c.UserContext(), claims.UserID, time.Now())
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, err.Error())
	}
	return utils.SendResponse(c, fiber.StatusOK, "Character growth retrieved", snapshot)
}

func (h *BaroCharacterHandler) Collection(c *fiber.Ctx) error {
	claims, ok := c.Locals("user").(*middleware.Claims)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}
	characters, err := h.service.Collection(context.Background(), claims.UserID)
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, err.Error())
	}
	return utils.SendResponse(c, fiber.StatusOK, "Characters retrieved", characters)
}

func (h *BaroCharacterHandler) RevealStarter(c *fiber.Ctx) error {
	claims, ok := c.Locals("user").(*middleware.Claims)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}
	character, err := h.service.RevealStarter(context.Background(), claims.UserID)
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, err.Error())
	}
	return utils.SendResponse(c, fiber.StatusOK, "Starter character revealed", character)
}
