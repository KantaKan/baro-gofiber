package calendar

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

const sheet = "JSD13_Ver1"

type book struct {
	t *testing.T
	f *excelize.File
}

func (b *book) set(cell, v string) {
	if err := b.f.SetCellValue(sheet, cell, v); err != nil {
		b.t.Fatal(err)
	}
}

func (b *book) fill(cell, rgb string) {
	st, err := b.f.NewStyle(&excelize.Style{Fill: excelize.Fill{Type: "pattern", Color: []string{rgb}, Pattern: 1}})
	if err != nil {
		b.t.Fatal(err)
	}
	if err := b.f.SetCellStyle(sheet, cell, cell, st); err != nil {
		b.t.Fatal(err)
	}
}

// session writes text into a (merged) cell range, with an optional fill.
func (b *book) session(from, to, text, rgb string) {
	b.set(from, text)
	if to != from {
		if err := b.f.MergeCell(sheet, from, to); err != nil {
			b.t.Fatal(err)
		}
	}
	if rgb != "" {
		b.fill(from, rgb)
	}
}

// header writes a week block header (WEEK/DAY row, weekday row, Facilitator+dates row, time labels).
func (b *book) header(row int, week string, firstDay int, dates [5]string, facilitatorLabel string) {
	cols := []string{"B", "C", "D", "E", "F"}
	days := []string{"MONDAY", "TUESDAY", "WEDNESDAY", "THURSDAY", "FRIDAY"}
	b.set(fmt.Sprintf("A%d", row), week)
	b.set(fmt.Sprintf("A%d", row+1), week)
	b.set(fmt.Sprintf("A%d", row+2), facilitatorLabel)
	for i, c := range cols {
		b.set(fmt.Sprintf("%s%d", c, row), fmt.Sprintf("DAY %d", firstDay+i))
		b.set(fmt.Sprintf("%s%d", c, row+1), days[i])
		b.set(fmt.Sprintf("%s%d", c, row+2), dates[i])
	}
	b.set(fmt.Sprintf("A%d", row+3), "9.00 - 9.15am")
}

func fixture(t *testing.T) *excelize.File {
	t.Helper()
	b := &book{t: t, f: excelize.NewFile()}
	b.f.SetSheetName("Sheet1", sheet)
	// Session types key (rows 1-3) with fills, as in the real file.
	for cell, kv := range map[string][2]string{
		"C1": {"Behavioral Skills & Mindset (BSM)", "FA67FA"}, "D1": {"Technical Skills (T)", "3BCAFD"}, "E1": {"Skills Application (SA)", "92D050"},
		"C2": {"Employment Essentials (EE)", "FFFF00"}, "D2": {"Reinforcement & Reflection (RR)", "FFC000"}, "E2": {"Community (C)", "F4CCCC"},
		"C3": {"Social Support & Mentorship (SSM)", "B6B6FF"}, "D3": {"Orientation", "C9DAF8"},
	} {
		b.set(cell, kv[0])
		b.fill(cell, kv[1])
	}
	b.set("H1", "Zoom JSD13 Classroom")
	_ = b.f.SetCellHyperLink(sheet, "H1", "https://zoom.us/j/123456?pwd=abc", "External")
	b.set("L47", "internal note that must never be imported")

	// Week 0 template: skipped.
	b.header(6, "WEEK 0\nORIENTATION", 0, [5]string{"DATE", "DATE", "DATE", "DATE", "DATE"}, "Facilitator")
	for i, c := range []string{"B", "C", "D", "E", "F"} {
		b.set(fmt.Sprintf("%s6", c), "DATE")
		_ = i
	}

	// Week 1 (row 44): first time row = 47 (9:00).
	b.header(44, "WEEK 1", 1, [5]string{"29-Jun", "30-Jun", "1-Jul", "2-Jul", "3-Jul"}, "Facilitator")
	b.session("B47", "B47", "C-DO1  Daily Opening - (Kan/Neeti)", "")
	b.session("B48", "B58", "REACT-4 - REACT Hooks (useEffect)\n- (Neeti/Kan)", "3BCAFD")
	b.session("B59", "B62", "LUNCH", "")
	b.session("B63", "B72", "JavaScript Assessment\n- (Kan/Neeti)", "FEBF00") // off-key, near Reinforcement
	b.session("C47", "C47", "Holiday", "")
	b.session("D48", "D50", "REACT-5 - Advanced REACT \n(Context)\n- (Neeti/Kan)", "3BCAFD")
	b.session("D51", "D52", "React - Build Day\n- (Neeti/Kan)", "123456") // unknown colour
	b.session("E47", "E47", "FLEX", "")
	b.session("E70", "E80", "SA-CS4 Case Study 4\n- (Kan)", "92D050") // runs past 17:00
	b.session("F47", "F49", "BUFFER", "")

	// Week 2 (row 83): the sheet says Jun 8 but the cohort calendar puts it on 6 Jul.
	b.header(83, "WEEK 2", 6, [5]string{"Jun 8", "Jun 9", "Jun 10", "Jun 11", "Jun 12"}, "Facilitator")
	b.session("B86", "B86", "T-X1 Something small - (Kan)", "3BCAFD")

	// Week 3 (row 122): repeats DAY numbers (starts DAY 8 right after a week that started DAY 6).
	b.header(122, "WEEK 3", 8, [5]string{"13-Jul", "14-Jul", "15-Jul", "16-Jul", "17-Jul"}, "Facilitator")
	b.session("B125", "B125", "C-DO1  Daily Opening - (Kan)", "")

	// Week 4 (row 161): the Facilitator row is missing.
	b.header(161, "WEEK 4", 11, [5]string{"20-Jul", "21-Jul", "22-Jul", "23-Jul", "24-Jul"}, "Oops")
	b.session("B164", "B164", "T-Y1 Lost - (Kan)", "3BCAFD")
	return b.f
}

