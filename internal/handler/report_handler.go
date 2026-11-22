package handler

import (
	"errors"

	"gofiber-baro/internal/service/report"
	"gofiber-baro/pkg/utils"

	"github.com/gofiber/fiber/v2"
)

type reportService interface {
	Get(cohort int, week string) (report.Report, error)
	SaveNotes(cohort int, week, notes string) error
	Send(cohort int, week string) (report.SendResult, error)
}

// ReportHandler serves the weekly report. The cohort is always the ?cohort= query so the
// staff guard's cohort scope applies to every route.
type ReportHandler struct{ svc reportService }

func NewReportHandler(s reportService) *ReportHandler { return &ReportHandler{svc: s} }

func reportErr(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, report.ErrSent):
		return utils.SendError(c, fiber.StatusConflict, err.Error())
	case errors.Is(err, report.ErrBadWeek), errors.Is(err, report.ErrFutureWeek), errors.Is(err, report.ErrNotesTooLong), errors.Is(err, report.ErrBadCohort), errors.Is(err, report.ErrTooOld):
		return utils.SendError(c, fiber.StatusBadRequest, err.Error())
	}
	return utils.SendError(c, fiber.StatusInternalServerError, "Couldn't load the report")
}

func reportCohort(c *fiber.Ctx) (int, bool) {
	n := c.QueryInt("cohort")
	return n, n > 0
}

// Get: the frozen report for a sent week, else a live draft.
// @Summary  Weekly report  @Tags admin  @Security BearerAuth  @Router /admin/report [get]
func (h *ReportHandler) Get(c *fiber.Ctx) error {
	n, ok := reportCohort(c)
	if !ok {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid cohort")
	}
	r, err := h.svc.Get(n, c.Query("week"))
	if err != nil {
		return reportErr(c, err)
	}
	return utils.SendResponse(c, fiber.StatusOK, "ok", r)
}

// SaveNotes stores the team's notes (locked once sent).
// @Summary  Save report notes  @Tags admin  @Security BearerAuth  @Router /admin/report/notes [put]
func (h *ReportHandler) SaveNotes(c *fiber.Ctx) error {
	n, ok := reportCohort(c)
	if !ok {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid cohort")
	}
	var b struct {
		Week  string `json:"week"`
		Notes string `json:"notes"`
	}
	if err := c.BodyParser(&b); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}
	if err := h.svc.SaveNotes(n, b.Week, b.Notes); err != nil {
		return reportErr(c, err)
	}
	return utils.SendResponse(c, fiber.StatusOK, "Saved", nil)
}

// Send freezes the report and posts it to Slack.
// @Summary  Send weekly report  @Tags admin  @Security BearerAuth  @Router /admin/report/send [post]
func (h *ReportHandler) Send(c *fiber.Ctx) error {
	n, ok := reportCohort(c)
	if !ok {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid cohort")
	}
	var b struct {
		Week string `json:"week"`
	}
	if err := c.BodyParser(&b); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}
	res, err := h.svc.Send(n, b.Week)
	if err != nil {
		return reportErr(c, err)
	}
	return utils.SendResponse(c, fiber.StatusOK, "ok", res)
}
