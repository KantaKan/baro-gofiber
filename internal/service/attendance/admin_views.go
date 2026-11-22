package attendance

import (
	"sort"
	"strings"
	"time"

	"gofiber-baro/internal/domain"
	"gofiber-baro/pkg/utils"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// SessionCell is one learner's status for one session, as the admin views show it.
type SessionCell struct {
	Status      domain.AttendanceStatus `json:"status"` // "" = not yet
	CheckedInAt *time.Time              `json:"checked_in_at"`
	Style       string                  `json:"style,omitempty"`
	MarkedBy    string                  `json:"marked_by,omitempty"` // self | admin | system
	RecordID    string                  `json:"record_id,omitempty"`
}

type TodayLearner struct {
	ID             string         `json:"id"`
	FirstName      string         `json:"first_name"`
	LastName       string         `json:"last_name"`
	ZoomName       string         `json:"zoom_name"`
	JSDNumber      string         `json:"jsd_number"`
	GenmateGroup   string         `json:"genmate_group"`
	Avatar         *domain.Avatar `json:"avatar,omitempty"`
	Morning        SessionCell    `json:"morning"`
	Afternoon      SessionCell    `json:"afternoon"`
	Streak         int            `json:"streak"`
	LeaveUsed      int            `json:"leave_used"`
	WarningLevel   string         `json:"warning_level"`
	AttendanceRate float64        `json:"attendance_rate"`
	MoodYesterday  string         `json:"mood_yesterday"`
	Last10         []DayResult    `json:"last10"`
}

type SessionCounts struct {
	OnTime int `json:"on_time"`
	Late   int `json:"late"`
	Leave  int `json:"leave"`
	Absent int `json:"absent"`
	NotYet int `json:"not_yet"`
	Total  int `json:"total"`
}

type TodayView struct {
	Date       string                                     `json:"date"`
	ClassToday bool                                       `json:"class_today"`
	Session    domain.AttendanceSession                   `json:"session"`
	Rules      domain.CheckinRules                        `json:"rules"`
	Counts     map[domain.AttendanceSession]SessionCounts `json:"counts"`
	Learners   []TodayLearner                             `json:"learners"`
}

var zoneAliases = map[string][]string{
	"comfort":             {"comfort zone", "comfort"},
	"stretch-enjoying":    {"stretch zone - enjoying the challenges", "stretch-enjoying", "enjoying"},
	"stretch-overwhelmed": {"stretch zone - overwhelmed", "stretch-overwhelmed", "overwhelmed"},
	"panic":               {"panic zone", "panic"},
}

// ZoneOf maps a stored barometer label to the learner-side zone id ("" if unknown).
func ZoneOf(label string) string {
	l := strings.ToLower(strings.TrimSpace(label))
	for id, as := range zoneAliases {
		if l == id {
			return id
		}
		for _, a := range as {
			if l == a {
				return id
			}
		}
	}
	return ""
}

func activeLearners(users []domain.User) []domain.User {
	out := make([]domain.User, 0, len(users))
	for _, u := range users {
		if u.AttendanceStatus == "dropout" || u.AttendanceStatus == "dismissed" || u.Deleted {
			continue
		}
		out = append(out, u)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].GenmateGroup != out[j].GenmateGroup {
			return out[i].GenmateGroup < out[j].GenmateGroup
		}
		return out[i].JSDNumber < out[j].JSDNumber
	})
	return out
}

func cellOf(r domain.AttendanceRecord) SessionCell {
	c := SessionCell{Status: r.Status, Style: r.CheckinStyle, RecordID: r.ID.Hex()}
	switch {
	case r.MarkedBy == domain.MarkedBySelf:
		c.MarkedBy = "self"
		if !r.SubmittedAt.IsZero() {
			t := r.SubmittedAt
			c.CheckedInAt = &t
		}
	case r.MarkedByUser == "system":
		c.MarkedBy = "system"
	default:
		c.MarkedBy = "admin"
	}
	return c
}

func reflectionDay(r domain.Reflection) string {
	if r.Day != "" {
		return r.Day
	}
	if r.Date.IsZero() {
		return ""
	}
	return utils.FormatDate(r.Date.In(bangkokLoc()))
}

