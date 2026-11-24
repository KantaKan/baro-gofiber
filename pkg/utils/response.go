package utils

import (
	"errors"
	"log"

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

// ErrorHandler is the app-wide last stop for errors a handler returns instead of answering.
// Errors made with fiber.NewError keep their status and message (they're written for people);
// anything else is logged and the client only learns that something went wrong, so database
// and driver details never reach the browser.
func ErrorHandler(c *fiber.Ctx, err error) error {
	var fe *fiber.Error
	if errors.As(err, &fe) {
		return SendError(c, fe.Code, fe.Message)
	}
	log.Printf("unhandled error on %s %s: %v", c.Method(), c.Path(), err)
	return SendError(c, fiber.StatusInternalServerError, "Something went wrong on our side. Please try again.")
}
