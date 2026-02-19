package controllers

import (
	"gofiber-baro/models"
	"gofiber-baro/services"
	"gofiber-baro/utils"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// GenerateCode generates a new attendance code
// @Summary Generate attendance code
// @Description Generate a new attendance code for a cohort and session
// @Tags attendance
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param cohort body integer true "Cohort number"
// @Param session body string true "Session (morning/afternoon)"
// @Success 200 {object} utils.StandardResponse "Code generated successfully"
// @Failure 400 {object} utils.StandardResponse "Invalid input"
// @Failure 500 {object} utils.StandardResponse "Error generating code"
// @Router /admin/attendance/generate-code [post]
func GenerateAttendanceCode(c *fiber.Ctx) error {
	type RequestBody struct {
		Cohort  int    `json:"cohort"`
		Session string `json:"session"`
	}

	var body RequestBody
	if err := c.BodyParser(&body); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if body.Cohort == 0 || body.Session == "" {
		return utils.SendError(c, fiber.StatusBadRequest, "Cohort and session are required")
	}

	session := models.AttendanceSession(body.Session)
	if session != models.SessionMorning && session != models.SessionAfternoon {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid session. Use 'morning' or 'afternoon'")
	}

	adminID := c.Locals("user_id")
	generatedBy := ""
	if id, ok := adminID.(string); ok {
		generatedBy = id
	}

	code, err := services.GenerateCode(body.Cohort, session, generatedBy)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error generating code")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Code generated successfully", code)
}

// GetActiveCode retrieves the active attendance code
// @Summary Get active attendance code
// @Description Get the currently active attendance code for a cohort and session
// @Tags attendance
// @Security BearerAuth
// @Produce json
// @Param cohort query int true "Cohort number"
// @Param session query string true "Session (morning/afternoon)"
// @Success 200 {object} utils.StandardResponse "Active code retrieved"
// @Router /admin/attendance/active-code [get]
func GetActiveAttendanceCode(c *fiber.Ctx) error {
	cohort := c.QueryInt("cohort", 0)
	session := c.Query("session", "")

	if cohort == 0 || session == "" {
		return utils.SendError(c, fiber.StatusBadRequest, "Cohort and session are required")
	}

	code, err := services.GetActiveCode(cohort, models.AttendanceSession(session))
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error fetching code")
	}

	if code == nil {
		return utils.SendResponse(c, fiber.StatusOK, "No active code", nil)
	}

	return utils.SendResponse(c, fiber.StatusOK, "Active code retrieved", code)
}

// SubmitAttendance allows a student to submit attendance
// @Summary Submit attendance
// @Description Student submits attendance using a code
// @Tags attendance
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param code body string true "Attendance code"
// @Success 200 {object} utils.StandardResponse "Attendance submitted"
// @Failure 400 {object} utils.StandardResponse "Invalid request"
// @Failure 401 {object} utils.StandardResponse "Unauthorized"
// @Failure 409 {object} utils.StandardResponse "Already submitted"
// @Failure 410 {object} utils.StandardResponse "Code expired"
// @Router /attendance/submit [post]
func SubmitAttendance(c *fiber.Ctx) error {
	userID := c.Locals("user_id")
	if userID == nil {
		return utils.SendError(c, fiber.StatusUnauthorized, "Unauthorized")
	}

	type RequestBody struct {
		Code   string `json:"code"`
		Cohort int    `json:"cohort"`
	}

	var body RequestBody
	if err := c.BodyParser(&body); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}

	oid, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid user ID")
	}

	ipAddress := c.IP()

	record, err := services.SubmitAttendance(oid, body.Code, body.Cohort, ipAddress)
	if err != nil {
		switch err {
		case services.ErrCodeExpired:
			return utils.SendError(c, fiber.StatusGone, "Code expired. Please contact admin for a new code.")
		case services.ErrInvalidCode:
			return utils.SendError(c, fiber.StatusBadRequest, "Invalid code. Please check and try again.")
		case services.ErrNoActiveCode:
			return utils.SendError(c, fiber.StatusBadRequest, "No active code for this session. Please contact admin.")
		case services.ErrCodeForWrongCohort:
			return utils.SendError(c, fiber.StatusBadRequest, "This code is for a different cohort.")
		case services.ErrAlreadySubmitted:
			return utils.SendError(c, fiber.StatusConflict, "You have already submitted attendance for this session.")
		case services.ErrSessionLocked:
			return utils.SendError(c, fiber.StatusForbidden, "Attendance for this session has been locked. Contact admin.")
		default:
			return utils.SendError(c, fiber.StatusInternalServerError, err.Error())
		}
	}

	return utils.SendResponse(c, fiber.StatusOK, "Attendance submitted successfully", record)
}

