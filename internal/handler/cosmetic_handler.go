package handler

import (
	"errors"
	"net/url"
	"strings"

	"gofiber-baro/internal/domain"
	"gofiber-baro/internal/service/user"
	"gofiber-baro/pkg/middleware"
	"gofiber-baro/pkg/utils"

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

func (h *CosmeticHandler) GetCharacterCatalog(c *fiber.Ctx) error {
	return utils.SendResponse(c, fiber.StatusOK, "Character cosmetic catalog retrieved", h.service.CharacterCatalog())
}

func (h *CosmeticHandler) GetCollection(c *fiber.Ctx) error {
	return h.getCollection(c, false)
}

func (h *CosmeticHandler) GetCharacterCollection(c *fiber.Ctx) error {
	return h.getCollection(c, true)
}

func (h *CosmeticHandler) getCollection(c *fiber.Ctx, character bool) error {
	claims, ok := c.Locals("user").(*middleware.Claims)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}
	var collection *domain.CosmeticCollection
	var err error
	if character {
		collection, err = h.service.CharacterCollection(claims.UserID)
	} else {
		collection, err = h.service.Collection(claims.UserID)
	}
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

func (h *CosmeticHandler) GetAdminCharacterCollection(c *fiber.Ctx) error {
	collection, err := h.service.CharacterCollection(c.Params("id"))
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, err.Error())
	}
	return utils.SendResponse(c, fiber.StatusOK, "Character cosmetic collection retrieved", collection)
}

func (h *CosmeticHandler) GrantCosmetic(c *fiber.Ctx) error {
	return h.grantCosmetic(c, false)
}

func (h *CosmeticHandler) GrantCharacterCosmetic(c *fiber.Ctx) error {
	return h.grantCosmetic(c, true)
}

func cosmeticIDParam(c *fiber.Ctx) (string, error) {
	cosmeticID, err := url.PathUnescape(c.Params("cosmeticId"))
	if err != nil {
		return "", errors.New("invalid cosmetic ID")
	}
	cosmeticID = strings.TrimSpace(cosmeticID)
	if cosmeticID == "" {
		return "", errors.New("cosmetic ID is required")
	}
	return cosmeticID, nil
}

func (h *CosmeticHandler) grantCosmetic(c *fiber.Ctx, character bool) error {
	var body adminCosmeticGrantRequest
	if err := c.BodyParser(&body); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}
	body.Message = strings.TrimSpace(body.Message)
	cosmeticID, err := cosmeticIDParam(c)
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, err.Error())
	}
	if len(body.Message) > 500 {
		return utils.SendError(c, fiber.StatusBadRequest, "Message must be 500 characters or fewer")
	}
	var granted bool
	if character {
		granted, err = h.service.GrantCharacter(c.Params("id"), cosmeticID, body.Message)
	} else {
		granted, err = h.service.Grant(c.Params("id"), cosmeticID, body.Message)
	}
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, err.Error())
	}
	return utils.SendResponse(c, fiber.StatusOK, "Cosmetic grant processed", fiber.Map{"granted": granted})
}

func (h *CosmeticHandler) RevokeCosmetic(c *fiber.Ctx) error {
	cosmeticID, err := cosmeticIDParam(c)
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, err.Error())
	}
	revoked, err := h.service.Revoke(c.Params("id"), cosmeticID)
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, err.Error())
	}
	return utils.SendResponse(c, fiber.StatusOK, "Cosmetic revocation processed", fiber.Map{"revoked": revoked})
}

func (h *CosmeticHandler) EquipCosmetic(c *fiber.Ctx) error {
	return h.equipCosmetic(c, false)
}

func (h *CosmeticHandler) EquipCharacterCosmetic(c *fiber.Ctx) error {
	return h.equipCosmetic(c, true)
}

func (h *CosmeticHandler) equipCosmetic(c *fiber.Ctx, character bool) error {
	claims, ok := c.Locals("user").(*middleware.Claims)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}
	var body equipCosmeticRequest
	if err := c.BodyParser(&body); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}
	var err error
	if character {
		err = h.service.EquipCharacter(claims.UserID, c.Params("slot"), strings.TrimSpace(body.CosmeticID))
	} else {
		err = h.service.Equip(claims.UserID, c.Params("slot"), strings.TrimSpace(body.CosmeticID))
	}
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, err.Error())
	}
	return utils.SendResponse(c, fiber.StatusOK, "Cosmetic equipped", nil)
}

func (h *CosmeticHandler) UnequipCosmetic(c *fiber.Ctx) error {
	return h.unequipCosmetic(c, false)
}

func (h *CosmeticHandler) UnequipCharacterCosmetic(c *fiber.Ctx) error {
	return h.unequipCosmetic(c, true)
}

func (h *CosmeticHandler) unequipCosmetic(c *fiber.Ctx, character bool) error {
	claims, ok := c.Locals("user").(*middleware.Claims)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}
	var err error
	if character {
		err = h.service.UnequipCharacter(claims.UserID, c.Params("slot"))
	} else {
		err = h.service.Unequip(claims.UserID, c.Params("slot"))
	}
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, err.Error())
	}
	return utils.SendResponse(c, fiber.StatusOK, "Cosmetic unequipped", nil)
}
