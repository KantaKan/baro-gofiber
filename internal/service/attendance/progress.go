package attendance

import (
	"context"
	"fmt"
	"sort"
	"time"

	"gofiber-baro/internal/domain"
	"gofiber-baro/pkg/utils"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	LeaveAllowance    = 8
	DefaultCourseDays = 75
)

// CourseProgress counts class weekdays from start (day 1) to today, capped at days.
// Returns zeros when start is empty/invalid; day/week are 0 before the course starts.
func CourseProgress(start string, days int, today string) (dayNumber, week, totalWeeks int) {
	s, err := utils.ParseDate(start)
	if err != nil || days <= 0 {
		return 0, 0, 0
	}
	totalWeeks = (days + 4) / 5
	t, err := utils.ParseDate(today)
	if err != nil || t.Before(s) {
		return 0, 0, totalWeeks
	}
	for d := s; !d.After(t) && dayNumber < days; d = d.AddDate(0, 0, 1) {
		if wd := d.Weekday(); wd != time.Saturday && wd != time.Sunday {
			dayNumber++
		}
	}
	if dayNumber == 0 {
		return 0, 0, totalWeeks
	}
	return dayNumber, (dayNumber-1)/5 + 1, totalWeeks
}

type DayResult struct {
	Date   string                  `json:"date"`
	Status domain.AttendanceStatus `json:"status"`
}

func skipsStreak(s domain.AttendanceStatus) bool {
	return s == domain.StatusHoliday || s == domain.StatusNoClass || s == domain.StatusAbsentExcused || s == domain.StatusLateExcused
}

// OnTimeStreak counts consecutive present days from the newest; holidays, no-class
// and excused days are skipped; late or absent ends the streak.
func OnTimeStreak(days []DayResult) int {
	n := 0
	for i := len(days) - 1; i >= 0; i-- {
		switch s := days[i].Status; {
		case s == domain.StatusPresent:
			n++
		case skipsStreak(s):
			continue
		default:
			return n
		}
	}
	return n
}

func LeaveDaysUsed(days []DayResult) int {
	n := 0
	for _, d := range days {
		switch d.Status {
		case domain.StatusAbsent, domain.StatusAbsentExcused, domain.StatusLate, domain.StatusLateExcused:
			n++
		}
	}
	return n
}

func WarningLevel(used int) string {
	switch {
	case used >= 7:
		return "third"
	case used >= 6:
		return "second"
	case used >= 4:
		return "first"
	default:
		return "none"
	}
}

func AttendanceRate(days []DayResult) float64 {
	attended, total := 0, 0
	for _, d := range days {
		switch d.Status {
		case domain.StatusHoliday, domain.StatusNoClass, domain.StatusDropout, domain.StatusDismissed, "":
			continue
		case domain.StatusPresent, domain.StatusLate, domain.StatusLateExcused:
			attended++
		}
		total++
	}
	if total == 0 {
		return 0
	}
	return float64(attended) / float64(total)
}

// HasPerfectWeek: some Mon–Fri week has records for all five weekdays, at least one
// class day, and every class day present.
func HasPerfectWeek(days []DayResult) bool {
	type wk struct {
		seen    map[time.Weekday]bool
		classes int
		broken  bool
	}
	weeks := map[string]*wk{}
	for _, d := range days {
		t, err := utils.ParseDate(d.Date)
		if err != nil || t.Weekday() == time.Saturday || t.Weekday() == time.Sunday {
			continue
		}
		y, w := t.ISOWeek()
		key := fmt.Sprintf("%d-%02d", y, w)
		k := weeks[key]
		if k == nil {
			k = &wk{seen: map[time.Weekday]bool{}}
			weeks[key] = k
		}
		k.seen[t.Weekday()] = true
		if d.Status == domain.StatusHoliday || d.Status == domain.StatusNoClass {
			continue
		}
		k.classes++
		if d.Status != domain.StatusPresent {
			k.broken = true
		}
	}
	for _, k := range weeks {
		if len(k.seen) == 5 && k.classes > 0 && !k.broken {
			return true
		}
	}
	return false
}

