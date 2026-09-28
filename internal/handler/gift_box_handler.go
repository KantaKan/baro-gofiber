package handler

import (
	"context"
	"strings"

	"gofiber-baro/internal/service/giftbox"
	"gofiber-baro/pkg/middleware"
	"gofiber-baro/pkg/utils"

	"github.com/gofiber/fiber/v2"
)

type GiftBoxHandler struct {
	service *giftbox.Service
}

type grantGiftBoxRequest struct {
	MinimumRarity string `json:"minimum_rarity"`
	Message       string `json:"message"`
}

func NewGiftBoxHandler(service *giftbox.Service) *GiftBoxHandler {
	return &GiftBoxHandler{service: service}
}

func (h *GiftBoxHandler) Grant(c *fiber.Ctx) error {
	claims, ok := c.Locals("user").(*middleware.Claims)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}
	var body grantGiftBoxRequest
	if err := c.BodyParser(&body); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}
	body.MinimumRarity = strings.TrimSpace(body.MinimumRarity)
	body.Message = strings.TrimSpace(body.Message)
	if !validMinimumRarity(body.MinimumRarity) || body.Message == "" || len(body.Message) > 500 {
		return utils.SendError(c, fiber.StatusBadRequest, "A valid rarity and message of 500 characters or fewer are required")
	}
	box, err := h.service.Grant(context.Background(), c.Params("id"), claims.UserID, body.MinimumRarity, body.Message)
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, err.Error())
	}
	return utils.SendResponse(c, fiber.StatusCreated, "Gift box sent", box)
}

func (h *GiftBoxHandler) List(c *fiber.Ctx) error {
	claims, ok := c.Locals("user").(*middleware.Claims)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}
	boxes, err := h.service.List(context.Background(), claims.UserID)
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, err.Error())
	}
	return utils.SendResponse(c, fiber.StatusOK, "Gift boxes retrieved", boxes)
}

func (h *GiftBoxHandler) Open(c *fiber.Ctx) error {
	claims, ok := c.Locals("user").(*middleware.Claims)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}
	result, err := h.service.Open(context.Background(), claims.UserID, c.Params("id"))
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, err.Error())
	}
	return utils.SendResponse(c, fiber.StatusOK, "Gift box opened", result)
}

func validMinimumRarity(rarity string) bool {
	for _, candidate := range []string{"Common", "Rare", "Epic", "Legendary"} {
		if candidate == rarity {
			return true
		}
	}
	return false
}
