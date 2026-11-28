package wellbeing

import (
	"html"
	"sort"
	"strings"
	"time"

	"gofiber-baro/internal/domain"
	"gofiber-baro/internal/service/attendance"
)

// JourneyInput is everything one learner's end-of-course story is built from.
type JourneyInput struct {
	Name        string
	Cohort      string
	ClassDays   []string // sorted class days from course start to graduation
	Reflections []domain.Reflection
	Records     []domain.AttendanceRecord
	Badges      int
	QuoteID     string
}

type GridDay struct {
	Date string  `json:"date"`
	Zone *string `json:"zone"` // nil: not written; "": written, zone unknown
}

type HardestWeek struct {
	Number int      `json:"number"`
	From   string   `json:"from"`
	To     string   `json:"to"`
	Zones  []string `json:"zones"`
}

type DarkestNight struct {
	Date      string `json:"date"`
	Zone      string `json:"zone"`
	Text      string `json:"text"`
	Reply     string `json:"reply"`
	ReplyName string `json:"reply_name"`
}

type Comeback struct {
	Length       int  `json:"length"`
	AfterDarkest bool `json:"after_darkest"`
}

type Quote struct {
	ReflectionID string `json:"reflection_id"`
	Date         string `json:"date"`
	Text         string `json:"text"`
}

type Journey struct {
	Unlocked     bool           `json:"unlocked"`
	Name         string         `json:"name"`
	Cohort       string         `json:"cohort"`
	FirstDay     string         `json:"first_day"`
	LastDay      string         `json:"last_day"`
	Weeks        int            `json:"weeks"`
	ClassDays    int            `json:"class_days"`
	Written      int            `json:"written"`
	Grid         []GridDay      `json:"grid"`
	ZoneCounts   map[string]int `json:"zone_counts"`
	MainZone     string         `json:"main_zone"`
	HardestWeek  *HardestWeek   `json:"hardest_week"`
	DarkestNight *DarkestNight  `json:"darkest_night"`
	Comeback     *Comeback      `json:"comeback"`
	Attendance   *float64       `json:"attendance_rate"`
	Badges       int            `json:"badges"`
	TAReplies    int            `json:"ta_replies"`
	Quote        *Quote         `json:"quote"`
	QuoteOptions []Quote        `json:"quote_options"`
}

// minStory is how many written nights the full story needs; fewer keeps only the basics.
const minStory = 5

var zoneOrder = []string{"comfort", "stretch-enjoying", "stretch-overwhelmed", "panic"}

func monday(day string) string {
	t, err := time.Parse("2006-01-02", day)
	if err != nil {
		return day
	}
	return t.AddDate(0, 0, -((int(t.Weekday()) + 6) % 7)).Format("2006-01-02")
}

func clean(s string) string { return strings.TrimSpace(html.UnescapeString(s)) }

