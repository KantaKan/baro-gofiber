package handler

import (
	"errors"
	"io"
	"strconv"
	"strings"

	"gofiber-baro/internal/service/calendar"
	"gofiber-baro/pkg/middleware"
	"gofiber-baro/pkg/utils"

	"github.com/gofiber/fiber/v2"
)

type calendarService interface {
	ImportAt(cohort int, startDate string, data []byte, sheet string, save bool, by string, expectVersion *int) (calendar.Preview, error)
	Get(cohort int) (*calendar.Calendar, error)
	Publish(cohort, n int, published bool) error
	SetZoom(cohort int, url string) error
	ForLearner(cohort int) (calendar.LearnerCalendar, error)
}

const maxCalendarUpload = 4 << 20 // matches Fiber's default body limit

// CalendarHandler: import the program calendar, publish weeks, and serve learners their published weeks.
// Admin routes take the cohort from ?cohort= so the staff guard's cohort scope applies.
type CalendarHandler struct {
	svc       calendarService
	startDate func(cohort int) (string, error)
}

func NewCalendarHandler(s calendarService, startDate func(cohort int) (string, error)) *CalendarHandler {
	return &CalendarHandler{svc: s, startDate: startDate}
}

func calendarErr(c *fiber.Ctx, err error) error {
	var ue *calendar.UserError
	switch {
	case errors.As(err, &ue):
		return utils.SendError(c, fiber.StatusBadRequest, ue.Msg)
	case errors.Is(err, calendar.ErrBadFile), errors.Is(err, calendar.ErrBadURL):
		return utils.SendError(c, fiber.StatusBadRequest, err.Error())
	case errors.Is(err, calendar.ErrConflict):
		return utils.SendError(c, fiber.StatusConflict, err.Error())
	case errors.Is(err, calendar.ErrNoWeek), errors.Is(err, calendar.ErrNoCalendar):
		return utils.SendError(c, fiber.StatusNotFound, err.Error())
	}
	return utils.SendError(c, fiber.StatusInternalServerError, "Something went wrong with the calendar")
}

// Import reads an uploaded .xlsx. Without save=1 it only previews (warnings and differences).
// @Summary  Import the program calendar  @Tags admin  @Security BearerAuth  @Router /admin/calendar/import [post]
func (h *CalendarHandler) Import(c *fiber.Ctx) error {
	cohort := c.QueryInt("cohort")
	if cohort <= 0 {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid cohort")
	}
	fh, err := c.FormFile("file")
	if err != nil || fh == nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Choose the calendar .xlsx file")
	}
	if !strings.HasSuffix(strings.ToLower(fh.Filename), ".xlsx") {
		return utils.SendError(c, fiber.StatusBadRequest, "The calendar must be an .xlsx file")
	}
	if fh.Size > maxCalendarUpload {
		return utils.SendError(c, fiber.StatusBadRequest, "That file is too large (4 MB at most)")
	}
	sheet := strings.TrimSpace(c.Query("sheet"))
	if len(sheet) > 100 {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid sheet name")
	}
	var version *int
	if v := c.Query("version"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return utils.SendError(c, fiber.StatusBadRequest, "Invalid version")
		}
		version = &n
	}
	f, err := fh.Open()
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Couldn't read that file")
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxCalendarUpload+1))
	if err != nil || len(data) > maxCalendarUpload {
		return utils.SendError(c, fiber.StatusBadRequest, "Couldn't read that file")
	}
	start, err := h.startDate(cohort)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Couldn't load the cohort")
	}
	by := ""
	if s, ok := StaffFrom(c); ok {
		by = s.Name
	}
	pv, err := h.svc.ImportAt(cohort, start, data, sheet, c.Query("save") == "1", by, version)
	if err != nil {
		return calendarErr(c, err)
	}
	return utils.SendResponse(c, fiber.StatusOK, "ok", pv)
}

// Get returns the saved calendar with each week's publish state.
// @Summary  Saved calendar  @Tags admin  @Security BearerAuth  @Router /admin/calendar [get]
func (h *CalendarHandler) Get(c *fiber.Ctx) error {
	cohort := c.QueryInt("cohort")
	if cohort <= 0 {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid cohort")
	}
	cal, err := h.svc.Get(cohort)
	if err != nil {
		return calendarErr(c, err)
	}
	if cal == nil {
		return utils.SendResponse(c, fiber.StatusOK, "ok", fiber.Map{"cohort": cohort, "weeks": []any{}, "zoom_url": "", "version": 0, "imported": false})
	}
	return utils.SendResponse(c, fiber.StatusOK, "ok", fiber.Map{
		"cohort": cal.Cohort, "sheet": cal.Sheet, "zoom_url": cal.ZoomURL, "weeks": cal.Weeks,
		"imported_at": cal.ImportedAt, "imported_by": cal.ImportedBy, "version": cal.Version, "imported": true,
	})
}

// Publish shows or hides one week to learners.
// @Summary  Publish a week  @Tags admin  @Security BearerAuth  @Router /admin/calendar/weeks/{n}/publish [put]
func (h *CalendarHandler) Publish(c *fiber.Ctx) error {
	cohort := c.QueryInt("cohort")
	n, err := c.ParamsInt("n")
	if cohort <= 0 || err != nil || n <= 0 {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid cohort or week")
	}
	var b struct {
		Published bool `json:"published"`
	}
	if err := c.BodyParser(&b); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}
	if err := h.svc.Publish(cohort, n, b.Published); err != nil {
		return calendarErr(c, err)
	}
	return utils.SendResponse(c, fiber.StatusOK, "ok", nil)
}

// Settings stores the Join Zoom link.
// @Summary  Calendar settings  @Tags admin  @Security BearerAuth  @Router /admin/calendar/settings [put]
func (h *CalendarHandler) Settings(c *fiber.Ctx) error {
	cohort := c.QueryInt("cohort")
	if cohort <= 0 {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid cohort")
	}
	var b struct {
		ZoomURL string `json:"zoom_url"`
	}
	if err := c.BodyParser(&b); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}
	if err := h.svc.SetZoom(cohort, b.ZoomURL); err != nil {
		return calendarErr(c, err)
	}
	return utils.SendResponse(c, fiber.StatusOK, "ok", nil)
}

// Mine is the learner's calendar: published weeks of their own cohort only.
// @Summary  My cohort's published calendar  @Tags learner  @Security BearerAuth  @Router /calendar/me [get]
func (h *CalendarHandler) Mine(c *fiber.Ctx) error {
	claims, ok := c.Locals("user").(*middleware.Claims)
	if !ok || claims.Cohort <= 0 {
		return utils.SendError(c, fiber.StatusForbidden, "No cohort")
	}
	lc, err := h.svc.ForLearner(claims.Cohort)
	if err != nil {
		return calendarErr(c, err)
	}
	return utils.SendResponse(c, fiber.StatusOK, "ok", lc)
}