func bangkokLoc() *time.Location {
	if l, err := time.LoadLocation("Asia/Bangkok"); err == nil {
		return l
	}
	return time.FixedZone("ICT", 7*3600)
}

// moodBefore is the zone of the learner's most recent reflection before today.
func moodBefore(refls []domain.Reflection, today string) string {
	best, zone := "", ""
	for _, r := range refls {
		d := reflectionDay(r)
		if d == "" || d >= today || d < best {
			continue
		}
		best, zone = d, ZoneOf(r.ReflectionData.Barometer)
	}
	return zone
}

func countCell(c *SessionCounts, s domain.AttendanceStatus) {
	switch s {
	case domain.StatusHoliday, domain.StatusNoClass, domain.StatusDropout, domain.StatusDismissed:
		return // not a class session for this learner
	}
	c.Total++
	switch s {
	case domain.StatusPresent:
		c.OnTime++
	case domain.StatusLate:
		c.Late++
	case domain.StatusLateExcused, domain.StatusAbsentExcused:
		c.Leave++
	case domain.StatusAbsent:
		c.Absent++
	case "":
		c.NotYet++
	}
}

func last(days []DayResult, n int) []DayResult {
	if len(days) > n {
		days = days[len(days)-n:]
	}
	out := make([]DayResult, len(days))
	copy(out, days)
	return out
}

// BuildTodayRoster assembles the admin Today board for one cohort.
func BuildTodayRoster(learners []domain.User, records []domain.AttendanceRecord, now time.Time, rules domain.CheckinRules, classToday bool) TodayView {
	now = now.In(bangkokLoc())
	today := utils.FormatDate(now)
	byUser := map[primitive.ObjectID][]domain.AttendanceRecord{}
	for _, r := range records {
		byUser[r.UserID] = append(byUser[r.UserID], r)
	}
	v := TodayView{Date: today, ClassToday: classToday, Session: rules.SessionAt(now), Rules: rules,
		Counts: map[domain.AttendanceSession]SessionCounts{}, Learners: []TodayLearner{}}
	var am, pm SessionCounts
	for _, u := range activeLearners(learners) {
		recs := byUser[u.ID]
		l := TodayLearner{ID: u.ID.Hex(), FirstName: u.FirstName, LastName: u.LastName, ZoomName: u.ZoomName,
			JSDNumber: u.JSDNumber, GenmateGroup: u.GenmateGroup, Avatar: u.Avatar}
		for _, r := range recs {
			if r.Date != today {
				continue
			}
			if r.Session == domain.SessionAfternoon {
				l.Afternoon = cellOf(r)
			} else {
				l.Morning = cellOf(r)
			}
		}
		days := BuildDays(recs)
		l.Streak = OnTimeStreak(days)
		l.LeaveUsed = LeaveDaysUsed(days)
		l.WarningLevel = WarningLevel(l.LeaveUsed)
		l.AttendanceRate = AttendanceRate(days)
		l.Last10 = last(days, 10)
		l.MoodYesterday = moodBefore(u.Reflections, today)
		countCell(&am, l.Morning.Status)
		countCell(&pm, l.Afternoon.Status)
		v.Learners = append(v.Learners, l)
	}
	v.Counts[domain.SessionMorning] = am
	v.Counts[domain.SessionAfternoon] = pm
	return v
}

type RegisterLearner struct {
	ID           string            `json:"id"`
	FirstName    string            `json:"first_name"`
	LastName     string            `json:"last_name"`
	ZoomName     string            `json:"zoom_name"`
	JSDNumber    string            `json:"jsd_number"`
	GenmateGroup string            `json:"genmate_group"`
	Avatar       *domain.Avatar    `json:"avatar,omitempty"`
	Days         [5][2]SessionCell `json:"days"`
	WeekRate     float64           `json:"week_rate"`
	LeaveUsed    int               `json:"leave_used"`
}

type RegisterKPIs struct {
	AttendanceRate float64 `json:"attendance_rate"`
	Lates          int     `json:"lates"`
	Absences       int     `json:"absences"`
	Excused        int     `json:"excused"`
	AtSixPlus      int     `json:"at_six_plus"`
	AtSeven        int     `json:"at_seven"`
}

