package handler

import (
	"context"
	"log"
	"sort"
	"time"

	"gofiber-baro/internal/domain"
	"gofiber-baro/internal/service/attendance"
	"gofiber-baro/internal/service/wellbeing"
	"gofiber-baro/pkg/utils"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type careReader interface {
	Notes(ctx context.Context, userID primitive.ObjectID) ([]domain.AdminNote, error)
	Items(ctx context.Context, userID primitive.ObjectID) ([]domain.SupportItem, error)
	ItemsFor(ctx context.Context, ids []primitive.ObjectID) ([]domain.SupportItem, error)
	AcksFor(ctx context.Context, ids []primitive.ObjectID, now time.Time) ([]domain.SupportAck, error)
}

type wbUsers interface {
	adminViewUsers
	GetUserByID(id string) (*domain.User, error)
}

// WellbeingViewsHandler serves Reflections, All learners, Learner overview and Needs support.
type WellbeingViewsHandler struct {
	users    wbUsers
	records  adminViewRecords
	holidays attendance.HolidayChecker
	care     careReader
	now      func() time.Time
	settings func(cohort int) (domain.CohortSettings, error)
}

// SetCohortSettings lets the support list skip cohorts that have graduated.
func (h *WellbeingViewsHandler) SetCohortSettings(fn func(cohort int) (domain.CohortSettings, error)) {
	h.settings = fn
}

func NewWellbeingViewsHandler(users wbUsers, records adminViewRecords, holidays attendance.HolidayChecker, care careReader) *WellbeingViewsHandler {
	return &WellbeingViewsHandler{users: users, records: records, holidays: holidays, care: care, now: utils.GetThailandTime}
}

// classNights returns the last n class days (oldest first) up to "last night":
// today once reflections open (16:45), otherwise the previous class day.
func (h *WellbeingViewsHandler) classNights(n int) []string {
	now := h.now()
	d := now
	if now.Hour()*60+now.Minute() < 16*60+45 {
		d = d.AddDate(0, 0, -1)
	}
	var out []string
	for i := 0; len(out) < n && i < n*3; i++ {
		day := d.AddDate(0, 0, -i)
		if ok, err := attendance.IsClassDay(day, h.holidays); err == nil && ok {
			out = append(out, utils.FormatDate(day))
		}
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

func (h *WellbeingViewsHandler) cohortUsers(c *fiber.Ctx) ([]domain.User, int, bool, error) {
	cohort := c.QueryInt("cohort")
	if cohort <= 0 {
		return nil, 0, false, utils.SendError(c, fiber.StatusBadRequest, "Invalid cohort")
	}
	users, _, err := h.users.GetAllUsers(cohort, "learner", "", "", "jsd_number", 1, 1, 1000)
	if err != nil {
		return nil, 0, false, utils.SendError(c, fiber.StatusInternalServerError, "Error loading learners")
	}
	return users, cohort, true, nil
}

func (h *WellbeingViewsHandler) cohortRecords(cohort int) ([]domain.AttendanceRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	recs, err := h.records.FindRecords(ctx, domain.AttendanceRecordFilter{Cohort: cohort, NotDeleted: true}, viewRecordFields())
	if err != nil {
		log.Printf("wellbeing cohort %d: attendance records: %v", cohort, err)
	}
	return recs, err
}

func ids(users []domain.User) []primitive.ObjectID {
	out := make([]primitive.ObjectID, len(users))
	for i, u := range users {
		out[i] = u.ID
	}
	return out
}

// Board is the Reflections page data.
// @Summary  Reflections board  @Tags admin  @Security BearerAuth  @Router /admin/reflections/board [get]
func (h *WellbeingViewsHandler) Board(c *fiber.Ctx) error {
	users, _, ok, err := h.cohortUsers(c)
	if !ok {
		return err
	}
	rng := c.Query("range", "night")
	if rng != "night" && rng != "week" && rng != "4w" {
		return utils.SendError(c, fiber.StatusBadRequest, "range must be night, week or 4w")
	}
	return utils.SendResponse(c, fiber.StatusOK, "Reflections", wellbeing.BuildBoard(users, h.classNights(20), rng, wellbeing.DefaultWords))
}

type learnerRow struct {
	wellbeing.LearnerRef
	Email            string     `json:"email"`
	AttendanceStatus string     `json:"attendance_status"`
	AttendanceRate   float64    `json:"attendance_rate"`
	LeaveUsed        int        `json:"leave_used"`
	Mood             []string   `json:"mood"`
	LastSeen         *time.Time `json:"last_seen"`
	Support          bool       `json:"support"`
}

// Learners is the All learners table (including dropped learners).
// @Summary  All learners  @Tags admin  @Security BearerAuth  @Router /admin/learners [get]
func (h *WellbeingViewsHandler) Learners(c *fiber.Ctx) error {
	users, cohort, ok, err := h.cohortUsers(c)
	if !ok {
		return err
	}
	recs, e := h.cohortRecords(cohort)
	if e != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error loading attendance")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	items, _ := h.care.ItemsFor(ctx, ids(users))
	acks, _ := h.care.AcksFor(ctx, ids(users), time.Now())
	byUser := map[primitive.ObjectID][]domain.AttendanceRecord{}
	for _, r := range recs {
		byUser[r.UserID] = append(byUser[r.UserID], r)
	}
	nights := h.classNights(10)
	recent := nights
	if len(recent) > 2 {
		recent = recent[len(recent)-2:]
	}
	out := []learnerRow{}
	for _, u := range users {
		if u.Role == "admin" || u.Deleted {
			continue
		}
		days := attendance.BuildDays(byUser[u.ID])
		row := learnerRow{LearnerRef: wellbeing.Ref(u), Email: u.Email, AttendanceStatus: u.AttendanceStatus, AttendanceRate: attendance.AttendanceRate(days), LeaveUsed: attendance.LeaveDaysUsed(days), Mood: make([]string, len(nights))}
		by := map[string]string{}
		for _, n := range wellbeing.Nights(u.Reflections) {
			by[n.Day] = n.Zone
		}
		for i, d := range nights {
			row.Mood[i] = by[d]
		}
		for _, r := range byUser[u.ID] {
			if r.MarkedBy == domain.MarkedBySelf && !r.SubmittedAt.IsZero() && (row.LastSeen == nil || r.SubmittedAt.After(*row.LastSeen)) {
				t := r.SubmittedAt
				row.LastSeen = &t
			}
		}
		if u.AttendanceStatus != "dropout" && u.AttendanceStatus != "dismissed" {
			rs, _, _ := wellbeing.ReasonsFor(u, byUser[u.ID], items, acks, recent, wellbeing.DefaultWords)
			row.Support = len(rs) > 0
		}
		out = append(out, row)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].JSDNumber < out[j].JSDNumber })
	return utils.SendResponse(c, fiber.StatusOK, "Learners", out)
}

type dayAMPM struct {
	Date      string                  `json:"date"`
	Morning   domain.AttendanceStatus `json:"morning"`
	Afternoon domain.AttendanceStatus `json:"afternoon"`
}

// Overview is one learner's full profile for admins.
// @Summary  Learner overview  @Tags admin  @Security BearerAuth  @Router /admin/learners/{id}/overview [get]
func (h *WellbeingViewsHandler) Overview(c *fiber.Ctx) error {
	id, e := primitive.ObjectIDFromHex(c.Params("id"))
	if e != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid learner ID")
	}
	u, e := h.users.GetUserByID(id.Hex())
	if e != nil || u == nil {
		return utils.SendError(c, fiber.StatusNotFound, "Learner not found")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	recs, e := h.records.FindRecords(ctx, domain.AttendanceRecordFilter{UserID: id, NotDeleted: true}, nil)
	if e != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error loading attendance")
	}
	notes, _ := h.care.Notes(ctx, id)
	items, _ := h.care.Items(ctx, id)
	acks, _ := h.care.AcksFor(ctx, []primitive.ObjectID{id}, time.Now())
	days := attendance.BuildDays(recs)
	nights := h.classNights(20)
	ampm := map[string]*dayAMPM{}
	for _, d := range nights {
		ampm[d] = &dayAMPM{Date: d}
	}
	for _, r := range recs {
		if x := ampm[r.Date]; x != nil {
			if r.Session == domain.SessionAfternoon {
				x.Afternoon = r.Status
			} else {
				x.Morning = r.Status
			}
		}
	}
	last20 := make([]dayAMPM, 0, len(nights))
	for _, d := range nights {
		last20 = append(last20, *ampm[d])
	}
	ns := wellbeing.Nights(u.Reflections)
	by := map[string]wellbeing.Night{}
	for _, n := range ns {
		by[n.Day] = n
	}
	n10 := nights
	if len(n10) > 10 {
		n10 = n10[len(n10)-10:]
	}
	type night struct {
		Day  string `json:"day"`
		Zone string `json:"zone"`
	}
	nightList := make([]night, len(n10))
	for i, d := range n10 {
		nightList[i] = night{d, by[d].Zone}
	}
	var quote any
	for i := len(ns) - 1; i >= 0; i-- {
		if ns[i].Zone == "panic" || ns[i].Zone == "stretch-overwhelmed" || len(wellbeing.Highlight(ns[i].Text, wellbeing.DefaultWords)) > 0 {
			quote = fiber.Map{"day": ns[i].Day, "zone": ns[i].Zone, "text": ns[i].Text}
			break
		}
	}
	recent := n10
	if len(recent) > 2 {
		recent = recent[len(recent)-2:]
	}
	reasons, _, _ := wellbeing.ReasonsFor(*u, recs, items, acks, recent, wellbeing.DefaultWords)
	return utils.SendResponse(c, fiber.StatusOK, "Overview", fiber.Map{
		"learner":         fiber.Map{"id": u.ID.Hex(), "first_name": u.FirstName, "last_name": u.LastName, "zoom_name": u.ZoomName, "jsd_number": u.JSDNumber, "genmate_group": u.GenmateGroup, "avatar": u.Avatar, "email": u.Email, "created_at": u.ID.Timestamp(), "cohort_number": u.CohortNumber},
		"attendance_rate": attendance.AttendanceRate(days),
		"leave_used":      attendance.LeaveDaysUsed(days),
		"streak":          attendance.OnTimeStreak(days),
		"last20":          last20,
		"nights":          nightList,
		"latest_quote":    quote,
		"patterns":        wellbeing.WorthALook(ns, recent, wellbeing.DefaultWords),
		"reasons":         reasons,
		"notes":           notes,
		"support":         items,
	})
}

