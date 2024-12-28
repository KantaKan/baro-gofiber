// utils/response.go
package utils

import (
	"github.com/gofiber/fiber/v2"
)

// StandardResponse is a structure for standardizing API responses
type StandardResponse struct {
	Status  string      `json:"status"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// SendResponse sends a standard response to the client
func SendResponse(c *fiber.Ctx, statusCode int, message string, data interface{}) error {
	response := StandardResponse{
		Status:  "success",
		Message: message,
		Data:    data,
	}

	if statusCode >= 400 {
		response.Status = "error"
	}

	return c.Status(statusCode).JSON(response)
}

// SendError sends an error response with a custom message
func SendError(c *fiber.Ctx, statusCode int, message string) error {
	response := StandardResponse{
		Status:  "error",
		Message: message,
	}

	return c.Status(statusCode).JSON(response)
}
