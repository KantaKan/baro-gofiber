package handler

import (
	"time"

	"gofiber-baro/internal/domain"
	"gofiber-baro/internal/service/attendance"
	"gofiber-baro/internal/service/user"
	"gofiber-baro/pkg/utils"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type AttendanceHandler struct {
	codeService       *attendance.CodeService
	submissionService *attendance.SubmissionService
	statsService      *attendance.StatsService
	overviewService   *attendance.OverviewService
	userService       *user.Service
}

func NewAttendanceHandler(
	codeService *attendance.CodeService,
	submissionService *attendance.SubmissionService,
	statsService *attendance.StatsService,
	overviewService *attendance.OverviewService,
	userService *user.Service,
) *AttendanceHandler {
	return &AttendanceHandler{
		codeService:       codeService,
		submissionService: submissionService,
		statsService:      statsService,
		overviewService:   overviewService,
		userService:       userService,
	}
}

func (h *AttendanceHandler) GenerateAttendanceCode(c *fiber.Ctx) error {
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

	session := domain.AttendanceSession(body.Session)
	if session != domain.SessionMorning && session != domain.SessionAfternoon {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid session. Use 'morning' or 'afternoon'")
	}

	adminID := c.Locals("userID")
	generatedBy := ""
	if id, ok := adminID.(string); ok {
		generatedBy = id
	}

	code, err := h.codeService.GenerateCode(body.Cohort, session, generatedBy)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error generating code")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Code generated successfully", code)
}

func (h *AttendanceHandler) GetActiveAttendanceCode(c *fiber.Ctx) error {
	cohort := c.QueryInt("cohort", 0)
	session := c.Query("session", "")

	if cohort == 0 || session == "" {
		return utils.SendError(c, fiber.StatusBadRequest, "Cohort and session are required")
	}

	code, err := h.codeService.GetActiveCode(cohort, domain.AttendanceSession(session))
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error fetching code")
	}

	if code == nil {
		return utils.SendResponse(c, fiber.StatusOK, "No active code", nil)
	}

	return utils.SendResponse(c, fiber.StatusOK, "Active code retrieved", code)
}

func (h *AttendanceHandler) SubmitAttendance(c *fiber.Ctx) error {
	userID := c.Locals("userID")
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

	record, err := h.codeService.SubmitAttendance(oid, body.Code, body.Cohort, ipAddress)
	if err != nil {
		switch err {
		case attendance.ErrCodeExpired:
			return utils.SendError(c, fiber.StatusGone, "Code expired. Please contact admin for a new code.")
		case attendance.ErrInvalidCode:
			return utils.SendError(c, fiber.StatusBadRequest, "Invalid code. Please check and try again.")
		case attendance.ErrNoActiveCode:
			return utils.SendError(c, fiber.StatusBadRequest, "No active code for this session. Please contact admin.")
		case attendance.ErrCodeForWrongCohort:
			return utils.SendError(c, fiber.StatusBadRequest, "This code is for a different cohort.")
		case attendance.ErrAlreadySubmitted:
			return utils.SendError(c, fiber.StatusConflict, "You have already submitted attendance for this session.")
		case attendance.ErrSessionLocked:
			return utils.SendError(c, fiber.StatusForbidden, "Attendance for this session has been locked. Contact admin.")
		default:
			return utils.SendError(c, fiber.StatusInternalServerError, err.Error())
		}
	}

	return utils.SendResponse(c, fiber.StatusOK, "Attendance submitted successfully", record)
}

func (h *AttendanceHandler) GetMyAttendanceStatus(c *fiber.Ctx) error {
	userID := c.Locals("userID")
	if userID == nil {
		return utils.SendError(c, fiber.StatusUnauthorized, "Unauthorized")
	}

	oid, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid user ID")
	}

	stats, err := h.submissionService.GetUserAttendanceStatus(oid)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error fetching attendance status")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Attendance status retrieved", stats)
}

