// Package report assembles the weekly report from data the admin already has: attendance
// register, reflection zones, the support list and the GenMate Cup standings.
package report

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"gofiber-baro/internal/domain"
	"gofiber-baro/internal/service/attendance"
	"gofiber-baro/internal/service/cup"
	"gofiber-baro/internal/service/wellbeing"
)

const trendWeeks = 8

type Input struct {
	Week        string // Monday, YYYY-MM-DD
	Today       string // Bangkok date; days after it don't count as missed
	Users       []domain.User
	Records     []domain.AttendanceRecord
	Holidays    map[string]bool // dates, for the report week and the 7 before it
	Support     []wellbeing.SupportRow
	PrevSupport map[string]bool // learner ids in the previous sent report's support list
	Cup         []cup.GroupScore
	Notes       string
}

type KPI struct {
	Value float64   `json:"value"`
	Prev  float64   `json:"prev"`
	Trend []float64 `json:"trend"` // 8 weeks, oldest to newest
}

type KPIs struct {
	Attendance  KPI `json:"attendance"`
	Lates       KPI `json:"lates"`
	Reflections KPI `json:"reflections"`
	Rough       KPI `json:"rough"`
}

type Tally struct {
	OnTime int `json:"on_time"`
	Late   int `json:"late"`
	Leave  int `json:"leave"`
	Absent int `json:"absent"`
	NotYet int `json:"not_yet"`
}

type DayRow struct {
	Day     string `json:"day"`
	Holiday bool   `json:"holiday"`
	AM      Tally  `json:"am"`
	PM      Tally  `json:"pm"`
}

type MoodDay struct {
	Day         string `json:"day"`
	Holiday     bool   `json:"holiday"`
	Comfort     int    `json:"comfort"`
	Enjoying    int    `json:"enjoying"`
	Overwhelmed int    `json:"overwhelmed"`
	Panic       int    `json:"panic"`
	None        int    `json:"none"`
}

type SupportItem struct {
	Learner wellbeing.LearnerRef `json:"learner"`
	Status  string               `json:"status"` // new | still | better
	Reasons []string             `json:"reasons"`
}

type GroupRow struct {
	Group      string  `json:"group"`
	Attendance float64 `json:"attendance"`
	Rough      float64 `json:"rough"`
	Cup        int     `json:"cup"`
}

type Report struct {
	Week               string        `json:"week"`
	WeekEnd            string        `json:"week_end"`
	Summary            string        `json:"summary"`
	KPIs               KPIs          `json:"kpis"`
	ReflectionsWritten int           `json:"reflections_written"`
	Mood               []MoodDay     `json:"mood"`
	ByDay              []DayRow      `json:"by_day"`
	Support            []SupportItem `json:"support"`
	Groups             []GroupRow    `json:"groups"`
	Notes              string        `json:"notes"`
	Status             string        `json:"status"` // draft | sent
	SentAt             *time.Time    `json:"sent_at,omitempty"`
}

func parseDay(s string) time.Time {
	t, _ := time.Parse("2006-01-02", s)
	return t
}

func weekDays(monday string) [5]string {
	var out [5]string
	m := parseDay(monday)
	for i := range out {
		out[i] = m.AddDate(0, 0, i).Format("2006-01-02")
	}
	return out
}

func ratio(a, b int) float64 {
	if b <= 0 {
		return 0
	}
	return float64(a) / float64(b)
}

// moodWeek: per-day zone counts for one week, plus reflections written / possible and rough count.
type moodWeek struct {
	days                     []MoodDay
	written, possible, rough int
	byGroup                  map[string][2]int // group -> {written, rough}
}

func zoneKey(z string) string {
	switch z {
	case "comfort", "stretch-enjoying", "stretch-overwhelmed", "panic":
		return z
	}
	return ""
}

