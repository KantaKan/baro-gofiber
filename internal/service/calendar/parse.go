// Package calendar reads the program's cohort calendar spreadsheet into a saved blueprint,
// diffs re-imports and serves published weeks to learners.
package calendar

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/xuri/excelize/v2"
)

const (
	slotMinutes = 15
	slotsPerDay = 32 // 09:00–17:00
	dayStart    = 9 * 60
	dayEnd      = dayStart + slotsPerDay*slotMinutes
	maxRows     = 6000
	maxMerges   = 20000
)

type Session struct {
	Start        string   `bson:"start" json:"start"`
	End          string   `bson:"end" json:"end"`
	Code         string   `bson:"code" json:"code"`
	Title        string   `bson:"title" json:"title"`
	Subtitle     string   `bson:"subtitle" json:"subtitle"`
	Facilitators []string `bson:"facilitators" json:"facilitators"`
	Type         string   `bson:"type" json:"type"`
	Kind         string   `bson:"kind" json:"kind"` // session | break
}

type Day struct {
	Date     string    `bson:"date" json:"date"`
	Label    string    `bson:"label" json:"label"`
	Weekday  int       `bson:"weekday" json:"weekday"` // 0 = Monday
	Holiday  bool      `bson:"holiday" json:"holiday"`
	Sessions []Session `bson:"sessions" json:"sessions"`
}

type Week struct {
	N           int        `bson:"n" json:"n"`
	Name        string     `bson:"name" json:"name"`
	Monday      string     `bson:"monday" json:"monday"`
	Published   bool       `bson:"published" json:"published"`
	PublishedAt *time.Time `bson:"published_at,omitempty" json:"published_at,omitempty"`
	Days        []Day      `bson:"days" json:"days"`
}

type Warning struct {
	Level   string `json:"level"` // warn | info
	Week    int    `json:"week"`
	Day     string `json:"day,omitempty"`
	Message string `json:"message"`
}

type Parsed struct {
	Weeks    []Week    `json:"weeks"`
	Warnings []Warning `json:"warnings"`
	ZoomURL  string    `json:"zoom_url"`
}

// Session types, matching the key at the top of the sheet.
var typeByCode = map[string]string{
	"BSM": "behavioral", "T": "technical", "SA": "skills_application", "EE": "employment",
	"RR": "reinforcement", "C": "community", "SSM": "social_support",
}

var typeNames = map[string]string{
	"behavioral": "Behavioral Skills & Mindset", "technical": "Technical Skills", "skills_application": "Skills Application",
	"employment": "Employment Essentials", "reinforcement": "Reinforcement & Reflection", "community": "Community",
	"social_support": "Social Support & Mentorship", "orientation": "Orientation",
}

// Colours of the key in the real file; used when the sheet's own key can't be read.
var defaultKey = map[string]string{
	"FA67FA": "behavioral", "3BCAFD": "technical", "92D050": "skills_application", "FFFF00": "employment",
	"FFC000": "reinforcement", "F4CCCC": "community", "B6B6FF": "social_support", "C9DAF8": "orientation",
}

