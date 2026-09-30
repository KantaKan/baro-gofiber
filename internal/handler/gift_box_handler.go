package handler

import (
	"context"
	"errors"
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
	RewardPool    string `json:"reward_pool"`
}

type grantCohortGiftBoxesRequest struct {
	MinimumRarity  string `json:"minimum_rarity"`
	Message        string `json:"message"`
	IdempotencyKey string `json:"idempotency_key"`
	Team           string `json:"team"`
	RewardPool     string `json:"reward_pool"`
}

func (h *GiftBoxHandler) PreviewAudience(c *fiber.Ctx) error {
	cohort, err := c.ParamsInt("cohortNumber")
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid cohort number")
	}
	result, err := h.service.PreviewAudience(c.UserContext(), cohort, c.Query("team"))
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, err.Error())
	}
	return utils.SendResponse(c, fiber.StatusOK, "Gift box audience preview", result)
}

type transferGiftBoxRequest struct {
	RecipientID string `json:"recipient_id"`
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
	body.RewardPool = strings.TrimSpace(body.RewardPool)
	if !validMinimumRarity(body.MinimumRarity) || body.Message == "" || len(body.Message) > 500 || (body.RewardPool != "" && body.RewardPool != "character-box" && body.RewardPool != giftbox.CharacterEggPool) {
		return utils.SendError(c, fiber.StatusBadRequest, "A valid rarity and message of 500 characters or fewer are required")
	}
	box, err := h.service.GrantWithPool(context.Background(), c.Params("id"), claims.UserID, body.MinimumRarity, body.Message, body.RewardPool)
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, err.Error())
	}
	return utils.SendResponse(c, fiber.StatusCreated, "Gift box sent", box)
}

func (h *GiftBoxHandler) GrantCohort(c *fiber.Ctx) error {
	claims, ok := c.Locals("user").(*middleware.Claims)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}
	var body grantCohortGiftBoxesRequest
	if err := c.BodyParser(&body); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}
	body.MinimumRarity = strings.TrimSpace(body.MinimumRarity)
	body.Message = strings.TrimSpace(body.Message)
	body.IdempotencyKey = strings.TrimSpace(body.IdempotencyKey)
	body.Team = strings.TrimSpace(body.Team)
	body.RewardPool = strings.TrimSpace(body.RewardPool)
	if !validMinimumRarity(body.MinimumRarity) || body.Message == "" || len(body.Message) > 500 || body.IdempotencyKey == "" {
		return utils.SendError(c, fiber.StatusBadRequest, "A valid rarity, message, and idempotency key are required")
	}
	cohort, err := c.ParamsInt("cohortNumber")
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid cohort number")
	}
	result, err := h.service.GrantAudience(c.UserContext(), cohort, body.Team, claims.UserID, body.MinimumRarity, body.Message, body.IdempotencyKey, body.RewardPool)
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, err.Error())
	}
	return utils.SendResponse(c, fiber.StatusOK, "Cohort gift boxes processed", result)
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

func (h *GiftBoxHandler) Odds(c *fiber.Ctx) error {
	claims, ok := c.Locals("user").(*middleware.Claims)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}
	result, err := h.service.Odds(context.Background(), claims.UserID, c.Params("id"))
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, err.Error())
	}
	return utils.SendResponse(c, fiber.StatusOK, "Gift box odds retrieved", result)
}

func (h *GiftBoxHandler) SearchRecipients(c *fiber.Ctx) error {
	claims, ok := c.Locals("user").(*middleware.Claims)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}
	items, err := h.service.SearchRecipients(context.Background(), claims.UserID, c.Query("query"))
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, err.Error())
	}
	return utils.SendResponse(c, fiber.StatusOK, "Gift box recipients retrieved", items)
}

func (h *GiftBoxHandler) Transfer(c *fiber.Ctx) error {
	claims, ok := c.Locals("user").(*middleware.Claims)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}
	var body transferGiftBoxRequest
	if err := c.BodyParser(&body); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}
	box, err := h.service.Transfer(context.Background(), claims.UserID, c.Params("id"), strings.TrimSpace(body.RecipientID))
	if err != nil {
		if errors.Is(err, giftbox.ErrBoxNotFound) || errors.Is(err, giftbox.ErrRecipientNotFound) {
			return utils.SendError(c, fiber.StatusNotFound, err.Error())
		}
		return utils.SendError(c, fiber.StatusBadRequest, err.Error())
	}
	return utils.SendResponse(c, fiber.StatusOK, "Gift box transferred", box)
}

func validMinimumRarity(rarity string) bool {
	for _, candidate := range []string{"Common", "Rare", "Epic", "Legendary"} {
		if candidate == rarity {
			return true
		}
	}
	return false
}
