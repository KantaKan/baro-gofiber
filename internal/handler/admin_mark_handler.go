package handler

import (
	"context"
	"time"

	"gofiber-baro/internal/domain"
	"gofiber-baro/pkg/utils"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type manualMarker interface {
	ManualMarkAttendance(userID primitive.ObjectID, date, session string, status domain.AttendanceStatus, markedBy string) (*domain.AttendanceRecord, error)
}

type markRecordStore interface {
	FindRecords(ctx interface{}, filter domain.AttendanceRecordFilter, opts interface{}) ([]domain.AttendanceRecord, error)
	UpdateRecord(ctx interface{}, id primitive.ObjectID, update interface{}) error
}

// AdminMarkHandler marks one session and lets the admin undo it exactly (server-side inverse).
type AdminMarkHandler struct {
	marker  manualMarker
	records markRecordStore
}

func NewAdminMarkHandler(m manualMarker, r markRecordStore) *AdminMarkHandler {
	return &AdminMarkHandler{marker: m, records: r}
}

// PreviousRecord is the state a mark replaced, enough to restore it.
type PreviousRecord struct {
	ID           string                  `json:"_id"`
	Status       domain.AttendanceStatus `json:"status"`
	MarkedBy     domain.MarkedBy         `json:"marked_by"`
	MarkedByUser string                  `json:"marked_by_user"`
	SubmittedAt  time.Time               `json:"submitted_at"`
	CheckinStyle string                  `json:"checkin_style"`
}

var validStatus = map[domain.AttendanceStatus]bool{
	domain.StatusPresent: true, domain.StatusLate: true, domain.StatusAbsent: true,
	domain.StatusLateExcused: true, domain.StatusAbsentExcused: true,
}

// Mark sets one learner's session and returns the record it replaced (or null).
// @Summary      Mark a session (with undo info)
// @Tags         admin
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Failure      400  {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/attendance/mark [post]
func (h *AdminMarkHandler) Mark(c *fiber.Ctx) error {
	var b struct {
		UserID  string `json:"user_id"`
		Date    string `json:"date"`
		Session string `json:"session"`
		Status  string `json:"status"`
	}
	if err := c.BodyParser(&b); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}
	uid, err := primitive.ObjectIDFromHex(b.UserID)
	st := domain.AttendanceStatus(b.Status)
	if err != nil || b.Date == "" || (b.Session != "morning" && b.Session != "afternoon") || !validStatus[st] {
		return utils.SendError(c, fiber.StatusBadRequest, "user_id, date, session and a valid status are required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	prevRecs, err := h.records.FindRecords(ctx, domain.AttendanceRecordFilter{UserID: uid, Date: b.Date, Session: domain.AttendanceSession(b.Session), NotDeleted: true}, nil)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error reading attendance")
	}
	var prev *PreviousRecord
	if len(prevRecs) > 0 {
		p := prevRecs[0]
		prev = &PreviousRecord{ID: p.ID.Hex(), Status: p.Status, MarkedBy: p.MarkedBy, MarkedByUser: p.MarkedByUser, SubmittedAt: p.SubmittedAt, CheckinStyle: p.CheckinStyle}
	}
	by, _ := c.Locals("userID").(string)
	rec, err := h.marker.ManualMarkAttendance(uid, b.Date, b.Session, st, by)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error marking attendance")
	}
	return utils.SendResponse(c, fiber.StatusOK, "Attendance marked", fiber.Map{"record": rec, "previous": prev})
}

// Restore puts a record back to the state returned as `previous` by Mark.
// @Summary      Undo a mark
// @Tags         admin
// @Accept       json
// @Param        id  path  string  true  "Record ID"
// @Success      200  {object}  map[string]string
// @Failure      400  {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/attendance/{id}/restore [post]
func (h *AdminMarkHandler) Restore(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid record ID")
	}
	var p PreviousRecord
	if err := c.BodyParser(&p); err != nil || !validStatus[p.Status] || (p.MarkedBy != domain.MarkedBySelf && p.MarkedBy != domain.MarkedByAdmin) {
		return utils.SendError(c, fiber.StatusBadRequest, "A valid previous status is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := h.records.UpdateRecord(ctx, id, bson.M{"status": p.Status, "marked_by": p.MarkedBy, "marked_by_user": p.MarkedByUser, "submitted_at": p.SubmittedAt, "checkin_style": p.CheckinStyle}); err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error restoring attendance")
	}
	return utils.SendResponse(c, fiber.StatusOK, "Attendance restored", nil)
}
