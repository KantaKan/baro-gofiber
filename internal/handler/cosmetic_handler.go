package handler

import (
	"gofiber-baro/internal/service/user"
	"gofiber-baro/pkg/middleware"
	"gofiber-baro/pkg/utils"
	"strings"

	"github.com/gofiber/fiber/v2"
)

type adminCosmeticGrantRequest struct {
	Message string `json:"message"`
}

type equipCosmeticRequest struct {
	CosmeticID string `json:"cosmetic_id"`
}

type CosmeticHandler struct {
	service *user.CosmeticService
}

func NewCosmeticHandler(service *user.CosmeticService) *CosmeticHandler {
	return &CosmeticHandler{service: service}
}

func (h *CosmeticHandler) GetCatalog(c *fiber.Ctx) error {
	return utils.SendResponse(c, fiber.StatusOK, "Cosmetic catalog retrieved", h.service.Catalog())
}

func (h *CosmeticHandler) GetCollection(c *fiber.Ctx) error {
	claims, ok := c.Locals("user").(*middleware.Claims)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}
	collection, err := h.service.Collection(claims.UserID)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Could not load cosmetic collection")
	}
	return utils.SendResponse(c, fiber.StatusOK, "Cosmetic collection retrieved", collection)
}

func (h *CosmeticHandler) GetAdminCollection(c *fiber.Ctx) error {
	collection, err := h.service.Collection(c.Params("id"))
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, err.Error())
	}
	return utils.SendResponse(c, fiber.StatusOK, "Learner cosmetic collection retrieved", collection)
}

func (h *CosmeticHandler) GrantCosmetic(c *fiber.Ctx) error {
	var body adminCosmeticGrantRequest
	if err := c.BodyParser(&body); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}
	body.Message = strings.TrimSpace(body.Message)
	cosmeticID := strings.TrimSpace(c.Params("cosmeticId"))
	if cosmeticID == "" {
		return utils.SendError(c, fiber.StatusBadRequest, "Cosmetic ID is required")
	}
	if len(body.Message) > 500 {
		return utils.SendError(c, fiber.StatusBadRequest, "Message must be 500 characters or fewer")
	}
	granted, err := h.service.Grant(c.Params("id"), cosmeticID, body.Message)
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, err.Error())
	}
	return utils.SendResponse(c, fiber.StatusOK, "Cosmetic grant processed", fiber.Map{"granted": granted})
}

func (h *CosmeticHandler) RevokeCosmetic(c *fiber.Ctx) error {
	revoked, err := h.service.Revoke(c.Params("id"), c.Params("cosmeticId"))
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, err.Error())
	}
	return utils.SendResponse(c, fiber.StatusOK, "Cosmetic revocation processed", fiber.Map{"revoked": revoked})
}

func (h *CosmeticHandler) EquipCosmetic(c *fiber.Ctx) error {
	claims, ok := c.Locals("user").(*middleware.Claims)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}
	var body equipCosmeticRequest
	if err := c.BodyParser(&body); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}
	if err := h.service.Equip(claims.UserID, c.Params("slot"), strings.TrimSpace(body.CosmeticID)); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, err.Error())
	}
	return utils.SendResponse(c, fiber.StatusOK, "Cosmetic equipped", nil)
}

func (h *CosmeticHandler) UnequipCosmetic(c *fiber.Ctx) error {
	claims, ok := c.Locals("user").(*middleware.Claims)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}
	if err := h.service.Unequip(claims.UserID, c.Params("slot")); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, err.Error())
	}
	return utils.SendResponse(c, fiber.StatusOK, "Cosmetic unequipped", nil)
}