// GetMyAttendanceStatus returns the current user's attendance status
// @Summary Get my attendance status
// @Description Get current user's attendance statistics and warnings
// @Tags attendance
// @Security BearerAuth
// @Produce json
// @Success 200 {object} utils.StandardResponse "Attendance status retrieved"
// @Router /attendance/my-status [get]
func GetMyAttendanceStatus(c *fiber.Ctx) error {
	userID := c.Locals("user_id")
	if userID == nil {
		return utils.SendError(c, fiber.StatusUnauthorized, "Unauthorized")
	}

	oid, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid user ID")
	}

	stats, err := services.GetUserAttendanceStatus(oid)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error fetching attendance status")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Attendance status retrieved", stats)
}

// ManualMarkAttendance allows admin to manually mark attendance
// @Summary Manual mark attendance
// @Description Admin manually marks a student as present/late/absent
// @Tags attendance
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param user_id body string true "User ID"
// @Param date body string true "Date (YYYY-MM-DD)"
// @Param session body string true "Session (morning/afternoon)"
// @Param status body string true "Status (present/late/absent/late_excused/absent_excused)"
// @Success 200 {object} utils.StandardResponse "Attendance marked"
// @Failure 400 {object} utils.StandardResponse "Invalid request"
// @Router /admin/attendance/manual [post]
func ManualMarkAttendance(c *fiber.Ctx) error {
	type RequestBody struct {
		UserID  string `json:"user_id"`
		Date    string `json:"date"`
		Session string `json:"session"`
		Status  string `json:"status"`
	}

	var body RequestBody
	if err := c.BodyParser(&body); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if body.UserID == "" || body.Date == "" || body.Session == "" || body.Status == "" {
		return utils.SendError(c, fiber.StatusBadRequest, "All fields are required")
	}

	oid, err := primitive.ObjectIDFromHex(body.UserID)
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid user ID")
	}

	status := models.AttendanceStatus(body.Status)
	validStatuses := []models.AttendanceStatus{
		models.StatusPresent, models.StatusLate, models.StatusAbsent,
		models.StatusLateExcused, models.StatusAbsentExcused,
	}

	valid := false
	for _, s := range validStatuses {
		if status == s {
			valid = true
			break
		}
	}
	if !valid {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid status")
	}

	adminID := c.Locals("user_id")
	markedBy := ""
	if id, ok := adminID.(string); ok {
		markedBy = id
	}

	record, err := services.ManualMarkAttendance(oid, body.Date, body.Session, status, markedBy)
	if err != nil {
		if err == services.ErrStudentNotFound {
			return utils.SendError(c, fiber.StatusNotFound, "Student not found")
		}
		return utils.SendError(c, fiber.StatusInternalServerError, "Error marking attendance")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Attendance marked successfully", record)
}

