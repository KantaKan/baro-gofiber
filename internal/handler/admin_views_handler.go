package handler

import (
	"context"
	"time"

	"gofiber-baro/internal/domain"
	"gofiber-baro/internal/service/attendance"
	"gofiber-baro/pkg/utils"

	"github.com/gofiber/fiber/v2"
)

type adminViewUsers interface {
	GetAllUsers(cohort int, role, email, search, sort string, sortDir, page, limit int, excludeAttendanceStatus ...string) ([]domain.User, int, error)
}

type adminViewRecords interface {
	FindRecords(ctx interface{}, filter domain.AttendanceRecordFilter, opts interface{}) ([]domain.AttendanceRecord, error)
}

// AdminViewsHandler serves the assembled admin read views (Today, Register).
type AdminViewsHandler struct {
	users    adminViewUsers
	records  adminViewRecords
	settings attendance.CohortSettingsGetter
	holidays attendance.HolidayChecker
	now      func() time.Time
}

func NewAdminViewsHandler(users adminViewUsers, records adminViewRecords, settings attendance.CohortSettingsGetter, holidays attendance.HolidayChecker) *AdminViewsHandler {
	return &AdminViewsHandler{users: users, records: records, settings: settings, holidays: holidays, now: utils.GetThailandTime}
}

func (h *AdminViewsHandler) cohortData(cohort int) ([]domain.User, []domain.AttendanceRecord, domain.CohortSettings, error) {
	users, _, err := h.users.GetAllUsers(cohort, "learner", "", "", "jsd_number", 1, 1, 1000)
	if err != nil {
		return nil, nil, domain.CohortSettings{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	recs, err := h.records.FindRecords(ctx, domain.AttendanceRecordFilter{Cohort: cohort, NotDeleted: true}, nil)
	if err != nil {
		return nil, nil, domain.CohortSettings{}, err
	}
	st, err := h.settings.Get(cohort)
	return users, recs, st, err
}

// Today returns the live check-in board for a cohort.
// @Summary      Admin Today board
// @Tags         admin
// @Produce      json
// @Param        cohort  query  int  true  "Cohort number"
// @Success      200  {object}  attendance.TodayView
// @Failure      400  {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/today [get]
func (h *AdminViewsHandler) Today(c *fiber.Ctx) error {
	cohort := c.QueryInt("cohort")
	if cohort <= 0 {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid cohort")
	}
	users, recs, st, err := h.cohortData(cohort)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error loading today")
	}
	now := h.now()
	classDay, err := attendance.IsClassDay(now, h.holidays)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error loading today")
	}
	return utils.SendResponse(c, fiber.StatusOK, "Today retrieved", attendance.BuildTodayRoster(users, recs, now, st.Rules, classDay))
}
