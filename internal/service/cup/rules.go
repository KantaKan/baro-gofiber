// Package cup runs the weekly GenMate Cup: group points, standings and the Friday close.
package cup

import (
	"sort"
	"time"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	CatCoding    = "coding"
	CatSoft      = "soft"
	CatShowingUp = "showing_up"
	CatHelping   = "helping"

	ShowingUpPoints = 15
)

// Categories in display order.
var Categories = []string{CatCoding, CatSoft, CatShowingUp, CatHelping}

var bangkok = func() *time.Location {
	l, err := time.LoadLocation("Asia/Bangkok")
	if err != nil {
		return time.FixedZone("ICT", 7*3600)
	}
	return l
}()

const dateFmt = "2006-01-02"

// WeekStart returns the Monday (Bangkok) of t's week as YYYY-MM-DD.
func WeekStart(t time.Time) string {
	t = t.In(bangkok)
	back := (int(t.Weekday()) + 6) % 7 // Mon=0 … Sun=6
	return time.Date(t.Year(), t.Month(), t.Day()-back, 0, 0, 0, 0, bangkok).Format(dateFmt)
}

func parseDay(d string) (time.Time, bool) {
	t, err := time.ParseInLocation(dateFmt, d, bangkok)
	return t, err == nil
}

// WeekEnd is Friday 16:45 Bangkok of the week starting on Monday `week`.
func WeekEnd(week string) time.Time {
	m, _ := parseDay(week)
	return time.Date(m.Year(), m.Month(), m.Day()+4, 16, 45, 0, 0, bangkok)
}

// CurrentWeek is the week that is still open at t: after Friday 16:45 (and over the
// weekend) points and standings belong to next week.
func CurrentWeek(t time.Time) string {
	w := WeekStart(t)
	if !t.Before(WeekEnd(w)) {
		m, _ := parseDay(w)
		return m.AddDate(0, 0, 7).Format(dateFmt)
	}
	return w
}

// WeekNo counts weeks from the cohort start (week 1 = the start's week); ISO week if no start.
func WeekNo(start, week string) int {
	w, ok := parseDay(week)
	if !ok {
		return 0
	}
	s, ok := parseDay(start)
	if !ok {
		_, n := w.ISOWeek()
		return n
	}
	first, _ := parseDay(WeekStart(s))
	n := int(w.Sub(first).Hours()/24)/7 + 1
	if n < 1 {
		return 1
	}
	return n
}

// GroupScore is one group's week.
type GroupScore struct {
	Group      string         `json:"group"`
	Total      int            `json:"total"`
	ByCategory map[string]int `json:"by_category"`
	ByDay      [5]int         `json:"by_day"` // Mon..Fri
}

// Standings ranks groups: total desc, then coding desc, then name. Groups without awards get 0.
func Standings(awards []domain.CupAward, groups []string, week string) []GroupScore {
	idx := map[string]*GroupScore{}
	order := []string{}
	add := func(g string) *GroupScore {
		if s, ok := idx[g]; ok {
			return s
		}
		s := &GroupScore{Group: g, ByCategory: map[string]int{}}
		for _, c := range Categories {
			s.ByCategory[c] = 0
		}
		idx[g] = s
		order = append(order, g)
		return s
	}
	for _, g := range groups {
		if g != "" {
			add(g)
		}
	}
	mon, _ := parseDay(week)
	for _, a := range awards {
		if a.Group == "" {
			continue
		}
		s := add(a.Group)
		s.Total += a.Points
		s.ByCategory[a.Category] += a.Points
		if d, ok := parseDay(a.Day); ok {
			if i := int(d.Sub(mon).Hours() / 24); i >= 0 && i < 5 {
				s.ByDay[i] += a.Points
			}
		}
	}
	out := make([]GroupScore, 0, len(order))
	for _, g := range order {
		out = append(out, *idx[g])
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Total != b.Total {
			return a.Total > b.Total
		}
		if a.ByCategory[CatCoding] != b.ByCategory[CatCoding] {
			return a.ByCategory[CatCoding] > b.ByCategory[CatCoding]
		}
		return a.Group < b.Group
	})
	return out
}

// Winner is the top group, or false when nobody scored.
func Winner(s []GroupScore) (string, bool) {
	if len(s) == 0 || s[0].Total <= 0 {
		return "", false
	}
	return s[0].Group, true
}

// WholeGroupOnTime: every member has both sessions present or excused.
func WholeGroupOnTime(members []primitive.ObjectID, statuses map[primitive.ObjectID][]domain.AttendanceStatus) bool {
	if len(members) == 0 {
		return false
	}
	for _, m := range members {
		st := statuses[m]
		if len(st) < 2 {
			return false
		}
		for _, s := range st {
			if s != domain.StatusPresent && s != domain.StatusLateExcused && s != domain.StatusAbsentExcused {
				return false
			}
		}
	}
	return true
}
