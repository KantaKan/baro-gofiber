package handler

import (
	"context"
	"log"
	"time"

	"gofiber-baro/internal/domain"
	"gofiber-baro/internal/service/attendance"
	"gofiber-baro/pkg/utils"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type adminViewUsers interface {
	ListCohortLearners(cohort int, projection bson.M) ([]domain.User, error)
}

// learnerFields is what the cohort list views read from a user, plus any extra paths
// (e.g. reflection fields). The learner overview loads the full user on its own.
func learnerFields(extra ...string) bson.M {
	p := bson.M{}
	for _, f := range []string{"jsd_number", "first_name", "last_name", "email", "cohort_number", "role", "genmate_group", "zoom_name", "attendance_status", "deleted", "avatar"} {
		p[f] = 1
	}
	for _, f := range extra {
		p[f] = 1
	}
	return p
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

// viewRecordFields drops the record fields the admin views never read.
func viewRecordFields() *options.FindOptions {
	return options.Find().SetProjection(bson.M{"first_name": 0, "last_name": 0, "jsd_number": 0, "cohort_number": 0, "locked": 0, "deleted": 0, "deleted_at": 0, "deleted_by": 0})
}

// historyFields is all that streaks, leave and attendance rate read.
func historyFields() *options.FindOptions {
	return options.Find().SetProjection(bson.M{"_id": 0, "user_id": 1, "date": 1, "session": 1, "status": 1})
}

// loadCohortRecords returns a cohort's slim attendance history, with full records for the
// dates from..to (the cells a view actually shows). Empty from means history only.
func loadCohortRecords(r adminViewRecords, cohort int, from, to string) ([]domain.AttendanceRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	hist, err := r.FindRecords(ctx, domain.AttendanceRecordFilter{Cohort: cohort, NotDeleted: true}, historyFields())
	if err != nil || from == "" {
		return hist, err
	}
	out, err := r.FindRecords(ctx, domain.AttendanceRecordFilter{Cohort: cohort, NotDeleted: true, DateFrom: from, DateTo: to}, viewRecordFields())
	if err != nil {
		return nil, err
	}
	for _, rec := range hist {
		if rec.Date < from || rec.Date > to {
			out = append(out, rec)
		}
	}
	return out, nil
}

func (h *AdminViewsHandler) cohortData(cohort int, from, to string, userFields bson.M) ([]domain.User, []domain.AttendanceRecord, domain.CohortSettings, error) {
	users, err := h.users.ListCohortLearners(cohort, userFields)
	if err != nil {
		log.Printf("admin views cohort %d: users: %v", cohort, err)
		return nil, nil, domain.CohortSettings{}, err
	}
	recs, err := loadCohortRecords(h.records, cohort, from, to)
	if err != nil {
		log.Printf("admin views cohort %d: attendance records: %v", cohort, err)
		return nil, nil, domain.CohortSettings{}, err
	}
	st, err := h.settings.Get(cohort)
	if err != nil {
		log.Printf("admin views cohort %d: settings: %v", cohort, err)
	}
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
	now := h.now()
	today := utils.FormatDate(now)
	users, recs, st, err := h.cohortData(cohort, today, today, learnerFields("reflections.day", "reflections.date", "reflections.reflection.barometer"))
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error loading today")
	}
	classDay, err := attendance.IsClassDay(now, h.holidays)
	if err != nil {
		log.Printf("admin today cohort %d: holidays: %v", cohort, err)
		return utils.SendError(c, fiber.StatusInternalServerError, "Error loading today")
	}
	classDay = classDay && st.InSession(utils.FormatDate(now))
	return utils.SendResponse(c, fiber.StatusOK, "Today retrieved", attendance.BuildTodayRoster(users, recs, now, st.Rules, classDay, st.FlagsOn()))
}

func mondayOf(t time.Time) string {
	back := (int(t.Weekday()) + 6) % 7
	return time.Date(t.Year(), t.Month(), t.Day()-back, 0, 0, 0, 0, t.Location()).Format("2006-01-02")
}

// Register returns one week of AM/PM attendance for a cohort.
// @Summary      Admin weekly register
// @Tags         admin
// @Produce      json
// @Param        cohort  query  int     true   "Cohort number"
// @Param        week    query  string  false  "Monday YYYY-MM-DD (default: this week)"
// @Success      200  {object}  attendance.RegisterView
// @Failure      400  {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/attendance/register [get]
func (h *AdminViewsHandler) Register(c *fiber.Ctx) error {
	cohort := c.QueryInt("cohort")
	if cohort <= 0 {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid cohort")
	}
	week := c.Query("week")
	if week == "" {
		week = mondayOf(h.now())
	} else if t, err := time.Parse("2006-01-02", week); err != nil || t.Weekday() != time.Monday {
		return utils.SendError(c, fiber.StatusBadRequest, "week must be a Monday (YYYY-MM-DD)")
	}
	t, _ := time.Parse("2006-01-02", week)
	users, recs, _, err := h.cohortData(cohort, week, t.AddDate(0, 0, 4).Format("2006-01-02"), learnerFields())
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error loading register")
	}
	hol := map[string]bool{}
	for i := 0; i < 5; i++ {
		d := t.AddDate(0, 0, i).Format("2006-01-02")
		if ok, _, err := h.holidays.IsHoliday(d); err == nil && ok {
			hol[d] = true
		}
	}
	return utils.SendResponse(c, fiber.StatusOK, "Register retrieved", attendance.BuildRegister(users, recs, week, hol))
}