// Support is the Needs support list.
// @Summary  Needs support  @Tags admin  @Security BearerAuth  @Router /admin/support [get]
func (h *WellbeingViewsHandler) Support(c *fiber.Ctx) error {
	users, cohort, ok, err := h.cohortUsers(c)
	if !ok {
		return err
	}
	// A graduated cohort has no class nights left, so "hasn't written" nudges would never end.
	if h.settings != nil {
		if st, err := h.settings(cohort); err == nil && !st.InSession(utils.FormatDate(h.now())) {
			return utils.SendResponse(c, fiber.StatusOK, "Needs support", []wellbeing.SupportRow{})
		}
	}
	recs, e := h.cohortRecords(cohort)
	if e != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error loading attendance")
	}
	return utils.SendResponse(c, fiber.StatusOK, "Needs support", h.SupportRows(users, recs))
}

// SupportRows is the Needs support list for a cohort's learners (shared with the weekly report).
func (h *WellbeingViewsHandler) SupportRows(users []domain.User, recs []domain.AttendanceRecord) []wellbeing.SupportRow {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	items, _ := h.care.ItemsFor(ctx, ids(users))
	acks, _ := h.care.AcksFor(ctx, ids(users), time.Now())
	return wellbeing.BuildSupportList(users, recs, items, acks, h.classNights(2), wellbeing.DefaultWords)
}