type RegisterView struct {
	Week     string            `json:"week"`
	Dates    [5]string         `json:"dates"`
	Holidays []string          `json:"holidays"`
	KPIs     RegisterKPIs      `json:"kpis"`
	Previous RegisterKPIs      `json:"previous"`
	Learners []RegisterLearner `json:"learners"`
}

func attended(s domain.AttendanceStatus) bool {
	return s == domain.StatusPresent || s == domain.StatusLate || s == domain.StatusLateExcused
}

func weekDates(week string) ([5]string, bool) {
	var out [5]string
	m, err := time.ParseInLocation("2006-01-02", week, bangkokLoc())
	if err != nil {
		return out, false
	}
	for i := 0; i < 5; i++ {
		out[i] = m.AddDate(0, 0, i).Format("2006-01-02")
	}
	return out, true
}

// weekKPIs counts one week's records across learners. Rate = attended / recorded sessions
// (the session closer writes a record for every past class session).
func weekKPIs(learners []domain.User, byUser map[primitive.ObjectID][]domain.AttendanceRecord, dates [5]string) RegisterKPIs {
	in := map[string]bool{}
	for _, d := range dates {
		in[d] = true
	}
	var k RegisterKPIs
	att, tot := 0, 0
	for _, u := range learners {
		for _, r := range byUser[u.ID] {
			if !in[r.Date] {
				continue
			}
			switch r.Status {
			case domain.StatusHoliday, domain.StatusNoClass, domain.StatusDropout, "":
				continue
			case domain.StatusLate:
				k.Lates++
			case domain.StatusAbsent:
				k.Absences++
			case domain.StatusAbsentExcused, domain.StatusLateExcused:
				k.Excused++
			}
			tot++
			if attended(r.Status) {
				att++
			}
		}
	}
	if tot > 0 {
		k.AttendanceRate = float64(att) / float64(tot)
	}
	return k
}

// BuildRegister assembles one week (Mon–Fri) of AM/PM statuses for a cohort.
func BuildRegister(learners []domain.User, records []domain.AttendanceRecord, week string, holidays map[string]bool) RegisterView {
	dates, _ := weekDates(week)
	v := RegisterView{Week: week, Dates: dates, Holidays: []string{}, Learners: []RegisterLearner{}}
	idx := map[string]int{}
	for i, d := range dates {
		idx[d] = i
		if holidays[d] {
			v.Holidays = append(v.Holidays, d)
		}
	}
	byUser := map[primitive.ObjectID][]domain.AttendanceRecord{}
	for _, r := range records {
		byUser[r.UserID] = append(byUser[r.UserID], r)
	}
	active := activeLearners(learners)
	for _, u := range active {
		l := RegisterLearner{ID: u.ID.Hex(), FirstName: u.FirstName, LastName: u.LastName, ZoomName: u.ZoomName,
			JSDNumber: u.JSDNumber, GenmateGroup: u.GenmateGroup, Avatar: u.Avatar}
		att, tot := 0, 0
		for _, r := range byUser[u.ID] {
			i, ok := idx[r.Date]
			if !ok {
				continue
			}
			s := 0
			if r.Session == domain.SessionAfternoon {
				s = 1
			}
			l.Days[i][s] = cellOf(r)
			switch r.Status {
			case domain.StatusHoliday, domain.StatusNoClass, domain.StatusDropout:
				continue
			}
			tot++
			if attended(r.Status) {
				att++
			}
		}
		if tot > 0 {
			l.WeekRate = float64(att) / float64(tot)
		}
		l.LeaveUsed = LeaveDaysUsed(BuildDays(byUser[u.ID]))
		v.Learners = append(v.Learners, l)
	}
	v.KPIs = weekKPIs(active, byUser, dates)
	if m, err := time.ParseInLocation("2006-01-02", week, bangkokLoc()); err == nil {
		prev, _ := weekDates(m.AddDate(0, 0, -7).Format("2006-01-02"))
		v.Previous = weekKPIs(active, byUser, prev)
	}
	for _, l := range v.Learners {
		if l.LeaveUsed >= 6 {
			v.KPIs.AtSixPlus++
		}
		if l.LeaveUsed >= 7 {
			v.KPIs.AtSeven++
		}
	}
	return v
}