func buildMood(users []domain.User, monday, today string, hol map[string]bool) moodWeek {
	days := weekDays(monday)
	w := moodWeek{days: make([]MoodDay, 5), byGroup: map[string][2]int{}}
	nights := make([]map[string]string, len(users)) // user -> day -> zone ("" = unknown label)
	for i, u := range users {
		nights[i] = map[string]string{}
		for _, n := range wellbeing.Nights(u.Reflections) {
			nights[i][n.Day] = n.Zone
		}
	}
	for d, day := range days {
		md := MoodDay{Day: day, Holiday: hol[day]}
		counts := day <= today && !md.Holiday
		for i, u := range users {
			zone, wrote := nights[i][day]
			if counts {
				w.possible++
			}
			if !wrote {
				if counts {
					md.None++
				}
				continue
			}
			w.written++
			g := w.byGroup[u.GenmateGroup]
			g[0]++
			switch zoneKey(zone) {
			case "comfort":
				md.Comfort++
			case "stretch-enjoying":
				md.Enjoying++
			case "stretch-overwhelmed":
				md.Overwhelmed++
				w.rough++
				g[1]++
			case "panic":
				md.Panic++
				w.rough++
				g[1]++
			}
			w.byGroup[u.GenmateGroup] = g
		}
		w.days[d] = md
	}
	return w
}

func tally(t *Tally, st domain.AttendanceStatus) {
	switch st {
	case domain.StatusPresent:
		t.OnTime++
	case domain.StatusLate:
		t.Late++
	case domain.StatusAbsentExcused, domain.StatusLateExcused:
		t.Leave++
	case domain.StatusAbsent:
		t.Absent++
	case "":
		t.NotYet++
	}
}

func attendedStatus(s domain.AttendanceStatus) bool {
	return s == domain.StatusPresent || s == domain.StatusLate || s == domain.StatusLateExcused
}

func countedStatus(s domain.AttendanceStatus) bool {
	switch s {
	case "", domain.StatusHoliday, domain.StatusNoClass, domain.StatusDropout, domain.StatusDismissed:
		return false
	}
	return true
}