// GetAttendanceLogs retrieves attendance logs
// @Summary Get attendance logs
// @Description Get attendance logs with filters
// @Tags attendance
// @Security BearerAuth
// @Produce json
// @Param cohort query int false "Cohort number"
// @Param date query string false "Date (YYYY-MM-DD)"
// @Param page query int false "Page number"
// @Param limit query int false "Items per page"
// @Success 200 {object} utils.StandardResponse "Logs retrieved"
// @Router /admin/attendance/logs [get]
func GetAttendanceLogs(c *fiber.Ctx) error {
	cohort := c.QueryInt("cohort", 0)
	date := c.Query("date", "")
	page := c.QueryInt("page", 1)
	if page < 1 {
		page = 1
	}
	limit := c.QueryInt("limit", 50)
	if limit < 1 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}

	logs, total, err := services.GetAttendanceLogs(cohort, date, page, limit)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error fetching logs")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Logs retrieved", fiber.Map{
		"logs":  logs,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

// GetAttendanceStats retrieves attendance statistics
// @Summary Get attendance stats
// @Description Get attendance statistics per student
// @Tags attendance
// @Security BearerAuth
// @Produce json
// @Param cohort query int false "Cohort number"
// @Success 200 {object} utils.StandardResponse "Stats retrieved"
// @Router /admin/attendance/stats [get]
func GetAttendanceStats(c *fiber.Ctx) error {
	cohort := c.QueryInt("cohort", 0)

	stats, err := services.GetAttendanceStats(cohort)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error fetching stats")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Stats retrieved", stats)
}

// GetStudentAttendanceHistory retrieves a student's attendance history
// @Summary Get student attendance history
// @Description Get detailed attendance history for a student
// @Tags attendance
// @Security BearerAuth
// @Produce json
// @Param id path string true "Student ID"
// @Success 200 {object} utils.StandardResponse "History retrieved"
// @Router /admin/attendance/student/{id} [get]
func GetStudentAttendanceHistory(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return utils.SendError(c, fiber.StatusBadRequest, "Student ID is required")
	}

	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid student ID")
	}

	history, err := services.GetStudentAttendanceHistory(oid)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error fetching history")
	}

	return utils.SendResponse(c, fiber.StatusOK, "History retrieved", history)
}

// GetTodayOverview retrieves today's attendance overview
// @Summary Get today's attendance overview
// @Description Get overview of today's attendance for a cohort
// @Tags attendance
// @Security BearerAuth
// @Produce json
// @Param cohort query int true "Cohort number"
// @Param session query string false "Session (morning/afternoon)"
// @Param date query string false "Date (YYYY-MM-DD, defaults to today)"
// @Success 200 {object} utils.StandardResponse "Overview retrieved"
// @Router /admin/attendance/today [get]
func GetTodayOverview(c *fiber.Ctx) error {
	cohort := c.QueryInt("cohort", 0)
	session := c.Query("session", "")
	date := c.Query("date", "")

	if cohort == 0 {
		return utils.SendError(c, fiber.StatusBadRequest, "Cohort is required")
	}

	var sess models.AttendanceSession
	if session == "morning" {
		sess = models.SessionMorning
	} else if session == "afternoon" {
		sess = models.SessionAfternoon
	}

	overview, err := services.GetAttendanceOverviewByDate(cohort, sess, date)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error fetching overview")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Overview retrieved", overview)
}

// LockSession locks/unlocks attendance for a session
// @Summary Lock/unlock attendance
// @Description Lock or unlock attendance for a specific date and session
// @Tags attendance
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param date body string true "Date (YYYY-MM-DD)"
// @Param session body string true "Session (morning/afternoon)"
// @Param locked body boolean true "Lock status"
// @Success 200 {object} utils.StandardResponse "Lock status updated"
// @Router /admin/attendance/lock [post]
func LockSession(c *fiber.Ctx) error {
	type RequestBody struct {
		Date    string `json:"date"`
		Session string `json:"session"`
		Locked  bool   `json:"locked"`
	}

	var body RequestBody
	if err := c.BodyParser(&body); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if body.Date == "" || body.Session == "" {
		return utils.SendError(c, fiber.StatusBadRequest, "Date and session are required")
	}

	err := services.LockSession(body.Date, body.Session, 0, body.Locked)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error updating lock status")
	}

	status := "unlocked"
	if body.Locked {
		status = "locked"
	}

	return utils.SendResponse(c, fiber.StatusOK, "Attendance "+status+" successfully", nil)
}