// BuildDays groups records by date and combines morning/afternoon, oldest first.
func BuildDays(records []domain.AttendanceRecord) []DayResult {
	type pair struct{ m, a domain.AttendanceStatus }
	byDate := map[string]*pair{}
	for _, r := range records {
		p := byDate[r.Date]
		if p == nil {
			p = &pair{}
			byDate[r.Date] = p
		}
		if r.Session == domain.SessionAfternoon {
			p.a = r.Status
		} else {
			p.m = r.Status
		}
	}
	out := make([]DayResult, 0, len(byDate))
	for d, p := range byDate {
		out = append(out, DayResult{Date: d, Status: CombineDayStatus(p.m, p.a)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date < out[j].Date })
	return out
}

type SessionToday struct {
	Status      domain.AttendanceStatus `json:"status"`
	CheckedInAt time.Time               `json:"checked_in_at"`
	Style       string                  `json:"style,omitempty"`
}

type MySummary struct {
	Today struct {
		Morning   *SessionToday `json:"morning"`
		Afternoon *SessionToday `json:"afternoon"`
	} `json:"today"`
	Streak         int     `json:"streak"`
	LeaveUsed      int     `json:"leave_used"`
	LeaveAllowance int     `json:"leave_allowance"`
	WarningLevel   string  `json:"warning_level"`
	AttendanceRate float64 `json:"attendance_rate"`
	PerfectWeek    bool    `json:"perfect_week"`
	// Champion: the learner's group won last week's GenMate Cup (Victory Lap, gold frame).
	Champion bool                `json:"champion"`
	Rules    domain.CheckinRules `json:"rules"`
	// ClassToday is false on weekends and holidays (no check-in).
	ClassToday bool `json:"class_today"`
	Course     struct {
		StartDate  string `json:"start_date"`
		CourseDays int    `json:"course_days"`
		DayNumber  int    `json:"day_number"`
		WeekNumber int    `json:"week_number"`
		TotalWeeks int    `json:"total_weeks"`
	} `json:"course"`
	Preferences struct {
		CheckinStyle  string `json:"checkin_style"`
		CheckinRandom bool   `json:"checkin_random"`
	} `json:"preferences"`
}

type progressRecords interface {
	FindRecords(ctx interface{}, filter domain.AttendanceRecordFilter, opts interface{}) ([]domain.AttendanceRecord, error)
}

type ProgressService struct {
	records  progressRecords
	users    UserServiceInterface
	settings CohortSettingsGetter
	holidays HolidayChecker
	now      func() time.Time
	champion func(domain.User) bool
}

func NewProgressService(records progressRecords, users UserServiceInterface, settings CohortSettingsGetter, holidays HolidayChecker) *ProgressService {
	return &ProgressService{records: records, users: users, settings: settings, holidays: holidays, now: utils.GetThailandTime}
}

// SetChampionCheck plugs in the GenMate Cup (kept optional to avoid a package cycle).
func (s *ProgressService) SetChampionCheck(fn func(domain.User) bool) { s.champion = fn }

func (s *ProgressService) Summary(userID primitive.ObjectID) (MySummary, error) {
	var out MySummary
	user, err := s.users.GetUserByID(userID.Hex())
	if err != nil {
		return out, ErrStudentNotFound
	}
	settings, err := s.settings.Get(user.CohortNumber)
	if err != nil {
		return out, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	recs, err := s.records.FindRecords(ctx, domain.AttendanceRecordFilter{UserID: userID, NotDeleted: true}, nil)
	if err != nil {
		return out, err
	}
	nowT := s.now()
	today := utils.FormatDate(nowT)
	if out.ClassToday, err = IsClassDay(nowT, s.holidays); err != nil {
		return out, err
	}
	for _, r := range recs {
		if r.Date != today {
			continue
		}
		st := &SessionToday{Status: r.Status, CheckedInAt: r.SubmittedAt, Style: r.CheckinStyle}
		if r.Session == domain.SessionAfternoon {
			out.Today.Afternoon = st
		} else {
			out.Today.Morning = st
		}
	}
	d := BuildDays(recs)
	out.Streak = OnTimeStreak(d)
	out.LeaveUsed = LeaveDaysUsed(d)
	out.LeaveAllowance = LeaveAllowance
	out.WarningLevel = WarningLevel(out.LeaveUsed)
	out.AttendanceRate = AttendanceRate(d)
	out.PerfectWeek = HasPerfectWeek(d)
	out.Rules = settings.Rules
	days := settings.CourseDays
	if days <= 0 {
		days = DefaultCourseDays
	}
	out.Course.StartDate, out.Course.CourseDays = settings.StartDate, days
	out.Course.DayNumber, out.Course.WeekNumber, out.Course.TotalWeeks = CourseProgress(settings.StartDate, days, today)
	out.Preferences.CheckinStyle = user.CheckinStyle
	out.Preferences.CheckinRandom = user.CheckinRandom
	if s.champion != nil {
		out.Champion = s.champion(*user)
	}
	return out, nil
}