func (h *AttendanceHandler) ManualMarkAttendance(c *fiber.Ctx) error {
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

	status := domain.AttendanceStatus(body.Status)
	validStatuses := []domain.AttendanceStatus{
		domain.StatusPresent, domain.StatusLate, domain.StatusAbsent,
		domain.StatusLateExcused, domain.StatusAbsentExcused,
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

	adminID := c.Locals("userID")
	markedBy := ""
	if id, ok := adminID.(string); ok {
		markedBy = id
	}

	record, err := h.submissionService.ManualMarkAttendance(oid, body.Date, body.Session, status, markedBy)
	if err != nil {
		if err == attendance.ErrStudentNotFound {
			return utils.SendError(c, fiber.StatusNotFound, "Student not found")
		}
		return utils.SendError(c, fiber.StatusInternalServerError, "Error marking attendance")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Attendance marked successfully", record)
}

func (h *AttendanceHandler) BulkMarkAttendance(c *fiber.Ctx) error {
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

	session := domain.AttendanceSession(body.Session)
	if session != domain.SessionMorning && session != domain.SessionAfternoon {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid session. Use 'morning' or 'afternoon'")
	}

	status := domain.AttendanceStatus(body.Status)
	validStatuses := []domain.AttendanceStatus{
		domain.StatusPresent, domain.StatusLate, domain.StatusAbsent,
		domain.StatusLateExcused, domain.StatusAbsentExcused,
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

	adminID := c.Locals("userID")
	markedBy := ""
	if id, ok := adminID.(string); ok {
		markedBy = id
	}

	records, err := h.submissionService.BulkMarkAttendance(userOIDs, body.Date, session, status, markedBy)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error marking attendance")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Attendance marked successfully", fiber.Map{
		"marked_count": len(records),
		"records":      records,
	})
}

func (h *AttendanceHandler) GetAttendanceLogs(c *fiber.Ctx) error {
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

	logs, total, err := h.submissionService.GetAttendanceLogs(cohort, date, page, limit)
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

func (h *AttendanceHandler) GetAttendanceStats(c *fiber.Ctx) error {
	cohort := c.QueryInt("cohort", 0)
	startDate := c.Query("start_date", "")
	endDate := c.Query("end_date", "")

	if startDate == "" || endDate == "" {
		endDate = time.Now().In(time.UTC).Format("2006-01-02")
		startDate = time.Now().In(time.UTC).AddDate(0, 0, -30).Format("2006-01-02")
	}

	stats, err := h.statsService.GetAttendanceStats(cohort, startDate, endDate)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error fetching stats")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Stats retrieved", stats)
}

func (h *AttendanceHandler) GetStudentAttendanceHistory(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return utils.SendError(c, fiber.StatusBadRequest, "Student ID is required")
	}

	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid student ID")
	}

	history, err := h.submissionService.GetStudentAttendanceHistory(oid)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error fetching history")
	}

	return utils.SendResponse(c, fiber.StatusOK, "History retrieved", history)
}

func (h *AttendanceHandler) GetTodayOverview(c *fiber.Ctx) error {
	cohort := c.QueryInt("cohort", 0)
	session := c.Query("session", "")
	date := c.Query("date", "")

	if cohort == 0 {
		return utils.SendError(c, fiber.StatusBadRequest, "Cohort is required")
	}

	var sess domain.AttendanceSession
	if session == "morning" {
		sess = domain.SessionMorning
	} else if session == "afternoon" {
		sess = domain.SessionAfternoon
	}

	overview, err := h.overviewService.GetAttendanceOverviewByDate(cohort, sess, date)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error fetching overview")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Overview retrieved", overview)
}

func (h *AttendanceHandler) LockSession(c *fiber.Ctx) error {
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

	err := h.submissionService.LockSession(body.Date, body.Session, 0, body.Locked)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error updating lock status")
	}

	status := "unlocked"
	if body.Locked {
		status = "locked"
	}

	return utils.SendResponse(c, fiber.StatusOK, "Attendance "+status+" successfully", nil)
}

func (h *AttendanceHandler) DeleteAttendanceRecord(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return utils.SendError(c, fiber.StatusBadRequest, "Record ID is required")
	}

	adminID := c.Locals("userID")
	deletedBy := ""
	if idStr, ok := adminID.(string); ok {
		deletedBy = idStr
	}

	record, err := h.submissionService.DeleteAttendanceRecord(id, deletedBy)
	if err != nil {
		if err == attendance.ErrRecordNotFound {
			return utils.SendError(c, fiber.StatusNotFound, "Attendance record not found")
		}
		return utils.SendError(c, fiber.StatusInternalServerError, "Error deleting record")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Attendance record deleted", record)
}

func (h *AttendanceHandler) GetAttendanceStatsByDays(c *fiber.Ctx) error {
	cohort := c.QueryInt("cohort", 0)
	days := c.QueryInt("days", 7)

	stats, err := h.statsService.GetAttendanceStatsWithFilter(cohort, days)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error fetching stats")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Stats retrieved", stats)
}

func (h *AttendanceHandler) GetDailyAttendanceStats(c *fiber.Ctx) error {
	cohort := c.QueryInt("cohort", 0)
	days := c.QueryInt("days", 7)

	stats, err := h.statsService.GetDailyAttendanceStats(cohort, days)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error fetching daily stats")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Daily stats retrieved", stats)
}

func (h *AttendanceHandler) GetMyAttendanceHistory(c *fiber.Ctx) error {
	userID := c.Locals("userID")
	if userID == nil {
		return utils.SendError(c, fiber.StatusUnauthorized, "Unauthorized")
	}

	oid, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid user ID")
	}

	history, err := h.submissionService.GetStudentAttendanceHistory(oid)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error fetching history")
	}

	return utils.SendResponse(c, fiber.StatusOK, "History retrieved", history)
}

func (h *AttendanceHandler) GetMyDailyStats(c *fiber.Ctx) error {
	userID := c.Locals("userID")
	if userID == nil {
		return utils.SendError(c, fiber.StatusUnauthorized, "Unauthorized")
	}

	days := c.QueryInt("days", 7)

	oid, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid user ID")
	}

	user, err := h.userService.GetUserByID(oid.Hex())
	if err != nil {
		return utils.SendError(c, fiber.StatusNotFound, "User not found")
	}

	stats, err := h.statsService.GetDailyAttendanceStats(user.CohortNumber, days)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error fetching daily stats")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Daily stats retrieved", stats)
}
