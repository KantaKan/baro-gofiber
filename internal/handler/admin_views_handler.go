package handler

import (
	"context"
	"log"
	"math"
	"slices"
	"strings"
	"time"

	"gofiber-baro/internal/domain"
	"gofiber-baro/internal/service/attendance"
	"gofiber-baro/pkg/utils"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/sync/errgroup"
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

// recentReflections is learnerFields plus only each learner's last n reflections
// (reflections are appended, so the tail is the newest).
func recentReflections(n int) bson.M {
	p := learnerFields()
	p["reflections"] = bson.M{"$slice": -n}
	return p
}

type adminViewRecords interface {
	FindRecords(ctx interface{}, filter domain.AttendanceRecordFilter, opts interface{}) ([]domain.AttendanceRecord, error)
}

// AdminViewsHandler serves the assembled admin read views (Today, Register).
type AdminViewsHandler struct {
	users     adminViewUsers
	records   adminViewRecords
	settings  attendance.CohortSettingsGetter
	holidays  attendance.HolidayChecker
	now       func() time.Time
	drift     statsReader
	driftSync bool
}

func NewAdminViewsHandler(users adminViewUsers, records adminViewRecords, settings attendance.CohortSettingsGetter, holidays attendance.HolidayChecker) *AdminViewsHandler {
	return &AdminViewsHandler{users: users, records: records, settings: settings, holidays: holidays, now: utils.GetThailandTime}
}

// statsReader is the subset of the learner_stats store Today needs to check for drift.
type statsReader interface {
	FindByCohort(ctx context.Context, cohort int) ([]domain.LearnerStats, error)
}

// SetStatsDriftCheck compares Today's computed numbers with learner_stats and logs mismatches.
// It proves learner_stats on real data before the views read it.
func (h *AdminViewsHandler) SetStatsDriftCheck(store statsReader) { h.drift = store }

func (h *AdminViewsHandler) logStatsDrift(cohort int, v attendance.TodayView) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	stored, err := h.drift.FindByCohort(ctx, cohort)
	if err != nil {
		log.Printf("stats drift check: cohort %d: %v", cohort, err)
		return
	}
	by := map[string]domain.LearnerStats{}
	for _, s := range stored {
		by[s.UserID.Hex()] = s
	}
	var examples []string
	n := 0
	for _, l := range v.Learners {
		if f := statsDiff(by[l.ID], l); len(f) > 0 {
			n++
			if len(examples) < 3 {
				examples = append(examples, l.ID+":"+strings.Join(f, ","))
			}
		}
	}
	if n > 0 {
		log.Printf("stats drift: cohort %d, %d learners differ (e.g. %s)", cohort, n, strings.Join(examples, " "))
	}
}

// statsDiff names the fields where stored stats disagree with the live Today row.
func statsDiff(s domain.LearnerStats, l attendance.TodayLearner) []string {
	var f []string
	if s.Streak != l.Streak {
		f = append(f, "streak")
	}
	if s.LeaveUsed != l.LeaveUsed {
		f = append(f, "leave")
	}
	if math.Abs(s.AttendanceRate-l.AttendanceRate) > 1e-9 {
		f = append(f, "rate")
	}
	if !slices.Equal(s.Last10, l.Last10) {
		f = append(f, "last10")
	}
	return f
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
	var recs []domain.AttendanceRecord
	var g errgroup.Group
	g.Go(func() (err error) {
		if recs, err = loadCohortRecords(h.records, cohort, from, to); err != nil {
			log.Printf("admin views cohort %d: attendance records: %v", cohort, err)
		}
		return err
	})
	users, err := h.users.ListCohortLearners(cohort, userFields)
	if err != nil {
		log.Printf("admin views cohort %d: users: %v", cohort, err)
	}
	if e := g.Wait(); err == nil {
		err = e
	}
	if err != nil {
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
	view := attendance.BuildTodayRoster(users, recs, now, st.Rules, classDay, st.FlagsOn())
	if h.drift != nil {
		if h.driftSync {
			h.logStatsDrift(cohort, view)
		} else {
			go h.logStatsDrift(cohort, view)
		}
	}
	return utils.SendResponse(c, fiber.StatusOK, "Today retrieved", view)
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
