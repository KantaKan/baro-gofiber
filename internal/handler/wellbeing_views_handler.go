package handler

import (
	"context"
	"html"
	"log"
	"math"
	"sort"
	"strconv"
	"time"

	"gofiber-baro/internal/domain"
	"gofiber-baro/internal/service/attendance"
	"gofiber-baro/internal/service/wellbeing"
	"gofiber-baro/pkg/utils"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/sync/errgroup"
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
	stats    statsReader
	holidays attendance.HolidayChecker
	care     careReader
	now      func() time.Time
	settings func(cohort int) (domain.CohortSettings, error)
}

// SetCohortSettings lets the support list skip cohorts that have graduated.
func (h *WellbeingViewsHandler) SetCohortSettings(fn func(cohort int) (domain.CohortSettings, error)) {
	h.settings = fn
}

func NewWellbeingViewsHandler(users wbUsers, records adminViewRecords, stats statsReader, holidays attendance.HolidayChecker, care careReader) *WellbeingViewsHandler {
	return &WellbeingViewsHandler{users: users, records: records, stats: stats, holidays: holidays, care: care, now: utils.GetThailandTime}
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

func (h *WellbeingViewsHandler) cohortUsers(c *fiber.Ctx, fields bson.M) ([]domain.User, int, bool, error) {
	cohort := c.QueryInt("cohort")
	if cohort <= 0 {
		return nil, 0, false, utils.SendError(c, fiber.StatusBadRequest, "Invalid cohort")
	}
	users, err := h.users.ListCohortLearners(cohort, fields)
	if err != nil {
		log.Printf("wellbeing cohort %d: users: %v", cohort, err)
		return nil, 0, false, utils.SendError(c, fiber.StatusInternalServerError, "Error loading learners")
	}
	return users, cohort, true, nil
}

// textNight is the latest class night: the only night whose reflection text any view reads.
// classNights(2) looks back far enough to cross a long weekend, matching Support's window.
func (h *WellbeingViewsHandler) textNight() string {
	if n := h.classNights(2); len(n) > 0 {
		return n[len(n)-1]
	}
	return ""
}

// learnersAndStats loads a cohort's learners (last n reflections, text for the latest night) and stored stats in parallel.
func (h *WellbeingViewsHandler) learnersAndStats(c *fiber.Ctx, n int) ([]domain.User, map[primitive.ObjectID]domain.LearnerStats, bool, error) {
	cohort := c.QueryInt("cohort")
	if cohort <= 0 {
		return nil, nil, false, utils.SendError(c, fiber.StatusBadRequest, "Invalid cohort")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var (
		users []domain.User
		stats []domain.LearnerStats
		g     errgroup.Group
	)
	g.Go(func() (err error) {
		users, err = h.users.ListCohortLearnersWithReflections(cohort, learnerFields(), n, h.textNight())
		return err
	})
	g.Go(func() (err error) {
		stats, err = h.stats.FindByCohort(ctx, cohort, false)
		return err
	})
	if err := g.Wait(); err != nil {
		log.Printf("wellbeing cohort %d: %v", cohort, err)
		return nil, nil, false, utils.SendError(c, fiber.StatusInternalServerError, "Error loading learners")
	}
	return users, statsMap(stats), true, nil
}

func (h *WellbeingViewsHandler) cohortRecords(cohort int) ([]domain.AttendanceRecord, error) {
	recs, err := loadCohortRecords(h.records, cohort)
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
	cohort := c.QueryInt("cohort")
	if cohort <= 0 {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid cohort")
	}
	rng := c.Query("range", "night")
	if rng != "night" && rng != "week" && rng != "4w" {
		return utils.SendError(c, fiber.StatusBadRequest, "range must be night, week or 4w")
	}
	nights := h.classNights(20)
	textDay := ""
	if len(nights) > 0 {
		textDay = nights[len(nights)-1]
	}
	n := 20
	if rng == "4w" {
		n = 30
	}
	users, err := h.users.ListCohortLearnersWithReflections(cohort, learnerFields(), n, textDay)
	if err != nil {
		log.Printf("wellbeing cohort %d: users: %v", cohort, err)
		return utils.SendError(c, fiber.StatusInternalServerError, "Error loading learners")
	}
	return utils.SendResponse(c, fiber.StatusOK, "Reflections", wellbeing.BuildBoard(users, nights, rng, wellbeing.DefaultWords))
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
	users, stats, ok, err := h.learnersAndStats(c, 20)
	if !ok {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	items, _ := h.care.ItemsFor(ctx, ids(users))
	acks, _ := h.care.AcksFor(ctx, ids(users), time.Now())
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
		st := stats[u.ID]
		row := learnerRow{LearnerRef: wellbeing.Ref(u), Email: u.Email, AttendanceStatus: u.AttendanceStatus, AttendanceRate: st.AttendanceRate, LeaveUsed: st.LeaveUsed, LastSeen: st.LastSeen, Mood: make([]string, len(nights))}
		by := map[string]string{}
		for _, n := range wellbeing.Nights(u.Reflections) {
			by[n.Day] = n.Zone
		}
		for i, d := range nights {
			row.Mood[i] = by[d]
		}
		if u.AttendanceStatus != "dropout" && u.AttendanceStatus != "dismissed" {
			rs, _, _ := wellbeing.ReasonsFor(u, st, items, acks, recent, wellbeing.DefaultWords)
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
type sessionAt struct {
	Status   domain.AttendanceStatus `json:"status"`
	At       time.Time               `json:"at"`
	MarkedBy domain.MarkedBy         `json:"marked_by"`
}

type dayReflection struct {
	ID             string     `json:"id"`
	Zone           string     `json:"zone"`
	SentAt         time.Time  `json:"sent_at"`
	TechHappy      string     `json:"tech_happy"`
	TechImprove    string     `json:"tech_improve"`
	NontechHappy   string     `json:"nontech_happy"`
	NontechImprove string     `json:"nontech_improve"`
	Reply          string     `json:"reply"`
	ReplyName      string     `json:"reply_name"`
	ReplyAt        *time.Time `json:"reply_at"`
}

type overviewDay struct {
	Date       string         `json:"date"`
	Extra      bool           `json:"extra,omitempty"`
	Morning    *sessionAt     `json:"morning"`
	Afternoon  *sessionAt     `json:"afternoon"`
	Reflection *dayReflection `json:"reflection"`
}

type holidayRange interface {
	GetHolidayDatesInRange(startDate, endDate string) (map[string]bool, error)
}

// courseDays lists class days from the course start up to today. A missing, malformed or
// future start falls back to the learner's first record or reflection; the walk is capped
// at 14 months and holidays come from one range lookup when the checker supports it.
func (h *WellbeingViewsHandler) courseDays(start string, recs []domain.AttendanceRecord, refls []domain.Reflection) []string {
	earliest := ""
	for _, r := range recs {
		if earliest == "" || r.Date < earliest {
			earliest = r.Date
		}
	}
	for _, r := range refls {
		if d := wellbeing.DayOf(r); d != "" && (earliest == "" || d < earliest) {
			earliest = d
		}
	}
	from := start
	if _, err := utils.ParseDate(from); err != nil || (earliest != "" && from > earliest) {
		from = earliest
	}
	first, err := utils.ParseDate(from)
	if err != nil {
		return nil
	}
	now := h.now()
	if floor := now.AddDate(0, -14, 0); first.Before(floor) {
		first = floor
	}
	today := utils.FormatDate(now)
	isClass := func(d time.Time) bool {
		ok, err := attendance.IsClassDay(d, h.holidays)
		return err == nil && ok
	}
	if hr, ok := h.holidays.(holidayRange); ok {
		if set, err := hr.GetHolidayDatesInRange(utils.FormatDate(first), today); err == nil {
			isClass = func(d time.Time) bool {
				wd := d.Weekday()
				return wd != time.Saturday && wd != time.Sunday && !set[utils.FormatDate(d)]
			}
		}
	}
	var out []string
	for d := first; utils.FormatDate(d) <= today; d = d.AddDate(0, 0, 1) {
		if isClass(d) {
			out = append(out, utils.FormatDate(d))
		}
	}
	return out
}

func overviewDays(dates []string, recs []domain.AttendanceRecord, refls []domain.Reflection) []*overviewDay {
	out := make([]*overviewDay, len(dates))
	by := make(map[string]*overviewDay, len(dates))
	for i, d := range dates {
		out[i] = &overviewDay{Date: d}
		by[d] = out[i]
	}
	for _, r := range refls {
		if d := wellbeing.DayOf(r); d != "" && by[d] == nil {
			x := &overviewDay{Date: d, Extra: true}
			out = append(out, x)
			by[d] = x
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date < out[j].Date })
	for _, r := range recs {
		x := by[r.Date]
		if x == nil {
			continue
		}
		s := &sessionAt{Status: r.Status, At: r.SubmittedAt, MarkedBy: r.MarkedBy}
		if r.Session == domain.SessionAfternoon {
			x.Afternoon = s
		} else {
			x.Morning = s
		}
	}
	un := html.UnescapeString
	for _, r := range refls {
		x := by[wellbeing.DayOf(r)]
		if x == nil {
			continue
		}
		d := r.ReflectionData
		x.Reflection = &dayReflection{
			ID: r.ID.Hex(), Zone: attendance.ZoneOf(d.Barometer), SentAt: r.CreatedAt,
			TechHappy: un(d.TechSessions.Happy), TechImprove: un(d.TechSessions.Improve),
			NontechHappy: un(d.NonTechSessions.Happy), NontechImprove: un(d.NonTechSessions.Improve),
			Reply: r.AdminFeedback, ReplyName: r.AdminFeedbackName, ReplyAt: r.AdminFeedbackAt,
		}
	}
	return out
}

// neighbors is the previous and next learner of the cohort in JSD-number order.
func neighbors(peers []domain.User, id primitive.ObjectID) fiber.Map {
	num := func(s string) (int, bool) { n, err := strconv.Atoi(s); return n, err == nil }
	sort.SliceStable(peers, func(i, j int) bool {
		a, aok := num(peers[i].JSDNumber)
		b, bok := num(peers[j].JSDNumber)
		if aok && bok {
			return a < b
		}
		return peers[i].JSDNumber < peers[j].JSDNumber
	})
	out := fiber.Map{"prev": nil, "next": nil, "index": 0, "total": len(peers)}
	for i, p := range peers {
		if p.ID != id {
			continue
		}
		out["index"] = i
		if i > 0 {
			out["prev"] = peers[i-1].ID.Hex()
		}
		if i+1 < len(peers) {
			out["next"] = peers[i+1].ID.Hex()
		}
	}
	return out
}

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
	var st domain.CohortSettings
	if h.settings != nil {
		st, _ = h.settings(u.CohortNumber)
	}
	dayN, week, total := attendance.CourseProgress(st.StartDate, st.CourseDays, utils.FormatDate(h.now()))
	cohortRate := 0.0
	if list, err := h.stats.FindByCohort(ctx, u.CohortNumber, false); err == nil && len(list) > 0 {
		for _, s := range list {
			cohortRate += s.AttendanceRate
		}
		cohortRate = math.Round(cohortRate/float64(len(list))*1000) / 1000
	}
	badges := make([]fiber.Map, 0, len(u.Badges))
	for _, b := range u.Badges {
		badges = append(badges, fiber.Map{"id": b.ID.Hex(), "name": b.Name, "emoji": b.Emoji, "image_url": b.ImageUrl, "color": b.Color, "style": b.Style, "awarded_at": b.AwardedAt})
	}
	peers, _ := h.users.ListCohortLearners(u.CohortNumber, bson.M{"_id": 1, "jsd_number": 1})
	reasons, _, _ := wellbeing.ReasonsFor(*u, attendance.ComputeStats(recs), items, acks, recent, wellbeing.DefaultWords)
	return utils.SendResponse(c, fiber.StatusOK, "Overview", fiber.Map{
		"learner":           fiber.Map{"id": u.ID.Hex(), "first_name": u.FirstName, "last_name": u.LastName, "zoom_name": u.ZoomName, "jsd_number": u.JSDNumber, "genmate_group": u.GenmateGroup, "avatar": u.Avatar, "email": u.Email, "created_at": u.ID.Timestamp(), "cohort_number": u.CohortNumber},
		"attendance_rate":   attendance.AttendanceRate(days),
		"leave_used":        attendance.LeaveDaysUsed(days),
		"streak":            attendance.OnTimeStreak(days),
		"last20":            last20,
		"nights":            nightList,
		"latest_quote":      quote,
		"patterns":          wellbeing.WorthALook(ns, recent, wellbeing.DefaultWords),
		"reasons":           reasons,
		"notes":             notes,
		"support":           items,
		"days":              overviewDays(h.courseDays(st.StartDate, recs, u.Reflections), recs, u.Reflections),
		"course":            fiber.Map{"week_number": week, "total_weeks": total, "day_number": dayN, "course_days": st.CourseDays},
		"best_streak":       attendance.LongestOnTimeStreak(days),
		"cohort_attendance": cohortRate,
		"badges":            badges,
		"neighbors":         neighbors(peers, u.ID),
		"plant":             fiber.Map{"palette": u.SelectedPalette, "species": u.SelectedSpecies, "pot": u.SelectedPot, "leaf": u.SelectedLeaf, "flower": u.SelectedFlower, "stem": u.SelectedStem},
	})
}

// Support is the Needs support list.
// @Summary  Needs support  @Tags admin  @Security BearerAuth  @Router /admin/support [get]
func (h *WellbeingViewsHandler) Support(c *fiber.Ctx) error {
	// A graduated cohort has no class nights left, so "hasn't written" nudges would never end.
	if cohort := c.QueryInt("cohort"); h.settings != nil && cohort > 0 {
		if st, err := h.settings(cohort); err == nil && !st.InSession(utils.FormatDate(h.now())) {
			return utils.SendResponse(c, fiber.StatusOK, "Needs support", []wellbeing.SupportRow{})
		}
	}
	users, stats, ok, err := h.learnersAndStats(c, 20)
	if !ok {
		return err
	}
	return utils.SendResponse(c, fiber.StatusOK, "Needs support", h.SupportRows(users, stats))
}

// SupportRows is the Needs support list for a cohort's learners (shared with the weekly report).
func (h *WellbeingViewsHandler) SupportRows(users []domain.User, stats map[primitive.ObjectID]domain.LearnerStats) []wellbeing.SupportRow {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	items, _ := h.care.ItemsFor(ctx, ids(users))
	acks, _ := h.care.AcksFor(ctx, ids(users), time.Now())
	return wellbeing.BuildSupportList(users, stats, items, acks, h.classNights(2), wellbeing.DefaultWords)
}

// Weeks: rough nights (overwhelmed or panic) per week for the Weekly summary, newest first.
// @Summary  Reflections by week  @Tags admin  @Security BearerAuth  @Router /admin/reflections/weeks [get]
func (h *WellbeingViewsHandler) Weeks(c *fiber.Ctx) error {
	n := c.QueryInt("weeks", 6)
	if n < 1 || n > 20 {
		return utils.SendError(c, fiber.StatusBadRequest, "weeks must be between 1 and 20")
	}
	users, _, ok, err := h.cohortUsers(c, learnerFields("reflections"))
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
