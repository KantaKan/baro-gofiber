package wellbeing

import (
	"html"
	"sort"
	"time"

	"gofiber-baro/internal/domain"
	"gofiber-baro/internal/service/attendance"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// LearnerRef is the small learner summary every view uses.
type LearnerRef struct {
	ID           string         `json:"id"`
	FirstName    string         `json:"first_name"`
	ZoomName     string         `json:"zoom_name"`
	JSDNumber    string         `json:"jsd_number"`
	GenmateGroup string         `json:"genmate_group"`
	Avatar       *domain.Avatar `json:"avatar,omitempty"`
}

func Ref(u domain.User) LearnerRef {
	return LearnerRef{ID: u.ID.Hex(), FirstName: u.FirstName, ZoomName: u.ZoomName, JSDNumber: u.JSDNumber, GenmateGroup: u.GenmateGroup, Avatar: u.Avatar}
}

func Active(users []domain.User) []domain.User {
	out := make([]domain.User, 0, len(users))
	for _, u := range users {
		if u.AttendanceStatus == "dropout" || u.AttendanceStatus == "dismissed" || u.Deleted || u.Role == "admin" {
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

var weight = map[string]int{"panic": 3, "stretch-overwhelmed": 2, "stretch-enjoying": 1, "comfort": 0}

type Worth struct {
	Learner  LearnerRef `json:"learner"`
	Patterns []Pattern  `json:"patterns"`
	Strip    []string   `json:"strip"`
	Quote    string     `json:"quote"`
}
type MapRow struct {
	Learner LearnerRef `json:"learner"`
	Nights  []string   `json:"nights"`
}
type Entry struct {
	Learner        LearnerRef `json:"learner"`
	ReflectionID   string     `json:"reflection_id"`
	Day            string     `json:"day"`
	Zone           string     `json:"zone"`
	TechHappy      string     `json:"tech_happy"`
	TechImprove    string     `json:"tech_improve"`
	NonTechHappy   string     `json:"nontech_happy"`
	NonTechImprove string     `json:"nontech_improve"`
	AdminFeedback  string     `json:"admin_feedback"`
	// who replied and when (empty for replies saved before this was recorded)
	AdminFeedbackName string     `json:"admin_feedback_name,omitempty"`
	AdminFeedbackAt   *time.Time `json:"admin_feedback_at,omitempty"`
	Words             []string   `json:"words"`
}
type GroupSent struct {
	Group string `json:"group"`
	Sent  int    `json:"sent"`
	Total int    `json:"total"`
}
type Board struct {
	Date      string         `json:"date"`
	Range     string         `json:"range"`
	Sent      int            `json:"sent"`
	Total     int            `json:"total"`
	Missing   []LearnerRef   `json:"missing"`
	Zones     map[string]int `json:"zones"`
	ZonesPrev map[string]int `json:"zones_prev"`
	ByGroup   []GroupSent    `json:"by_group"`
	Worth     []Worth        `json:"worth"`
	Map       []MapRow       `json:"map"`
	Dates     []string       `json:"dates"`
	Entries   []Entry        `json:"entries"`
}

func zoneCounts() map[string]int {
	return map[string]int{"comfort": 0, "stretch-enjoying": 0, "stretch-overwhelmed": 0, "panic": 0}
}

func minus7(day string) string {
	t, err := time.Parse("2006-01-02", day)
	if err != nil {
		return ""
	}
	return t.AddDate(0, 0, -7).Format("2006-01-02")
}

func byDay(n []Night) map[string]Night {
	m := map[string]Night{}
	for _, x := range n {
		m[x.Day] = x
	}
	return m
}

// BuildBoard assembles the Reflections page. nights = recent class days, oldest first, newest = "last night".
// rng "night" | "week" (last 5 nights) | "4w" (last 20) sets which nights the zone counts cover.
func BuildBoard(users []domain.User, nights []string, rng string, words []string) Board {
	b := Board{Range: rng, Missing: []LearnerRef{}, Zones: zoneCounts(), ZonesPrev: zoneCounts(), ByGroup: []GroupSent{}, Worth: []Worth{}, Map: []MapRow{}, Entries: []Entry{}}
	if len(nights) == 0 {
		return b
	}
	b.Date = nights[len(nights)-1]
	span := map[string]int{"night": 1, "week": 5, "4w": 20}[rng]
	if span == 0 {
		span = 1
	}
	inSpan := map[string]bool{}
	for i := len(nights) - 1; i >= 0 && len(nights)-i <= span; i-- {
		inSpan[nights[i]] = true
	}
	last10 := nights
	if len(last10) > 10 {
		last10 = last10[len(last10)-10:]
	}
	b.Dates = last10
	prev := minus7(b.Date)
	groups := map[string]*GroupSent{}
	type scored struct {
		row   MapRow
		score int
	}
	var rows []scored
	for _, u := range Active(users) {
		ref := Ref(u)
		n := Nights(u.Reflections)
		by := byDay(n)
		b.Total++
		g := groups[u.GenmateGroup]
		if g == nil {
			g = &GroupSent{Group: u.GenmateGroup}
			groups[u.GenmateGroup] = g
		}
		g.Total++
		if x, ok := by[b.Date]; ok {
			b.Sent++
			g.Sent++
			for _, r := range u.Reflections {
				if dayOf(r) == b.Date {
					d := r.ReflectionData
					un := html.UnescapeString
					b.Entries = append(b.Entries, Entry{Learner: ref, ReflectionID: r.ID.Hex(), Day: b.Date, Zone: x.Zone,
						TechHappy: un(d.TechSessions.Happy), TechImprove: un(d.TechSessions.Improve), NonTechHappy: un(d.NonTechSessions.Happy), NonTechImprove: un(d.NonTechSessions.Improve),
						AdminFeedback: un(r.AdminFeedback), AdminFeedbackName: r.AdminFeedbackName, AdminFeedbackAt: r.AdminFeedbackAt, Words: Highlight(joinText(r), words)})
					break // one entry per learner per night (the last one written wins in Nights)
				}
			}
		} else {
			b.Missing = append(b.Missing, ref)
		}
		for d := range inSpan {
			if x, ok := by[d]; ok && x.Zone != "" {
				b.Zones[x.Zone]++
			}
		}
		if x, ok := by[prev]; ok && x.Zone != "" && span == 1 {
			b.ZonesPrev[x.Zone]++
		}
		row := MapRow{Learner: ref, Nights: make([]string, len(last10))}
		score := 0
		for i, d := range last10 {
			row.Nights[i] = by[d].Zone
			score += weight[by[d].Zone]
		}
		rows = append(rows, scored{row, score})
		recent := last10
		if len(recent) > 2 {
			recent = recent[len(recent)-2:]
		}
		if ps := WorthALook(n, recent, words); len(ps) > 0 {
			q := ""
			if x, ok := by[b.Date]; ok {
				q = x.Text
			}
			b.Worth = append(b.Worth, Worth{Learner: ref, Patterns: ps, Strip: row.Nights, Quote: firstLine(q)})
		}
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].score > rows[j].score })
	sev := func(w Worth) int {
		n := 0
		for _, p := range w.Patterns {
			n += map[string]int{"panic_run": 5, "words": 4, "drop": 3, "rough_stretch": 3, "silent": 1}[p.Key]
		}
		return n
	}
	sort.SliceStable(b.Worth, func(i, j int) bool { return sev(b.Worth[i]) > sev(b.Worth[j]) })
	for _, r := range rows {
		b.Map = append(b.Map, r.row)
	}
	sort.SliceStable(b.Entries, func(i, j int) bool { return weight[b.Entries[i].Zone] > weight[b.Entries[j].Zone] })
	for _, g := range groups {
		b.ByGroup = append(b.ByGroup, *g)
	}
	sort.Slice(b.ByGroup, func(i, j int) bool { return b.ByGroup[i].Group < b.ByGroup[j].Group })
	return b
}

func firstLine(s string) string {
	for _, l := range splitLines(s) {
		if l != "" {
			if r := []rune(l); len(r) > 160 {
				return string(r[:160]) + "…"
			}
			return l
		}
	}
	return ""
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i, ch := range s {
		if ch == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return append(out, s[start:])
}

// SupportRow is one learner on the Needs support list.
type SupportRow struct {
	Learner   LearnerRef `json:"learner"`
	Reasons   []Reason   `json:"reasons"`
	OpenItems int        `json:"open_items"`
	DoneItems int        `json:"done_items"`
	Owner     string     `json:"owner"`
}

// ReasonsFor computes one learner's active (not acknowledged) support reasons.
func ReasonsFor(u domain.User, recs []domain.AttendanceRecord, items []domain.SupportItem, acks []domain.SupportAck, recentClassDays []string, words []string) (reasons []Reason, open, done int) {
	for _, it := range items {
		if it.UserID != u.ID {
			continue
		}
		if it.Done {
			done++
		} else {
			open++
		}
	}
	acked := map[string]bool{}
	for _, a := range acks {
		if a.UserID == u.ID {
			acked[a.ReasonKey+"|"+a.ReasonText] = true
		}
	}
	days := attendance.BuildDays(recs)
	all := SupportReasons(attendance.LeaveDaysUsed(days), days, WorthALook(Nights(u.Reflections), recentClassDays, words), open)
	for _, r := range all {
		if !acked[r.Key+"|"+r.Text] {
			reasons = append(reasons, r)
		}
	}
	return reasons, open, done
}

// BuildSupportList lists learners with at least one active reason, most reasons first.
func BuildSupportList(users []domain.User, records []domain.AttendanceRecord, items []domain.SupportItem, acks []domain.SupportAck, recentClassDays []string, words []string) []SupportRow {
	byUser := map[primitive.ObjectID][]domain.AttendanceRecord{}
	for _, r := range records {
		byUser[r.UserID] = append(byUser[r.UserID], r)
	}
	out := []SupportRow{}
	for _, u := range Active(users) {
		rs, open, done := ReasonsFor(u, byUser[u.ID], items, acks, recentClassDays, words)
		if len(rs) == 0 {
			continue
		}
		out = append(out, SupportRow{Learner: Ref(u), Reasons: rs, OpenItems: open, DoneItems: done})
	}
	sort.SliceStable(out, func(i, j int) bool { return len(out[i].Reasons) > len(out[j].Reasons) })
	return out
}