// BuildJourney turns one learner's course into the end-of-course story. Parts with no data are nil.
func BuildJourney(in JourneyInput) Journey {
	byDay := map[string]domain.Reflection{}
	for _, r := range in.Reflections {
		if d := dayOf(r); d != "" {
			byDay[d] = r
		}
	}
	j := Journey{Unlocked: true, Name: in.Name, Cohort: in.Cohort, ClassDays: len(in.ClassDays), Written: len(byDay), Badges: in.Badges, ZoneCounts: map[string]int{}}
	if n := len(in.ClassDays); n > 0 {
		j.FirstDay, j.LastDay = in.ClassDays[0], in.ClassDays[n-1]
	}

	zoneOf := map[string]string{}
	for d, r := range byDay {
		z := attendance.ZoneOf(r.ReflectionData.Barometer)
		zoneOf[d] = z
		if z != "" {
			j.ZoneCounts[z]++
		}
		if r.AdminFeedback != "" {
			j.TAReplies++
		}
	}
	for _, z := range zoneOrder {
		if j.ZoneCounts[z] > 0 && (j.MainZone == "" || j.ZoneCounts[z] > j.ZoneCounts[j.MainZone]) {
			j.MainZone = z
		}
	}

	weeks := []string{}
	week := map[string][]string{}
	for _, d := range in.ClassDays {
		g := GridDay{Date: d}
		if _, ok := byDay[d]; ok {
			z := zoneOf[d]
			g.Zone = &z
		}
		j.Grid = append(j.Grid, g)
		m := monday(d)
		if _, seen := week[m]; !seen {
			weeks = append(weeks, m)
		}
		week[m] = append(week[m], d)
	}
	j.Weeks = len(weeks)

	if len(in.Records) > 0 {
		rate := attendance.AttendanceRate(attendance.BuildDays(in.Records))
		j.Attendance = &rate
	}
	j.Quote, j.QuoteOptions = quotes(in.Reflections, in.QuoteID)

	if j.Written < minStory {
		return j
	}

	best, bestN := -1, 0
	for i, m := range weeks {
		n := 0
		for _, d := range week[m] {
			if hard(zoneOf[d]) {
				n++
			}
		}
		if n > bestN {
			best, bestN = i, n
		}
	}
	darkest := ""
	if best >= 0 {
		days := week[weeks[best]]
		hw := &HardestWeek{Number: best + 1, From: days[0], To: days[len(days)-1]}
		for _, d := range days {
			hw.Zones = append(hw.Zones, zoneOf[d])
		}
		j.HardestWeek = hw
		darkest = firstWhere(days, zoneOf, func(z string) bool { return z == "panic" })
		if darkest == "" {
			darkest = firstWhere(in.ClassDays, zoneOf, func(z string) bool { return z == "panic" })
		}
		if darkest == "" {
			darkest = firstWhere(days, zoneOf, hard)
		}
	}
	if darkest != "" {
		r := byDay[darkest]
		text := clean(r.ReflectionData.TechSessions.Improve)
		if text == "" {
			text = clean(r.ReflectionData.NonTechSessions.Improve)
		}
		if text == "" {
			text = joinText(r)
		}
		j.DarkestNight = &DarkestNight{Date: darkest, Zone: zoneOf[darkest], Text: text, Reply: clean(r.AdminFeedback), ReplyName: r.AdminFeedbackName}
	}

	from := 0
	if darkest != "" {
		from = sort.SearchStrings(in.ClassDays, darkest) + 1
	}
	if run := longestRun(in.ClassDays[min(from, len(in.ClassDays)):], byDay); run >= 2 {
		j.Comeback = &Comeback{Length: run, AfterDarkest: darkest != ""}
	} else if darkest != "" {
		if run := longestRun(in.ClassDays, byDay); run >= 2 {
			j.Comeback = &Comeback{Length: run}
		}
	}
	return j
}

func firstWhere(days []string, zoneOf map[string]string, ok func(string) bool) string {
	for _, d := range days {
		if z, written := zoneOf[d]; written && ok(z) {
			return d
		}
	}
	return ""
}

func longestRun(days []string, byDay map[string]domain.Reflection) int {
	best, n := 0, 0
	for _, d := range days {
		if _, ok := byDay[d]; ok {
			n++
			best = max(best, n)
		} else {
			n = 0
		}
	}
	return best
}

// quotes is the chosen sentence (or the longest "went well" answer) and up to 30 options, newest first.
func quotes(rs []domain.Reflection, chosen string) (*Quote, []Quote) {
	var opts []Quote
	var longest *Quote
	for _, r := range rs {
		d := r.ReflectionData
		for _, t := range []string{d.TechSessions.Happy, d.NonTechSessions.Happy, d.TechSessions.Improve, d.NonTechSessions.Improve} {
			if t = clean(t); t != "" {
				opts = append(opts, Quote{ReflectionID: r.ID.Hex(), Date: dayOf(r), Text: t})
				break
			}
		}
		for _, t := range []string{d.TechSessions.Happy, d.NonTechSessions.Happy} {
			if t = clean(t); t != "" && (longest == nil || len([]rune(t)) > len([]rune(longest.Text))) {
				longest = &Quote{ReflectionID: r.ID.Hex(), Date: dayOf(r), Text: t}
			}
		}
	}
	sort.SliceStable(opts, func(a, b int) bool { return opts[a].Date > opts[b].Date })
	pick := longest
	for _, q := range opts {
		if chosen != "" && q.ReflectionID == chosen {
			pick = &q
			break
		}
	}
	if len(opts) > 30 {
		opts = opts[:30]
	}
	return pick, opts
}