var (
	weekRe  = regexp.MustCompile(`(?i)^week\s*(\d+)`)
	dayRe   = regexp.MustCompile(`(?i)^day\s*(\d+)`)
	facRe   = regexp.MustCompile(`(?s)\s*[-–—]\s*\(([^()]*)\)\s*$`)
	codeRe  = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]*(-[A-Za-z0-9]+)+$|^[A-Za-z]+[0-9]+$`)
	keyTag  = regexp.MustCompile(`\(([A-Za-z]+)\)`)
	dateRe1 = regexp.MustCompile(`(?i)^(\d{1,2})[\s-]*([a-z]{3,})`)
	dateRe2 = regexp.MustCompile(`(?i)^([a-z]{3,})[\s-]*(\d{1,2})`)
	months  = map[string]time.Month{"jan": 1, "feb": 2, "mar": 3, "apr": 4, "may": 5, "jun": 6, "jul": 7, "aug": 8, "sep": 9, "oct": 10, "nov": 11, "dec": 12}
)

func hhmm(min int) string { return fmt.Sprintf("%02d:%02d", min/60, min%60) }

func mondayOf(t time.Time) time.Time {
	return t.AddDate(0, 0, -((int(t.Weekday()) + 6) % 7))
}

// parseDate reads "29-Jun" or "Aug 3" into a month and day (the sheet has no year).
func parseDate(s string) (time.Month, int, bool) {
	s = strings.TrimSpace(s)
	if m := dateRe1.FindStringSubmatch(s); m != nil {
		if mo, ok := months[strings.ToLower(m[2])[:3]]; ok {
			d, _ := strconv.Atoi(m[1])
			return mo, d, true
		}
	}
	if m := dateRe2.FindStringSubmatch(s); m != nil {
		if mo, ok := months[strings.ToLower(m[1])[:3]]; ok {
			d, _ := strconv.Atoi(m[2])
			return mo, d, true
		}
	}
	return 0, 0, false
}

type rgb struct{ r, g, b int }

func parseRGB(h string) (rgb, bool) {
	if len(h) < 6 {
		return rgb{}, false
	}
	h = h[len(h)-6:]
	v, err := strconv.ParseUint(h, 16, 32)
	if err != nil {
		return rgb{}, false
	}
	return rgb{int(v >> 16 & 255), int(v >> 8 & 255), int(v & 255)}, true
}

func dist(a, b rgb) float64 {
	return math.Sqrt(float64((a.r-b.r)*(a.r-b.r) + (a.g-b.g)*(a.g-b.g) + (a.b-b.b)*(a.b-b.b)))
}

type reader struct {
	f      *excelize.File
	sheet  string
	styles map[int]string
}

// fill returns the cell's fill colour as upper-case RRGGBB ("" for none or white).
func (r *reader) fill(cell string) string {
	id, err := r.f.GetCellStyle(r.sheet, cell)
	if err != nil {
		return ""
	}
	if c, ok := r.styles[id]; ok {
		return c
	}
	col := ""
	if st, err := r.f.GetStyle(id); err == nil && st != nil && len(st.Fill.Color) > 0 {
		col = strings.ToUpper(strings.TrimPrefix(st.Fill.Color[0], "#"))
		if len(col) > 6 {
			col = col[len(col)-6:]
		}
		if col == "FFFFFF" {
			col = ""
		}
	}
	r.styles[id] = col
	return col
}

func (r *reader) value(cell string) string {
	v, _ := r.f.GetCellValue(r.sheet, cell)
	return strings.TrimSpace(v)
}

// readKey builds colour → type from the key in rows 1–3 (falls back to the standard key).
func (r *reader) readKey() map[string]string {
	key := map[string]string{}
	for _, cell := range []string{"C1", "D1", "E1", "C2", "D2", "E2", "C3", "D3", "E3"} {
		txt, col := r.value(cell), r.fill(cell)
		if txt == "" || col == "" {
			continue
		}
		slug := ""
		if m := keyTag.FindStringSubmatch(txt); m != nil {
			slug = typeByCode[strings.ToUpper(m[1])]
		}
		if slug == "" && strings.Contains(strings.ToLower(txt), "orientation") {
			slug = "orientation"
		}
		if slug != "" {
			key[col] = slug
		}
	}
	if len(key) < 4 {
		return defaultKey
	}
	return key
}

func codeType(code string) string {
	return typeByCode[strings.ToUpper(strings.SplitN(code, "-", 2)[0])]
}

// parseText splits "CODE  Title\n(subtitle)\n- (Fac/Fac)" into its parts.
func parseText(text string) (code, title, subtitle string, facs []string) {
	t := strings.ReplaceAll(text, "\r", "")
	if m := facRe.FindStringSubmatchIndex(t); m != nil {
		for _, p := range strings.FieldsFunc(t[m[2]:m[3]], func(r rune) bool { return r == '/' || r == ',' }) {
			if p = strings.TrimSpace(p); p != "" {
				facs = append(facs, p)
			}
		}
		t = t[:m[0]]
	}
	var lines []string
	for _, l := range strings.Split(t, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			lines = append(lines, l)
		}
	}
	if len(lines) == 0 {
		return "", "", "", facs
	}
	first := lines[0]
	subtitle = strings.Join(lines[1:], " ")
	if f := strings.Fields(first); len(f) > 0 && codeRe.MatchString(f[0]) {
		code = f[0]
		first = strings.TrimLeft(strings.TrimSpace(strings.TrimPrefix(first, f[0])), "-–— ")
		if first == "" {
			first = code
		}
	}
	return code, first, subtitle, facs
}

func titleCase(s string) string {
	if s == "" {
		return s
	}
	r := []rune(strings.ToLower(s))
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

func breakKind(text string) string {
	u := strings.ToUpper(strings.TrimSpace(text))
	for _, k := range []string{"LUNCH", "FLEX", "BUFFER"} {
		if u == k || strings.HasPrefix(u, k+" ") {
			return k
		}
	}
	return ""
}

type cellInfo struct {
	rows int
	text string
	fill string
}

// ParseSheet reads one cohort sheet. Dates come from the cohort start date (week n starts
// start + 7(n−1) days), because the sheet's own dates have no year and carry mistakes.
func ParseSheet(f *excelize.File, sheet, startDate string) (Parsed, error) {
	start, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		return Parsed{}, fmt.Errorf("the cohort needs a start date first (set it in Check-in rules)")
	}
	monday1 := mondayOf(start)
	if idx, _ := f.GetSheetIndex(sheet); idx < 0 {
		return Parsed{}, fmt.Errorf("sheet %q not found in this file", sheet)
	}
	rows, err := f.GetRows(sheet)
	if err != nil {
		return Parsed{}, fmt.Errorf("couldn't read sheet %q", sheet)
	}
	if len(rows) > maxRows {
		return Parsed{}, fmt.Errorf("sheet %q is too large (%d rows)", sheet, len(rows))
	}
	rd := &reader{f: f, sheet: sheet, styles: map[int]string{}}
	key := rd.readKey()
	out := Parsed{Weeks: []Week{}, Warnings: []Warning{}}
	warn := func(week int, day, msg string) { out.Warnings = append(out.Warnings, Warning{"warn", week, day, msg}) }

	for _, cell := range []string{"H1", "H2", "H3", "H4"} {
		if ok, target, _ := f.GetCellHyperLink(sheet, cell); ok && strings.HasPrefix(target, "https://") && strings.Contains(strings.ToLower(rd.value(cell)+target), "zoom") {
			out.ZoomURL = target
			break
		}
	}

	// Merged ranges: per (col,row) the session text and height; covered cells are skipped.
	info := map[[2]int]cellInfo{}
	covered := map[[2]int]bool{}
	merges, _ := f.GetMergeCells(sheet)
	if len(merges) > maxMerges {
		return Parsed{}, fmt.Errorf("sheet %q has too many merged cells (%d)", sheet, len(merges))
	}
	for _, m := range merges {
		sc, sr, e1 := excelize.CellNameToCoordinates(m.GetStartAxis())
		ec, er, e2 := excelize.CellNameToCoordinates(m.GetEndAxis())
		if e1 != nil || e2 != nil || sc > 6 || ec < 2 {
			continue
		}
		if er > sr+slotsPerDay { // only a day's 32 slots matter; a crafted file may claim millions of rows
			er = sr + slotsPerDay
		}
		anchor := m.GetStartAxis()
		ci := cellInfo{rows: er - sr + 1, text: strings.TrimSpace(m.GetCellValue()), fill: rd.fill(anchor)}
		for c := sc; c <= ec; c++ {
			if c < 2 || c > 6 {
				continue
			}
			info[[2]int{c, sr}] = ci
			for r := sr + 1; r <= er; r++ {
				covered[[2]int{c, r}] = true
			}
			if c != sc {
				covered[[2]int{c, sr}] = false
			}
		}
	}

	colours := colourStats{}
	seenWeeks := map[int]bool{}
	prevFirstDay := 0
	for i, row := range rows {
		if len(row) < 2 {
			continue
		}
		wm := weekRe.FindStringSubmatch(strings.TrimSpace(row[0]))
		dm := dayRe.FindStringSubmatch(strings.TrimSpace(row[1]))
		if wm == nil || dm == nil {
			continue // the template block ("DATE") and everything else
		}
		n, _ := strconv.Atoi(wm[1])
		if n <= 0 {
			warn(0, "", "Week 0 was skipped: weeks are numbered from 1, so it can't be published. Add it as week 1 or leave it out.")
			continue
		}
		if seenWeeks[n] {
			warn(n, "", fmt.Sprintf("Week %d appears twice in the sheet; only the first one is used.", n))
			continue
		}
		seenWeeks[n] = true
		firstDay, _ := strconv.Atoi(dm[1])
		header := i + 1
		dateRow := header + 2
		if dateRow > len(rows) || len(rows[dateRow-1]) == 0 || !strings.EqualFold(strings.TrimSpace(rows[dateRow-1][0]), "facilitator") {
			warn(n, "", fmt.Sprintf("Week %d was skipped: it needs a Facilitator row (with the dates) two rows under the week heading.", n))
			continue
		}
		if prevFirstDay > 0 && firstDay <= prevFirstDay+4 {
			warn(n, "", fmt.Sprintf("DAY numbers repeat: the previous week starts at DAY %d and week %d starts at DAY %d.", prevFirstDay, n, firstDay))
		}
		prevFirstDay = firstDay

		mon := monday1.AddDate(0, 0, 7*(n-1))
		w := Week{N: n, Name: strings.ToUpper(strings.TrimSpace(strings.Split(row[0], "\n")[0])), Monday: mon.Format("2006-01-02"), Days: []Day{}}
		first := header + 3
		mismatch := false
		for d := 0; d < 5; d++ {
			col := 2 + d
			date := mon.AddDate(0, 0, d)
			if mo, dy, ok := parseDate(rd.value(cellName(col, dateRow))); ok && (mo != date.Month() || dy != date.Day()) && !mismatch {
				mismatch = true
				warn(n, "", fmt.Sprintf("The sheet says %s %d for %s, but the cohort start date puts it on %s. Dates follow the cohort calendar.", mo.String()[:3], dy, []string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday"}[d], date.Format("2006-01-02")))
			}
			day := Day{Date: date.Format("2006-01-02"), Label: rd.value(cellName(col, header)), Weekday: d, Sessions: []Session{}}
			for r := first; r < first+slotsPerDay; r++ {
				if covered[[2]int{col, r}] {
					continue
				}
				ci, ok := info[[2]int{col, r}]
				if !ok {
					cell := cellName(col, r)
					ci = cellInfo{rows: 1, text: rd.value(cell), fill: rd.fill(cell)}
				}
				if ci.text == "" {
					continue
				}
				if strings.HasPrefix(strings.ToLower(ci.text), "holiday") {
					day.Holiday = true
					continue
				}
				startMin := dayStart + (r-first)*slotMinutes
				endMin := startMin + ci.rows*slotMinutes
				s := Session{Start: hhmm(startMin), End: hhmm(endMin), Facilitators: []string{}, Kind: "session", Type: "other"}
				if k := breakKind(ci.text); k != "" {
					s.Kind, s.Type, s.Title = "break", "break", titleCase(k)
				} else {
					s.Code, s.Title, s.Subtitle, s.Facilitators = parseText(ci.text)
					if s.Facilitators == nil {
						s.Facilitators = []string{}
					}
					s.Type = typeOf(key, ci.fill, s.Code, func(near string) { colours.add(ci.fill, near, n) })
				}
				if endMin > dayEnd {
					warn(n, day.Date, fmt.Sprintf("%s %s ends after 17:00 (%s); check the merged cells in the sheet.", day.Label, strings.TrimSpace(s.Code+" "+s.Title), s.End))
					s.End = hhmm(dayEnd) // never show a session running past the end of the grid
				}
				day.Sessions = append(day.Sessions, s)
			}
			w.Days = append(w.Days, day)
		}
		// Anything just below the 17:00 row isn't imported; say so instead of dropping it silently.
		var stray []string
		for r := first + slotsPerDay; r < first+slotsPerDay+4 && len(stray) < 3; r++ {
			for c := 2; c <= 6 && len(stray) < 3; c++ {
				if v := rd.value(cellName(c, r)); v != "" && !strings.HasPrefix(strings.ToLower(v), "holiday") { // holidays are repeated down the column; already handled
					if len(v) > 40 {
						v = v[:40] + "…"
					}
					stray = append(stray, fmt.Sprintf("%s: %q", cellName(c, r), strings.ReplaceAll(v, "\n", " ")))
				}
			}
		}
		if len(stray) > 0 {
			out.Warnings = append(out.Warnings, Warning{"info", n, "", "Text below the 17:00 row was not imported (" + strings.Join(stray, "; ") + ")."})
		}
		out.Weeks = append(out.Weeks, w)
	}
	out.Warnings = append(out.Warnings, colours.warnings()...)
	if len(out.Weeks) == 0 {
		return Parsed{}, fmt.Errorf("no weeks found in sheet %q (expected blocks headed WEEK 1 / DAY 1…)", sheet)
	}
	return out, nil
}

func cellName(col, row int) string {
	n, _ := excelize.CoordinatesToCellName(col, row)
	return n
}

// colourStats counts fills that aren't in the key, so each colour is reported once for the whole sheet.
type colourStats map[string]*struct {
	count int
	weeks []int
	near  string // type slug of the closest key colour, when it is a near miss
}

func (c colourStats) add(colour, near string, week int) {
	st := c[colour]
	if st == nil {
		st = &struct {
			count int
			weeks []int
			near  string
		}{near: near}
		c[colour] = st
	}
	st.count++
	if n := len(st.weeks); n == 0 || st.weeks[n-1] != week {
		st.weeks = append(st.weeks, week)
	}
}

func (c colourStats) warnings() []Warning {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out []Warning
	for _, k := range keys {
		st := c[k]
		ws := make([]string, len(st.weeks))
		for i, w := range st.weeks {
			ws[i] = strconv.Itoa(w)
		}
		where := fmt.Sprintf("%d session%s, week%s %s", st.count, map[bool]string{true: "", false: "s"}[st.count == 1], map[bool]string{true: "", false: "s"}[len(ws) == 1], strings.Join(ws, ", "))
		if st.near != "" {
			out = append(out, Warning{"warn", st.weeks[0], "", fmt.Sprintf("Colour %s isn't in the session types key; treated as %s, the closest match (%s). Probably a slip in the sheet.", k, typeNames[st.near], where)})
		} else {
			out = append(out, Warning{"info", st.weeks[0], "", fmt.Sprintf("Colour %s isn't in the session types key; shown as Other (%s).", k, where)})
		}
	}
	return out
}

// typeOf maps a fill colour to a session type: exact key match, else the nearest key colour
// (recorded as a near miss), else the code's prefix, else "other" (recorded as unknown).
func typeOf(key map[string]string, fill, code string, record func(near string)) string {
	if fill == "" {
		if t := codeType(code); t != "" {
			return t
		}
		return "other"
	}
	if t, ok := key[fill]; ok {
		return t
	}
	best, bestD := "", 1e9
	if c, ok := parseRGB(fill); ok {
		for k, t := range key {
			if kc, ok := parseRGB(k); ok {
				if d := dist(c, kc); d < bestD {
					best, bestD = t, d
				}
			}
		}
	}
	if best != "" && bestD <= 24 {
		record(best)
		return best
	}
	record("")
	if t := codeType(code); t != "" {
		return t
	}
	return "other"
}
