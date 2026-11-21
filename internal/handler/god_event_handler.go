package handler

import (
	"errors"
	"time"

	"gofiber-baro/internal/service/godevent"
	"gofiber-baro/pkg/middleware"
	"gofiber-baro/pkg/utils"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
)

type GodEventHandler struct{ service *godevent.Service }

func NewGodEventHandler(service *godevent.Service) *GodEventHandler {
	return &GodEventHandler{service: service}
}

func GodEventCastLimiter() fiber.Handler {
	return limiter.New(limiter.Config{
		Max: 6, Expiration: time.Hour,
		KeyGenerator: func(c *fiber.Ctx) string {
			claims, ok := c.Locals("user").(*middleware.Claims)
			if !ok {
				return c.IP()
			}
			return claims.UserID
		},
		LimitReached: func(c *fiber.Ctx) error {
			return utils.SendError(c, fiber.StatusTooManyRequests, "GOD event limit reached; try again later")
		},
	})
}

type castGodEventRequest struct {
	Preset  string `json:"preset"`
	Caption string `json:"caption"`
	Cohort  int    `json:"cohort"`
}

func (h *GodEventHandler) Cast(c *fiber.Ctx) error {
	claims, ok := c.Locals("user").(*middleware.Claims)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}
	var body castGodEventRequest
	if err := c.BodyParser(&body); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}
	event, err := h.service.Cast(c.UserContext(), claims.UserID, body.Preset, body.Caption, body.Cohort)
	if errors.Is(err, godevent.ErrAdminRequired) {
		return utils.SendError(c, fiber.StatusForbidden, err.Error())
	}
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, err.Error())
	}
	return utils.SendResponse(c, fiber.StatusCreated, "GOD event cast", event)
}

func (h *GodEventHandler) List(c *fiber.Ctx) error {
	claims, ok := c.Locals("user").(*middleware.Claims)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}
	events, err := h.service.List(c.UserContext(), claims.UserID)
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, err.Error())
	}
	return utils.SendResponse(c, fiber.StatusOK, "GOD event history retrieved", events)
}
