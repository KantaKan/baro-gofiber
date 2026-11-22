package calendar

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// WeekDiff is how one week differs between the saved calendar and a freshly read file.
type WeekDiff struct {
	N         int      `json:"n"`
	Name      string   `json:"name"`
	Status    string   `json:"status"` // new | changed | same | removed
	Published bool     `json:"published"`
	Added     int      `json:"added"`
	Removed   int      `json:"removed"`
	Lines     []string `json:"lines"` // up to 6 human-readable changes
}

const maxDiffLines = 6

func sessionKey(date string, s Session) string {
	return strings.Join([]string{date, s.Start, s.End, s.Code, s.Title, s.Subtitle, s.Type, s.Kind, strings.Join(s.Facilitators, "/")}, "|")
}

func countSessions(w Week) int {
	n := 0
	for _, d := range w.Days {
		for _, s := range d.Sessions {
			if s.Kind != "break" {
				n++
			}
		}
	}
	return n
}

func dayLabel(date string) string {
	if t, err := time.Parse("2006-01-02", date); err == nil {
		return t.Format("Mon 2 Jan")
	}
	return date
}

type entry struct {
	date string
	s    Session
}

func entries(w Week) map[string]entry {
	m := map[string]entry{}
	for _, d := range w.Days {
		for _, s := range d.Sessions {
			m[sessionKey(d.Date, s)] = entry{d.Date, s}
		}
	}
	return m
}

func holidays(w Week) map[string]bool {
	m := map[string]bool{}
	for _, d := range w.Days {
		if d.Holiday {
			m[d.Date] = true
		}
	}
	return m
}

func line(sign string, e entry) string {
	what := strings.TrimSpace(e.s.Code + " " + e.s.Title)
	return fmt.Sprintf("%s %s %s–%s %s", sign, dayLabel(e.date), e.s.Start, e.s.End, what)
}

func sorted(es []entry) []entry {
	sort.Slice(es, func(i, j int) bool {
		if es[i].date != es[j].date {
			return es[i].date < es[j].date
		}
		return es[i].s.Start < es[j].s.Start
	})
	return es
}

// Diff compares a saved calendar with a new one, week by week (matched by week number).
func Diff(old, cur []Week) []WeekDiff {
	oldBy := map[int]Week{}
	for _, w := range old {
		oldBy[w.N] = w
	}
	var out []WeekDiff
	seen := map[int]bool{}
	for _, w := range cur {
		seen[w.N] = true
		o, had := oldBy[w.N]
		d := WeekDiff{N: w.N, Name: w.Name, Lines: []string{}, Published: had && o.Published}
		if !had {
			d.Status, d.Added = "new", countSessions(w)
			out = append(out, d)
			continue
		}
		oe, ne := entries(o), entries(w)
		var added, removed []entry
		for k, e := range ne {
			if _, ok := oe[k]; !ok {
				added = append(added, e)
			}
		}
		for k, e := range oe {
			if _, ok := ne[k]; !ok {
				removed = append(removed, e)
			}
		}
		var holidayLines []string
		oh, nh := holidays(o), holidays(w)
		for date := range nh {
			if !oh[date] {
				holidayLines = append(holidayLines, fmt.Sprintf("+ %s is a holiday", dayLabel(date)))
			}
		}
		for date := range oh {
			if !nh[date] {
				holidayLines = append(holidayLines, fmt.Sprintf("− %s is no longer a holiday", dayLabel(date)))
			}
		}
		sort.Strings(holidayLines)
		d.Added, d.Removed = len(added), len(removed)
		lines := holidayLines
		for _, e := range sorted(removed) {
			lines = append(lines, line("−", e))
		}
		for _, e := range sorted(added) {
			lines = append(lines, line("+", e))
		}
		if len(lines) > maxDiffLines {
			lines = append(lines[:maxDiffLines-1], fmt.Sprintf("…and %d more changes", len(lines)-(maxDiffLines-1)))
		}
		d.Lines = lines
		d.Status = "same"
		if len(added)+len(removed)+len(holidayLines) > 0 {
			d.Status = "changed"
		}
		out = append(out, d)
	}
	for _, o := range old {
		if !seen[o.N] {
			out = append(out, WeekDiff{N: o.N, Name: o.Name, Status: "removed", Published: o.Published, Removed: countSessions(o), Lines: []string{}})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].N < out[j].N })
	return out
}
