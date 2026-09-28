package handler

import (
	"gofiber-baro/internal/service/user"
	"gofiber-baro/pkg/middleware"
	"gofiber-baro/pkg/utils"

	"github.com/gofiber/fiber/v2"
)

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
