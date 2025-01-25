// Package docs provides API documentation for the Barometer application
package docs

import (
	"gofiber-baro/models"
	"gofiber-baro/services"
	"time"
)

// LoginRequest represents the login request payload
// swagger:model LoginRequest
type LoginRequest struct {
	Email    string `json:"email" example:"user@example.com"`
	Password string `json:"password" example:"password123"`
}

// UserResponse represents the user response
// swagger:model UserResponse
type UserResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    models.User `json:"data"`
}

// TokenResponse represents the login response
// swagger:model TokenResponse
type TokenResponse struct {
	Token string `json:"token"`
	Role  string `json:"role"`
}

// ErrorResponse represents the error response
// swagger:model ErrorResponse
type ErrorResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// ReflectionRequest represents a new reflection request
// swagger:model ReflectionRequest
type ReflectionRequest struct {
	Barometer string    `json:"barometer" example:"Comfort Zone"`
	Text      string    `json:"text" example:"Today was productive"`
	Date      time.Time `json:"date"`
}

// BarometerData represents barometer statistics
// swagger:model BarometerData
type BarometerData struct {
	Date                             string `json:"date"`
	ComfortZone                      int    `json:"Comfort Zone"`
	PanicZone                        int    `json:"Panic Zone"`
	StretchZoneEnjoyingTheChallenges int    `json:"Stretch Zone - Enjoying the Challenges"`
	StretchZoneOverwhelmed           int    `json:"Stretch Zone - Overwhelmed"`
}

// BarometerResponse represents the barometer response
// swagger:model BarometerResponse
type BarometerResponse struct {
	Success bool                   `json:"success"`
	Message string                `json:"message"`
	Data    services.BarometerData `json:"data"`
}

// ReflectionWithUser represents the reflection with user
// swagger:model ReflectionWithUser
type ReflectionWithUser struct {
	User        models.User     `json:"user"`
	FirstName   string         `json:"first_name"`
	LastName    string         `json:"last_name"`
	JSDNumber   string         `json:"jsd_number"`
	Date        time.Time      `json:"date"`
	Reflection  models.Reflection `json:"reflection"`
}

// ChartDataPoint represents the chart data point
// swagger:model ChartDataPoint
type ChartDataPoint struct {
	Day             string `json:"day"`
	DailyReflection int    `json:"DailyReflection"`
}

// Response represents the response
// swagger:model Response
type Response struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
	Total   int        `json:"total,omitempty"`
} 