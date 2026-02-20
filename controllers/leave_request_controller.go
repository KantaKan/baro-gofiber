package controllers

import (
	"gofiber-baro/models"
	"gofiber-baro/services"
	"gofiber-baro/utils"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func CreateLeaveRequest(c *fiber.Ctx) error {
	userID := c.Locals("userID")
	if userID == nil {
		return utils.SendError(c, fiber.StatusUnauthorized, "Unauthorized")
	}

	type RequestBody struct {
		Type    string `json:"type"`
		Date    string `json:"date"`
		Session string `json:"session"`
		Reason  string `json:"reason"`
	}

	var body RequestBody
	if err := c.BodyParser(&body); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if body.Type == "" || body.Date == "" || body.Reason == "" {
		return utils.SendError(c, fiber.StatusBadRequest, "Type, date, and reason are required")
	}

	leaveType := models.LeaveType(body.Type)
	if leaveType != models.LeaveTypeLate && leaveType != models.LeaveTypeHalfDay && leaveType != models.LeaveTypeFullDay {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid leave type. Use 'late', 'half_day', or 'full_day'")
	}

	var session *models.AttendanceSession
	if body.Session != "" {
		s := models.AttendanceSession(body.Session)
		if s != models.SessionMorning && s != models.SessionAfternoon {
			return utils.SendError(c, fiber.StatusBadRequest, "Invalid session. Use 'morning' or 'afternoon'")
		}
		session = &s
	}

	oid, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid user ID")
	}

	request, err := services.CreateLeaveRequest(oid, leaveType, body.Date, session, body.Reason)
	if err != nil {
		if err == services.ErrStudentNotFound {
			return utils.SendError(c, fiber.StatusNotFound, "Student not found")
		}
		if err == services.ErrInvalidSession {
			return utils.SendError(c, fiber.StatusBadRequest, "Session is required for half_day leave type")
		}
		return utils.SendError(c, fiber.StatusInternalServerError, "Failed to create leave request")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Leave request submitted successfully", request)
}

func GetMyLeaveRequests(c *fiber.Ctx) error {
	userID := c.Locals("userID")
	if userID == nil {
		return utils.SendError(c, fiber.StatusUnauthorized, "Unauthorized")
	}

	oid, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid user ID")
	}

	requests, err := services.GetMyLeaveRequests(oid)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Failed to fetch leave requests")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Leave requests retrieved", requests)
}

func GetAllLeaveRequests(c *fiber.Ctx) error {
	cohort := c.QueryInt("cohort", 0)
	status := c.Query("status", "")
	fromDate := c.Query("from_date", "")
	toDate := c.Query("to_date", "")

	requests, err := services.GetAllLeaveRequests(cohort, status, fromDate, toDate)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Failed to fetch leave requests")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Leave requests retrieved", requests)
}

func AdminCreateLeaveRequest(c *fiber.Ctx) error {
	type RequestBody struct {
		UserID  string `json:"user_id"`
		Type    string `json:"type"`
		Date    string `json:"date"`
		Session string `json:"session"`
		Reason  string `json:"reason"`
	}

	var body RequestBody
	if err := c.BodyParser(&body); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if body.UserID == "" || body.Type == "" || body.Date == "" || body.Reason == "" {
		return utils.SendError(c, fiber.StatusBadRequest, "User ID, type, date, and reason are required")
	}

	leaveType := models.LeaveType(body.Type)
	if leaveType != models.LeaveTypeLate && leaveType != models.LeaveTypeHalfDay && leaveType != models.LeaveTypeFullDay {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid leave type")
	}

	var session *models.AttendanceSession
	if body.Session != "" {
		s := models.AttendanceSession(body.Session)
		if s != models.SessionMorning && s != models.SessionAfternoon {
			return utils.SendError(c, fiber.StatusBadRequest, "Invalid session")
		}
		session = &s
	}

	userOID, err := primitive.ObjectIDFromHex(body.UserID)
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid user ID")
	}

	adminID := c.Locals("userID")
	adminOID, _ := primitive.ObjectIDFromHex(adminID.(string))

	adminName := "Admin"
	if name, ok := c.Locals("user_name").(string); ok && name != "" {
		adminName = name
	}

	request, err := services.AdminCreateLeaveRequest(userOID, leaveType, body.Date, session, body.Reason, adminOID, adminName)
	if err != nil {
		if err == services.ErrStudentNotFound {
			return utils.SendError(c, fiber.StatusNotFound, "Student not found")
		}
		if err == services.ErrInvalidSession {
			return utils.SendError(c, fiber.StatusBadRequest, "Session is required for half_day leave type")
		}
		return utils.SendError(c, fiber.StatusInternalServerError, "Failed to create leave request")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Leave request created and approved", request)
}

func UpdateLeaveRequestStatus(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return utils.SendError(c, fiber.StatusBadRequest, "Leave request ID is required")
	}

	requestOID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid leave request ID")
	}

	type RequestBody struct {
		Status      string `json:"status"`
		ReviewNotes string `json:"review_notes"`
	}

	var body RequestBody
	if err := c.BodyParser(&body); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if body.Status != "approved" && body.Status != "rejected" {
		return utils.SendError(c, fiber.StatusBadRequest, "Status must be 'approved' or 'rejected'")
	}

	status := models.LeaveRequestStatus(body.Status)

	adminID := c.Locals("userID")
	adminOID, _ := primitive.ObjectIDFromHex(adminID.(string))

	adminName := "Admin"
	if name, ok := c.Locals("user_name").(string); ok && name != "" {
		adminName = name
	}

	request, err := services.UpdateLeaveRequestStatus(requestOID, status, body.ReviewNotes, adminOID, adminName)
	if err != nil {
		if err == services.ErrLeaveRequestNotFound {
			return utils.SendError(c, fiber.StatusNotFound, "Leave request not found")
		}
		return utils.SendError(c, fiber.StatusInternalServerError, "Failed to update leave request")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Leave request updated", request)
}

func GetMyAttendanceHistory(c *fiber.Ctx) error {
	userID := c.Locals("userID")
	if userID == nil {
		return utils.SendError(c, fiber.StatusUnauthorized, "Unauthorized")
	}

	oid, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid user ID")
	}

	days := c.QueryInt("days", 0)

	records, err := services.GetStudentAttendanceHistoryWithDays(oid, days)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Failed to fetch attendance history")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Attendance history retrieved", records)
}

func GetMyDailyStats(c *fiber.Ctx) error {
	userID := c.Locals("userID")
	if userID == nil {
		return utils.SendError(c, fiber.StatusUnauthorized, "Unauthorized")
	}

	oid, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid user ID")
	}

	days := c.QueryInt("days", 7)

	stats, err := services.GetStudentDailyStats(oid, days)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Failed to fetch daily stats")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Daily stats retrieved", stats)
}
