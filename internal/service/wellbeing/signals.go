// Package wellbeing turns reflections and attendance into the patterns admins watch for.
package wellbeing

import (
	"fmt"
	"sort"
	"strings"

	"gofiber-baro/internal/domain"
	"gofiber-baro/internal/service/attendance"
)

// Night is one written reflection: its day, zone id ("" if unknown) and all answers joined.
type Night struct {
	Day  string `json:"day"`
	Zone string `json:"zone"`
	Text string `json:"text"`
}

type Pattern struct {
	Key    string `json:"key"`
	Title  string `json:"title"`
	Detail string `json:"detail"`
}

type Reason struct {
	Key  string `json:"key"`
	Text string `json:"text"`
}

// DefaultWords are matched case-insensitively as substrings (works for Thai without spaces).
var DefaultWords = []string{"quit", "give up", "lost", "can't", "cannot", "behind", "hopeless", "ลาออก", "ไม่ไหว", "ท้อ", "เหนื่อย"}

func dayOf(r domain.Reflection) string {
	if r.Day != "" {
		return r.Day
	}
	if r.Date.IsZero() {
		return ""
	}
	return r.Date.Format("2006-01-02")
}

func joinText(r domain.Reflection) string {
	d := r.ReflectionData
	return strings.Join([]string{d.TechSessions.Happy, d.TechSessions.Improve, d.NonTechSessions.Happy, d.NonTechSessions.Improve}, "\n")
}

// Nights sorts reflections by day, keeping the last one written for each day.
func Nights(refls []domain.Reflection) []Night {
	by := map[string]Night{}
	for _, r := range refls {
		d := dayOf(r)
		if d == "" {
			continue
		}
		by[d] = Night{Day: d, Zone: attendance.ZoneOf(r.ReflectionData.Barometer), Text: joinText(r)}
	}
	out := make([]Night, 0, len(by))
	for _, n := range by {
		out = append(out, n)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Day < out[j].Day })
	return out
}

// Highlight returns the watch words found in text.
func Highlight(text string, words []string) []string {
	low := strings.ToLower(text)
	var out []string
	for _, w := range words {
		if w != "" && strings.Contains(low, strings.ToLower(w)) {
			out = append(out, w)
		}
	}
	return out
}

func hard(z string) bool { return z == "stretch-overwhelmed" || z == "panic" }

// WorthALook finds patterns (not single bad days). recentClassDays = the last 2 class dates, newest last.
func WorthALook(n []Night, recentClassDays []string, words []string) []Pattern {
	var out []Pattern
	if len(n) >= 2 && n[len(n)-1].Zone == "panic" && n[len(n)-2].Zone == "panic" {
		run := 0
		for i := len(n) - 1; i >= 0 && n[i].Zone == "panic"; i-- {
			run++
		}
		out = append(out, Pattern{"panic_run", fmt.Sprintf("Panic %d nights in a row", run), "Check in with them today."})
	}
	last10 := n
	if len(last10) > 10 {
		last10 = last10[len(last10)-10:]
	}
	h := 0
	for _, x := range last10 {
		if hard(x.Zone) {
			h++
		}
	}
	if h >= 6 {
		out = append(out, Pattern{"rough_stretch", fmt.Sprintf("Overwhelmed or panic %d of last %d", h, len(last10)), "A rough stretch, not a single bad day."})
	}
	if len(n) >= 2 && hard(n[len(n)-1].Zone) {
		prev := n[:len(n)-1]
		if len(prev) > 5 {
			prev = prev[len(prev)-5:]
		}
		c := 0
		for _, x := range prev {
			if x.Zone == "comfort" {
				c++
			}
		}
		if c >= 3 {
			out = append(out, Pattern{"drop", "Dropped from comfort to " + map[bool]string{true: "panic", false: "overwhelmed"}[n[len(n)-1].Zone == "panic"], "Was mostly comfortable before last night."})
		}
	}
	if len(recentClassDays) > 0 {
		have := map[string]bool{}
		for _, x := range n {
			have[x.Day] = true
		}
		missing := 0
		for _, d := range recentClassDays {
			if !have[d] {
				missing++
			}
		}
		if missing == len(recentClassDays) {
			out = append(out, Pattern{"silent", fmt.Sprintf("Hasn't written for %d nights", missing), "Usually a sign to check in."})
		}
	}
	if len(n) > 0 {
		if w := Highlight(n[len(n)-1].Text, words); len(w) > 0 {
			out = append(out, Pattern{"words", "Wrote “" + w[0] + "”", "Worrying words in last night's reflection."})
		}
	}
	return out
}

// SupportReasons lists why a learner needs support right now.
func SupportReasons(leaveUsed int, days []attendance.DayResult, patterns []Pattern, openItems int) []Reason {
	var out []Reason
	if leaveUsed >= 6 {
		out = append(out, Reason{"leave", fmt.Sprintf("Leave %d/8", leaveUsed)})
	}
	run := 0
	for i := len(days) - 1; i >= 0; i-- {
		s := days[i].Status
		if s == domain.StatusHoliday || s == domain.StatusNoClass {
			continue
		}
		if s != domain.StatusLate {
			break
		}
		run++
	}
	if run >= 3 {
		out = append(out, Reason{"late_run", fmt.Sprintf("Late %d days running", run)})
	}
	for _, p := range patterns {
		out = append(out, Reason{"pattern:" + p.Key, p.Title})
	}
	if openItems > 0 {
		out = append(out, Reason{"plan", fmt.Sprintf("%d support step%s open", openItems, map[bool]string{true: "", false: "s"}[openItems == 1])})
	}
	return out
}