func parse(t *testing.T) Parsed {
	t.Helper()
	p, err := ParseSheet(fixture(t), sheet, "2026-06-29")
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func hasWarning(p Parsed, week int, substr string) bool {
	for _, w := range p.Warnings {
		if w.Week == week && strings.Contains(strings.ToLower(w.Message), strings.ToLower(substr)) {
			return true
		}
	}
	return false
}

func TestWeeksAndDates(t *testing.T) {
	p := parse(t)
	if len(p.Weeks) != 3 || p.Weeks[0].N != 1 || p.Weeks[1].N != 2 || p.Weeks[2].N != 3 {
		t.Fatalf("weeks 0 (template) and 4 (broken) are skipped: %+v", p.Weeks)
	}
	w := p.Weeks[0]
	if w.Name != "WEEK 1" || w.Monday != "2026-06-29" || len(w.Days) != 5 || w.Published {
		t.Fatalf("week 1: %+v", w)
	}
	if w.Days[0].Date != "2026-06-29" || w.Days[4].Date != "2026-07-03" || w.Days[0].Label != "DAY 1" || w.Days[0].Weekday != 0 {
		t.Errorf("days: %+v", w.Days)
	}
	if p.Weeks[1].Monday != "2026-07-06" || p.Weeks[1].Days[0].Date != "2026-07-06" {
		t.Errorf("dates come from the cohort start date, not the sheet: %+v", p.Weeks[1].Days[0])
	}
	if p.ZoomURL != "https://zoom.us/j/123456?pwd=abc" {
		t.Errorf("zoom: %q", p.ZoomURL)
	}
}

func TestSessionsParsing(t *testing.T) {
	mon := parse(t).Weeks[0].Days[0].Sessions
	if len(mon) != 4 {
		t.Fatalf("monday sessions: %+v", mon)
	}
	do, hooks, lunch, js := mon[0], mon[1], mon[2], mon[3]
	if do.Start != "09:00" || do.End != "09:15" || do.Code != "C-DO1" || do.Title != "Daily Opening" || len(do.Facilitators) != 2 || do.Facilitators[0] != "Kan" || do.Type != "community" {
		t.Errorf("single 15-minute cell: %+v", do)
	}
	if hooks.Start != "09:15" || hooks.End != "12:00" || hooks.Code != "REACT-4" || hooks.Title != "REACT Hooks (useEffect)" || hooks.Type != "technical" || hooks.Facilitators[0] != "Neeti" {
		t.Errorf("merged session: %+v", hooks)
	}
	if lunch.Kind != "break" || lunch.Start != "12:00" || lunch.End != "13:00" || lunch.Title != "Lunch" {
		t.Errorf("lunch: %+v", lunch)
	}
	if js.Code != "" || js.Title != "JavaScript Assessment" || js.Start != "13:00" || js.End != "15:30" || js.Type != "reinforcement" {
		t.Errorf("a title that merely starts with a capitalised word has no code, and the near-key colour wins: %+v", js)
	}
}

func TestSubtitleAndCodeRules(t *testing.T) {
	wed := parse(t).Weeks[0].Days[2].Sessions
	if len(wed) != 2 {
		t.Fatalf("wednesday: %+v", wed)
	}
	if wed[0].Code != "REACT-5" || wed[0].Title != "Advanced REACT" || wed[0].Subtitle != "(Context)" || wed[0].Start != "09:15" || wed[0].End != "10:00" {
		t.Errorf("subtitle: %+v", wed[0])
	}
	if wed[1].Code != "" || wed[1].Title != "React - Build Day" || wed[1].Type != "other" {
		t.Errorf("'React - Build Day' has no code; unknown colour is 'other': %+v", wed[1])
	}
}

func TestBreaksAndHolidays(t *testing.T) {
	w := parse(t).Weeks[0]
	if tue := w.Days[1]; !tue.Holiday || len(tue.Sessions) != 0 {
		t.Errorf("Holiday = no class: %+v", tue)
	}
	if thu := w.Days[3].Sessions[0]; thu.Kind != "break" || thu.Title != "Flex" {
		t.Errorf("flex: %+v", thu)
	}
	if fri := w.Days[4].Sessions[0]; fri.Kind != "break" || fri.Title != "Buffer" || fri.End != "09:45" {
		t.Errorf("buffer: %+v", fri)
	}
}

func TestNothingInternalIsImported(t *testing.T) {
	for _, w := range parse(t).Weeks {
		for _, d := range w.Days {
			for _, s := range d.Sessions {
				if strings.Contains(strings.ToLower(s.Title+s.Subtitle+s.Code), "internal note") {
					t.Fatalf("column L is internal notes and must never be imported: %+v", s)
				}
			}
		}
	}
}

func TestWarningsForFileMistakes(t *testing.T) {
	p := parse(t)
	for _, c := range []struct {
		week int
		want string
	}{
		{2, "puts it on 2026-07-06"}, // sheet dates disagree with the cohort calendar
		{3, "repeat"},                // DAY numbers repeat
		{4, "facilitator"},           // block skipped: no Facilitator row
		{1, "ends after 17:00"},      // SA-CS4 runs past the end of the day
		{1, "123456"},                // colour not in the key
		{1, "febf00"},                // off-key colour, matched to its nearest type
	} {
		if !hasWarning(p, c.week, c.want) {
			t.Errorf("week %d should warn about %q; got %+v", c.week, c.want, p.Warnings)
		}
	}
	if hasWarning(p, 0, "") && len(p.Warnings) == 0 {
		t.Error("unreachable")
	}
}

func TestColourWarningsAreGroupedPerColour(t *testing.T) {
	f := fixture(t)
	b := &book{t: t, f: f}
	b.session("B100", "B100", "T-Z1 Another off-key - (Kan)", "123456") // week 1 again? row 100 is inside week 2's block
	p, err := ParseSheet(f, sheet, "2026-06-29")
	if err != nil {
		t.Fatal(err)
	}
	var lines []Warning
	for _, w := range p.Warnings {
		if strings.Contains(w.Message, "123456") {
			lines = append(lines, w)
		}
	}
	if len(lines) != 1 {
		t.Fatalf("one line per colour across the whole sheet, got %d: %+v", len(lines), lines)
	}
	if lines[0].Level != "info" || !strings.Contains(lines[0].Message, "2 sessions") {
		t.Errorf("a colour that matches no type is information, with its count: %+v", lines[0])
	}
	for _, w := range p.Warnings {
		if strings.Contains(strings.ToLower(w.Message), "febf00") && w.Level != "warn" {
			t.Errorf("a near-miss colour (likely a slip) stays a warning: %+v", w)
		}
	}
}

func TestHugeMergedRangesCannotExhaustMemory(t *testing.T) {
	f := fixture(t)
	b := &book{t: t, f: f}
	// A crafted file can claim a session spans a million rows; only the week's 32 slots matter.
	b.session("B100", "B1000100", "T-BIG Enormous session - (Kan)", "3BCAFD")
	began := time.Now()
	p, err := ParseSheet(f, sheet, "2026-06-29")
	if err != nil {
		t.Fatal(err)
	}
	if time.Since(began) > 3*time.Second {
		t.Fatalf("parsing took %s; ranges must be clamped to the grid", time.Since(began))
	}
	for _, d := range p.Weeks[1].Days {
		for _, s := range d.Sessions {
			if s.End > "17:00" {
				t.Errorf("a clamped session must not run past the grid: %+v", s)
			}
		}
	}
}

func TestTooManyMergedCellsIsRefused(t *testing.T) {
	f := fixture(t)
	b := &book{t: t, f: f}
	for i := 0; i < maxMerges+5; i++ {
		_ = f.MergeCell(sheet, fmt.Sprintf("N%d", 2*i+1), fmt.Sprintf("N%d", 2*i+2))
	}
	_ = b
	if _, err := ParseSheet(f, sheet, "2026-06-29"); err == nil || !strings.Contains(err.Error(), "merged") {
		t.Errorf("an absurd number of merged cells must be refused: %v", err)
	}
}

func TestWeekZeroAndDuplicateWeeksAreFlagged(t *testing.T) {
	f := fixture(t)
	b := &book{t: t, f: f}
	b.header(200, "WEEK 0", 0, [5]string{"22-Jun", "23-Jun", "24-Jun", "25-Jun", "26-Jun"}, "Facilitator")
	b.session("B203", "B203", "T-ORI Orientation - (Kan)", "C9DAF8")
	b.header(240, "WEEK 2", 6, [5]string{"6-Jul", "7-Jul", "8-Jul", "9-Jul", "10-Jul"}, "Facilitator")
	b.session("B243", "B243", "T-DUP Duplicate - (Kan)", "3BCAFD")
	p, err := ParseSheet(f, sheet, "2026-06-29")
	if err != nil {
		t.Fatal(err)
	}
	seen := map[int]int{}
	for _, w := range p.Weeks {
		seen[w.N]++
		if w.N <= 0 {
			t.Errorf("a week numbered %d can never be published", w.N)
		}
	}
	for n, c := range seen {
		if c > 1 {
			t.Errorf("week %d appears %d times", n, c)
		}
	}
	if !hasWarning(p, 0, "week 0") || !hasWarning(p, 2, "twice") {
		t.Errorf("both problems are reported: %+v", p.Warnings)
	}
}

func TestContentOutsideTheGridIsReported(t *testing.T) {
	f := fixture(t)
	b := &book{t: t, f: f}
	b.session("B79", "B79", "Weekly Chat - (Kan)", "") // the 17:00–18:00 row of week 1
	p, err := ParseSheet(f, sheet, "2026-06-29")
	if err != nil {
		t.Fatal(err)
	}
	if !hasWarning(p, 1, "below the 17:00") {
		t.Errorf("text below the 09:00–17:00 grid must not vanish silently: %+v", p.Warnings)
	}
	b.session("C79", "C79", "Holiday", "")
	p, _ = ParseSheet(f, sheet, "2026-06-29")
	for _, w := range p.Warnings {
		if strings.Contains(w.Message, "Holiday") {
			t.Errorf("a repeated Holiday marker below the grid is not worth a note: %+v", w)
		}
	}
}

func TestParseErrors(t *testing.T) {
	f := fixture(t)
	if _, err := ParseSheet(f, "Nope", "2026-06-29"); err == nil || !strings.Contains(err.Error(), "Nope") {
		t.Errorf("unknown sheet: %v", err)
	}
	if _, err := ParseSheet(f, sheet, ""); err == nil {
		t.Error("needs the cohort start date")
	}
	if _, err := ParseSheet(f, sheet, "29/06/2026"); err == nil {
		t.Error("bad start date")
	}
	empty := excelize.NewFile()
	empty.SetSheetName("Sheet1", sheet)
	if _, err := ParseSheet(empty, sheet, "2026-06-29"); err == nil || !strings.Contains(err.Error(), "no weeks") {
		t.Errorf("a sheet with no week blocks is an error: %v", err)
	}
}

// Runs against the real program file when CALENDAR_XLSX points at it.
func TestRealFile(t *testing.T) {
	path := os.Getenv("CALENDAR_XLSX")
	if path == "" {
		t.Skip("set CALENDAR_XLSX to the JSD13 calendar to run this")
	}
	f, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	p, err := ParseSheet(f, "JSD13_Ver1", "2026-06-29")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Weeks) < 14 || p.ZoomURL == "" {
		t.Fatalf("weeks=%d zoom=%q", len(p.Weeks), p.ZoomURL)
	}
	sessions := 0
	for _, w := range p.Weeks {
		for _, d := range w.Days {
			sessions += len(d.Sessions)
		}
	}
	t.Logf("weeks=%d sessions=%d warnings=%d", len(p.Weeks), sessions, len(p.Warnings))
	for _, w := range p.Warnings {
		t.Logf("%s w%d %s: %s", w.Level, w.Week, w.Day, w.Message)
	}
	if sessions < 300 {
		t.Errorf("expected hundreds of sessions, got %d", sessions)
	}
}
