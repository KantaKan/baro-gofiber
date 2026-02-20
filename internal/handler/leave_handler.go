package handler

import (
	"gofiber-baro/internal/domain"
	"gofiber-baro/internal/service/leave"
	"gofiber-baro/internal/service/user"
	"gofiber-baro/pkg/utils"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type LeaveHandler struct {
	leaveService *leave.Service
	userService  *user.Service
}

func NewLeaveHandler(leaveService *leave.Service, userService *user.Service) *LeaveHandler {
	return &LeaveHandler{
		leaveService: leaveService,
		userService:  userService,
	}
}

func (h *LeaveHandler) GetMyLeaveRequests(c *fiber.Ctx) error {
	userID := c.Locals("userID")
	if userID == nil {
		return utils.SendError(c, fiber.StatusUnauthorized, "Unauthorized")
	}

	oid, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid user ID")
	}

	requests, err := h.leaveService.GetMyLeaveRequests(oid)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error fetching leave requests")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Leave requests retrieved", requests)
}

func (h *LeaveHandler) CreateLeaveRequest(c *fiber.Ctx) error {
	userID := c.Locals("userID")
	if userID == nil {
		return utils.SendError(c, fiber.StatusUnauthorized, "Unauthorized")
	}

	type RequestBody struct {
		Type    string  `json:"type"`
		Session *string `json:"session"`
		Date    string  `json:"date"`
		Reason  string  `json:"reason"`
	}

	var body RequestBody
	if err := c.BodyParser(&body); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if body.Type == "" || body.Date == "" || body.Reason == "" {
		return utils.SendError(c, fiber.StatusBadRequest, "Type, date, and reason are required")
	}

	oid, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid user ID")
	}

	var session *domain.AttendanceSession
	if body.Session != nil {
		s := domain.AttendanceSession(*body.Session)
		session = &s
	}

	request, err := h.leaveService.CreateLeaveRequest(
		oid,
		domain.LeaveType(body.Type),
		session,
		body.Date,
		body.Reason,
		false,
		userID.(string),
	)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error creating leave request")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Leave request created", request)
}

func (h *LeaveHandler) GetAllLeaveRequests(c *fiber.Ctx) error {
	cohort := c.QueryInt("cohort", 0)
	status := c.Query("status", "")

	filter := domain.LeaveRequestFilter{
		Cohort: cohort,
	}

	if status != "" {
		filter.Status = domain.LeaveRequestStatus(status)
	}

	requests, err := h.leaveService.GetLeaveRequests(filter)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error fetching leave requests")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Leave requests retrieved", requests)
}

func (h *LeaveHandler) UpdateLeaveRequestStatus(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return utils.SendError(c, fiber.StatusBadRequest, "Leave request ID is required")
	}

	type RequestBody struct {
		Status      string `json:"status"`
		ReviewNotes string `json:"review_notes"`
	}

	var body RequestBody
	if err := c.BodyParser(&body); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if body.Status == "" {
		return utils.SendError(c, fiber.StatusBadRequest, "Status is required")
	}

	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid leave request ID")
	}

	userID := c.Locals("userID")
	userOID, _ := primitive.ObjectIDFromHex(userID.(string))

	user, err := h.getUserByID(userID.(string))
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error fetching user")
	}

	err = h.leaveService.UpdateLeaveRequestStatus(
		oid,
		domain.LeaveRequestStatus(body.Status),
		userOID,
		user.FirstName+" "+user.LastName,
		body.ReviewNotes,
	)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error updating leave request")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Leave request updated", nil)
}

func (h *LeaveHandler) CreateLeaveRequestAdmin(c *fiber.Ctx) error {
	type RequestBody struct {
		UserID  string  `json:"user_id"`
		Type    string  `json:"type"`
		Session *string `json:"session"`
		Date    string  `json:"date"`
		Reason  string  `json:"reason"`
	}

	var body RequestBody
	if err := c.BodyParser(&body); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if body.UserID == "" || body.Type == "" || body.Date == "" {
		return utils.SendError(c, fiber.StatusBadRequest, "User ID, type, and date are required")
	}

	userID, err := primitive.ObjectIDFromHex(body.UserID)
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid user ID")
	}

	adminID := c.Locals("userID").(string)

	var session *domain.AttendanceSession
	if body.Session != nil {
		s := domain.AttendanceSession(*body.Session)
		session = &s
	}

	reason := body.Reason
	if reason == "" {
		reason = "Created by admin"
	}

	request, err := h.leaveService.CreateLeaveRequest(
		userID,
		domain.LeaveType(body.Type),
		session,
		body.Date,
		reason,
		true,
		adminID,
	)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error creating leave request")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Leave request created", request)
}

func (h *LeaveHandler) getUserByID(id string) (*domain.User, error) {
	return h.userService.GetUserByID(id)
}
