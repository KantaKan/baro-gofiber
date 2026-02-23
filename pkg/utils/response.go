package utils

import (
	"github.com/gofiber/fiber/v2"
)

type StandardResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func SendResponse(c *fiber.Ctx, statusCode int, message string, data interface{}) error {
	response := StandardResponse{
		Success: true,
		Message: message,
		Data:    data,
	}

	if statusCode >= 400 {
		response.Success = false
	}

	return c.Status(statusCode).JSON(response)
}

func SendError(c *fiber.Ctx, statusCode int, message string) error {
	response := StandardResponse{
		Success: false,
		Message: message,
	}

	return c.Status(statusCode).JSON(response)
}