// DeleteAttendanceRecord deletes (soft delete) an attendance record
// @Summary Delete attendance record
// @Description Soft delete an attendance record
// @Tags attendance
// @Security BearerAuth
// @Produce json
// @Param id path string true "Record ID"
// @Success 200 {object} utils.StandardResponse "Record deleted"
// @Router /admin/attendance/:id [delete]
func DeleteAttendanceRecord(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return utils.SendError(c, fiber.StatusBadRequest, "Record ID is required")
	}

	adminID := c.Locals("user_id")
	deletedBy := ""
	if idStr, ok := adminID.(string); ok {
		deletedBy = idStr
	}

	record, err := services.DeleteAttendanceRecord(id, deletedBy)
	if err != nil {
		if err == services.ErrRecordNotFound {
			return utils.SendError(c, fiber.StatusNotFound, "Attendance record not found")
		}
		return utils.SendError(c, fiber.StatusInternalServerError, "Error deleting record")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Attendance record deleted", record)
}

// GetAttendanceStatsByDays retrieves attendance statistics for a date range
// @Summary Get attendance stats by days
// @Description Get attendance statistics for last N days
// @Tags attendance
// @Security BearerAuth
// @Produce json
// @Param cohort query int false "Cohort number"
// @Param days query int false "Number of days (default 7)"
// @Success 200 {object} utils.StandardResponse "Stats retrieved"
// @Router /admin/attendance/stats-by-days [get]
func GetAttendanceStatsByDays(c *fiber.Ctx) error {
	cohort := c.QueryInt("cohort", 0)
	days := c.QueryInt("days", 7)

	stats, err := services.GetAttendanceStatsWithFilter(cohort, days)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error fetching stats")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Stats retrieved", stats)
}

// GetDailyAttendanceStats retrieves daily attendance statistics
// @Summary Get daily attendance stats
// @Description Get attendance stats per day
// @Tags attendance
// @Security BearerAuth
// @Produce json
// @Param cohort query int false "Cohort number"
// @Param days query int false "Number of days (default 7)"
// @Success 200 {object} utils.StandardResponse "Daily stats retrieved"
// @Router /admin/attendance/daily-stats [get]
func GetDailyAttendanceStats(c *fiber.Ctx) error {
	cohort := c.QueryInt("cohort", 0)
	days := c.QueryInt("days", 7)

	stats, err := services.GetDailyAttendanceStats(cohort, days)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error fetching daily stats")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Daily stats retrieved", stats)
}

func BulkMarkAttendance(c *fiber.Ctx) error {
	type RequestBody struct {
		UserIDs []string `json:"user_ids"`
		Date    string   `json:"date"`
		Session string   `json:"session"`
		Status  string   `json:"status"`
	}

	var body RequestBody
	if err := c.BodyParser(&body); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if len(body.UserIDs) == 0 || body.Date == "" || body.Session == "" || body.Status == "" {
		return utils.SendError(c, fiber.StatusBadRequest, "User IDs, date, session, and status are required")
	}

	session := models.AttendanceSession(body.Session)
	if session != models.SessionMorning && session != models.SessionAfternoon {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid session. Use 'morning' or 'afternoon'")
	}

	status := models.AttendanceStatus(body.Status)
	validStatuses := []models.AttendanceStatus{
		models.StatusPresent, models.StatusLate, models.StatusAbsent,
		models.StatusLateExcused, models.StatusAbsentExcused,
	}

	valid := false
	for _, s := range validStatuses {
		if status == s {
			valid = true
			break
		}
	}
	if !valid {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid status")
	}

	var userOIDs []primitive.ObjectID
	for _, id := range body.UserIDs {
		oid, err := primitive.ObjectIDFromHex(id)
		if err != nil {
			continue
		}
		userOIDs = append(userOIDs, oid)
	}

	if len(userOIDs) == 0 {
		return utils.SendError(c, fiber.StatusBadRequest, "No valid user IDs provided")
	}

	adminID := c.Locals("user_id")
	markedBy := ""
	if id, ok := adminID.(string); ok {
		markedBy = id
	}

	records, err := services.BulkMarkAttendance(userOIDs, body.Date, session, status, markedBy)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error marking attendance")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Attendance marked successfully", fiber.Map{
		"marked_count": len(records),
		"records":      records,
	})
}