// Weeks: rough nights (overwhelmed or panic) per week for the Weekly summary, newest first.
// @Summary  Reflections by week  @Tags admin  @Security BearerAuth  @Router /admin/reflections/weeks [get]
func (h *WellbeingViewsHandler) Weeks(c *fiber.Ctx) error {
	n := c.QueryInt("weeks", 6)
	if n < 1 || n > 20 {
		return utils.SendError(c, fiber.StatusBadRequest, "weeks must be between 1 and 20")
	}
	users, _, ok, err := h.cohortUsers(c)
	if !ok {
		return err
	}
	now := h.now()
	today := utils.FormatDate(now)
	d := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	monday := d.AddDate(0, 0, -((int(d.Weekday()) + 6) % 7))
	mondays := make([]string, n)
	holidays := map[string]bool{}
	for i := 0; i < n; i++ {
		m := monday.AddDate(0, 0, -7*i)
		mondays[i] = m.Format("2006-01-02")
		for j := 0; j < 5; j++ {
			day := m.AddDate(0, 0, j).Format("2006-01-02")
			if hol, _, err := h.holidays.IsHoliday(day); err == nil && hol {
				holidays[day] = true
			}
		}
	}
	return utils.SendResponse(c, fiber.StatusOK, "ok", wellbeing.WeeklyRough(users, mondays, today, holidays))
}