// Build assembles the report for in.Week. It is pure: all data comes in through Input.
func Build(in Input) Report {
	active := wellbeing.Active(in.Users)
	days := weekDays(in.Week)
	r := Report{Week: in.Week, WeekEnd: days[4], Notes: in.Notes, Status: "draft",
		Mood: []MoodDay{}, ByDay: []DayRow{}, Support: []SupportItem{}, Groups: []GroupRow{}}

	// 8-week trends, oldest first.
	var att, lates, refl, rough [trendWeeks]float64
	var cur attendance.RegisterView
	var curMood moodWeek
	for i := 0; i < trendWeeks; i++ {
		m := parseDay(in.Week).AddDate(0, 0, -7*(trendWeeks-1-i)).Format("2006-01-02")
		v := attendance.BuildRegister(in.Users, in.Records, m, in.Holidays)
		mw := buildMood(active, m, in.Today, in.Holidays)
		att[i], lates[i] = v.KPIs.AttendanceRate, float64(v.KPIs.Lates)
		refl[i], rough[i] = ratio(mw.written, mw.possible), ratio(mw.rough, mw.written)
		if i == trendWeeks-1 {
			cur, curMood = v, mw
		}
	}
	mk := func(t [trendWeeks]float64) KPI {
		return KPI{Value: t[trendWeeks-1], Prev: t[trendWeeks-2], Trend: append([]float64{}, t[:]...)}
	}
	r.KPIs = KPIs{Attendance: mk(att), Lates: mk(lates), Reflections: mk(refl), Rough: mk(rough)}
	r.Mood, r.ReflectionsWritten = curMood.days, curMood.written

	// Attendance by day, AM and PM.
	r.ByDay = make([]DayRow, 5)
	for d, day := range days {
		row := DayRow{Day: day, Holiday: in.Holidays[day]}
		if !row.Holiday {
			for _, l := range cur.Learners {
				tally(&row.AM, l.Days[d][0].Status)
				tally(&row.PM, l.Days[d][1].Status)
			}
		}
		r.ByDay[d] = row
	}

	// Groups table.
	type gc struct{ att, tot int }
	gAtt := map[string]gc{}
	for _, l := range cur.Learners {
		g := gAtt[l.GenmateGroup]
		for d := range l.Days {
			for s := 0; s < 2; s++ {
				if st := l.Days[d][s].Status; countedStatus(st) {
					g.tot++
					if attendedStatus(st) {
						g.att++
					}
				}
			}
		}
		gAtt[l.GenmateGroup] = g
	}
	cupBy := map[string]int{}
	for _, c := range in.Cup {
		cupBy[c.Group] = c.Total
	}
	for g, a := range gAtt {
		if g == "" {
			continue
		}
		mg := curMood.byGroup[g]
		r.Groups = append(r.Groups, GroupRow{Group: g, Attendance: ratio(a.att, a.tot), Rough: ratio(mg[1], mg[0]), Cup: cupBy[g]})
	}
	sort.Slice(r.Groups, func(i, j int) bool { return r.Groups[i].Group < r.Groups[j].Group })

	// Support: New / Still / Better against the previous sent report.
	seen := map[string]bool{}
	for _, row := range in.Support {
		st := "new"
		if in.PrevSupport[row.Learner.ID] {
			st = "still"
		}
		seen[row.Learner.ID] = true
		reasons := make([]string, 0, len(row.Reasons))
		for _, re := range row.Reasons {
			reasons = append(reasons, re.Text)
		}
		r.Support = append(r.Support, SupportItem{Learner: row.Learner, Status: st, Reasons: reasons})
	}
	for _, u := range in.Users {
		if id := u.ID.Hex(); in.PrevSupport[id] && !seen[id] {
			r.Support = append(r.Support, SupportItem{Learner: wellbeing.Ref(u), Status: "better", Reasons: []string{}})
		}
	}
	order := map[string]int{"new": 0, "still": 1, "better": 2}
	sort.SliceStable(r.Support, func(i, j int) bool {
		if a, b := order[r.Support[i].Status], order[r.Support[j].Status]; a != b {
			return a < b
		}
		return r.Support[i].Learner.FirstName < r.Support[j].Learner.FirstName
	})

	r.Summary = Summary(r)
	return r
}

func pct(v float64) int { return int(math.Round(v * 100)) }

// Summary is the one-line (really, a few short sentences) overview. It never names learners.
func Summary(r Report) string {
	var parts []string
	a := r.KPIs.Attendance
	s := fmt.Sprintf("Attendance %d%%", pct(a.Value))
	if a.Prev > 0 {
		switch d := pct(a.Value) - pct(a.Prev); {
		case d > 0:
			s += fmt.Sprintf(" (up %d points from last week)", d)
		case d < 0:
			s += fmt.Sprintf(" (down %d points from last week)", -d)
		default:
			s += " (same as last week)"
		}
	}
	parts = append(parts, s+".")
	if n := int(r.KPIs.Lates.Value); n == 1 {
		parts = append(parts, "1 late check-in.")
	} else {
		parts = append(parts, fmt.Sprintf("%d late check-ins.", n))
	}
	need, fresh := 0, 0
	for _, it := range r.Support {
		if it.Status != "better" {
			need++
		}
		if it.Status == "new" {
			fresh++
		}
	}
	switch need {
	case 0:
		parts = append(parts, "No learners need support.")
	case 1:
		parts = append(parts, fmt.Sprintf("1 learner needs support (%d new).", fresh))
	default:
		parts = append(parts, fmt.Sprintf("%d learners need support (%d new).", need, fresh))
	}
	if r.ReflectionsWritten > 0 {
		parts = append(parts, fmt.Sprintf("%d%% of reflections were overwhelmed or panic.", pct(r.KPIs.Rough.Value)))
	} else {
		parts = append(parts, "No reflections yet this week.")
	}
	return strings.Join(parts, " ")
}
