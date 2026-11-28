package wellbeing

import (
	"strings"
	"testing"
	"time"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Two weeks of class days: Mon 2 Nov .. Fri 13 Nov 2026.
var twoWeeks = []string{"2026-11-02", "2026-11-03", "2026-11-04", "2026-11-05", "2026-11-06", "2026-11-09", "2026-11-10", "2026-11-11", "2026-11-12", "2026-11-13"}

func jrefl(day, zone, happy, improve string) domain.Reflection {
	labels := map[string]string{"comfort": "Comfort Zone", "enjoying": "Stretch zone - enjoying the challenges", "over": "Stretch zone - overwhelmed", "panic": "Panic Zone", "old": "Some old label"}
	return domain.Reflection{ID: primitive.NewObjectID(), Day: day, ReflectionData: domain.ReflectionContent{
		Barometer: labels[zone], TechSessions: domain.SessionDetails{Happy: happy, Improve: improve},
	}}
}

func TestJourneyFullStory(t *testing.T) {
	reply := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	dark := jrefl("2026-11-11", "panic", "", "lost")
	dark.AdminFeedback, dark.AdminFeedbackName, dark.AdminFeedbackAt = "pair tomorrow", "Nan", &reply
	in := JourneyInput{Name: "Fern", ClassDays: twoWeeks, Badges: 3, Reflections: []domain.Reflection{
		jrefl("2026-11-02", "comfort", "a", ""), jrefl("2026-11-03", "comfort", "longest went-well answer here", ""),
		jrefl("2026-11-04", "enjoying", "b", ""), jrefl("2026-11-09", "over", "", "hard"), jrefl("2026-11-10", "over", "", "hard"),
		dark, jrefl("2026-11-12", "comfort", "c", ""), jrefl("2026-11-13", "comfort", "d", ""),
	}, Records: []domain.AttendanceRecord{{Date: "2026-11-02", Session: domain.SessionMorning, Status: domain.StatusPresent}}}
	j := BuildJourney(in)
	if j.ClassDays != 10 || j.Written != 8 || len(j.Grid) != 10 {
		t.Fatalf("days %d written %d grid %d", j.ClassDays, j.Written, len(j.Grid))
	}
	if j.Grid[4].Zone != nil {
		t.Error("an unwritten day has a nil zone")
	}
	if j.MainZone != "comfort" || j.ZoneCounts["comfort"] != 4 || j.ZoneCounts["panic"] != 1 {
		t.Errorf("zones %v main %q", j.ZoneCounts, j.MainZone)
	}
	if j.HardestWeek == nil || j.HardestWeek.Number != 2 || j.HardestWeek.From != "2026-11-09" {
		t.Errorf("hardest %+v", j.HardestWeek)
	}
	if z := j.HardestWeek.Zones; len(z) != 5 || z[0] == nil || *z[0] != "stretch-overwhelmed" {
		t.Errorf("hardest week zones %v", z)
	}
	if j.DarkestNight == nil || j.DarkestNight.Date != "2026-11-11" || j.DarkestNight.Reply != "pair tomorrow" || j.DarkestNight.ReplyName != "Nan" {
		t.Errorf("darkest %+v", j.DarkestNight)
	}
	if j.Comeback == nil || !j.Comeback.AfterDarkest || j.Comeback.Length != 2 {
		t.Errorf("comeback %+v", j.Comeback)
	}
	if j.Attendance == nil || j.TAReplies != 1 || j.Badges != 3 {
		t.Errorf("tiles: attendance %v replies %d badges %d", j.Attendance, j.TAReplies, j.Badges)
	}
	if j.Quote == nil || j.Quote.Text != "longest went-well answer here" {
		t.Errorf("default quote %+v", j.Quote)
	}
}

func TestJourneyOldCohort(t *testing.T) {
	r := jrefl("2026-11-04", "over", "", "can&#39;t keep up")
	r.AdminFeedback = "you can"
	in := JourneyInput{ClassDays: twoWeeks, Reflections: []domain.Reflection{
		jrefl("2026-11-02", "old", "x", ""), jrefl("2026-11-03", "comfort", "y", ""), r,
		jrefl("2026-11-05", "comfort", "z", ""), jrefl("2026-11-06", "comfort", "w", ""),
		jrefl("2026-11-07", "comfort", "saturday study", ""),
	}}
	j := BuildJourney(in)
	if j.Attendance != nil {
		t.Error("no records means no attendance")
	}
	if j.Written != 6 || len(j.Grid) != 10 {
		t.Errorf("weekend reflection counts as written but not as a grid day: written %d grid %d", j.Written, len(j.Grid))
	}
	if j.Grid[0].Zone == nil || *j.Grid[0].Zone != "" {
		t.Error("an unknown label is written with an empty zone")
	}
	if j.ZoneCounts["comfort"] != 4 {
		t.Errorf("unknown label must not be counted: %v", j.ZoneCounts)
	}
	if j.DarkestNight == nil || j.DarkestNight.Text != "can't keep up" || j.DarkestNight.ReplyName != "" {
		t.Errorf("darkest %+v", j.DarkestNight)
	}
}

func TestJourneyNoHardNights(t *testing.T) {
	var rs []domain.Reflection
	for _, d := range twoWeeks[:6] {
		rs = append(rs, jrefl(d, "comfort", "ok", ""))
	}
	j := BuildJourney(JourneyInput{ClassDays: twoWeeks, Reflections: rs})
	if j.HardestWeek != nil || j.DarkestNight != nil {
		t.Error("no hard nights means no hardest week or darkest night")
	}
	if j.Comeback == nil || j.Comeback.AfterDarkest || j.Comeback.Length != 6 {
		t.Errorf("longest run overall: %+v", j.Comeback)
	}
}

func TestJourneyFewNights(t *testing.T) {
	j := BuildJourney(JourneyInput{ClassDays: twoWeeks, Reflections: []domain.Reflection{jrefl("2026-11-02", "panic", "", "x"), jrefl("2026-11-03", "comfort", "", "")}})
	if j.Written != 2 || j.HardestWeek != nil || j.DarkestNight != nil || j.Comeback != nil {
		t.Errorf("fewer than 5 nights keeps only the basics: %+v", j)
	}
	empty := BuildJourney(JourneyInput{ClassDays: twoWeeks})
	if empty.Written != 0 || empty.Quote != nil || len(empty.Grid) != 10 {
		t.Errorf("no reflections: %+v", empty)
	}
}

func TestJourneyChosenQuote(t *testing.T) {
	chosen := jrefl("2026-11-05", "comfort", "first deploy!", "")
	blank := jrefl("2026-11-06", "comfort", "", "")
	rs := []domain.Reflection{jrefl("2026-11-02", "comfort", "a much longer went-well answer", ""), chosen, blank}
	if q := BuildJourney(JourneyInput{ClassDays: twoWeeks, Reflections: rs, QuoteID: chosen.ID.Hex()}).Quote; q == nil || q.Text != "first deploy!" {
		t.Errorf("chosen quote %+v", q)
	}
	if q := BuildJourney(JourneyInput{ClassDays: twoWeeks, Reflections: rs, QuoteID: blank.ID.Hex()}).Quote; q == nil || q.Text != "a much longer went-well answer" {
		t.Errorf("a chosen quote with no text falls back: %+v", q)
	}
}

func TestJourneyHardestWeekTellsUnwrittenFromUnknown(t *testing.T) {
	var rs []domain.Reflection
	for _, d := range []string{"2026-11-02", "2026-11-03", "2026-11-04", "2026-11-05"} {
		rs = append(rs, jrefl(d, "comfort", "ok", ""))
	}
	rs = append(rs, jrefl("2026-11-09", "over", "", "x"), jrefl("2026-11-10", "old", "", "y"))
	w := BuildJourney(JourneyInput{ClassDays: twoWeeks, Reflections: rs}).HardestWeek
	if w == nil || w.Zones[1] == nil || *w.Zones[1] != "" || w.Zones[2] != nil {
		t.Errorf("unknown zone is \"\", unwritten is nil: %+v", w)
	}
}

func TestJourneyComebackSkippedWhenNothingFollowsTheDarkestNight(t *testing.T) {
	var rs []domain.Reflection
	for _, d := range twoWeeks[:9] {
		rs = append(rs, jrefl(d, "comfort", "ok", ""))
	}
	rs[8] = jrefl(twoWeeks[8], "panic", "", "worst")
	if c := BuildJourney(JourneyInput{ClassDays: twoWeeks, Reflections: rs}).Comeback; c != nil {
		t.Errorf("only one night after the darkest: no comeback slide, got %+v", c)
	}
}

func TestJourneyPrefersAShortEnoughQuote(t *testing.T) {
	long := strings.Repeat("ยาวมาก ", 40)
	rs := []domain.Reflection{jrefl("2026-11-02", "comfort", long, ""), jrefl("2026-11-03", "comfort", "a good shareable sentence", "")}
	if q := BuildJourney(JourneyInput{ClassDays: twoWeeks, Reflections: rs}).Quote; q == nil || q.Text != "a good shareable sentence" {
		t.Errorf("default quote should fit a slide: %+v", q)
	}
}
