package report

import (
	"math"
	"strings"
	"testing"
	"time"

	"gofiber-baro/internal/domain"
	"gofiber-baro/internal/service/cup"
	"gofiber-baro/internal/service/wellbeing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	cmf = "Comfort Zone"
	pan = "Panic Zone"
)

func learner(name, group string, refls ...domain.Reflection) domain.User {
	return domain.User{ID: primitive.NewObjectID(), FirstName: name, ZoomName: "01_" + name, GenmateGroup: group, Role: "learner", Reflections: refls}
}

func refl(day, label string) domain.Reflection {
	return domain.Reflection{Day: day, ReflectionData: domain.ReflectionContent{Barometer: label}}
}

func rec(u domain.User, date string, session domain.AttendanceSession, st domain.AttendanceStatus) domain.AttendanceRecord {
	return domain.AttendanceRecord{UserID: u.ID, Date: date, Session: session, Status: st}
}

const (
	week  = "2026-11-16" // Monday
	today = "2026-11-20" // Friday
	prev  = "2026-11-09"
)

func fixture() (Input, domain.User, domain.User, domain.User) {
	a := learner("A", "B", refl("2026-11-16", cmf))
	b := learner("B", "B", refl("2026-11-16", pan))
	c := learner("C", "C")
	recs := []domain.AttendanceRecord{
		rec(a, week, domain.SessionMorning, domain.StatusPresent), rec(a, week, domain.SessionAfternoon, domain.StatusPresent),
		rec(b, week, domain.SessionMorning, domain.StatusLate), rec(b, week, domain.SessionAfternoon, domain.StatusPresent),
		rec(c, week, domain.SessionMorning, domain.StatusAbsent), rec(c, week, domain.SessionAfternoon, domain.StatusPresent),
	}
	for _, u := range []domain.User{a, b, c} {
		recs = append(recs, rec(u, prev, domain.SessionMorning, domain.StatusPresent), rec(u, prev, domain.SessionAfternoon, domain.StatusPresent))
	}
	return Input{Week: week, Today: today, Users: []domain.User{a, b, c}, Records: recs, Holidays: map[string]bool{"2026-11-17": true}}, a, b, c
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestAttendanceKPIsAndTrend(t *testing.T) {
	in, _, _, _ := fixture()
	r := Build(in)
	if !near(r.KPIs.Attendance.Value, 5.0/6) || !near(r.KPIs.Attendance.Prev, 1) {
		t.Fatalf("attendance %+v", r.KPIs.Attendance)
	}
	if r.KPIs.Lates.Value != 1 || r.KPIs.Lates.Prev != 0 {
		t.Fatalf("lates %+v", r.KPIs.Lates)
	}
	tr := r.KPIs.Attendance.Trend
	if len(tr) != 8 || !near(tr[7], 5.0/6) || !near(tr[6], 1) || tr[0] != 0 {
		t.Fatalf("trend must be 8 weeks oldest→newest: %v", tr)
	}
	if r.Week != week || r.WeekEnd != "2026-11-20" {
		t.Errorf("week %s–%s", r.Week, r.WeekEnd)
	}
}

func TestByDayCounts(t *testing.T) {
	in, _, _, _ := fixture()
	r := Build(in)
	if len(r.ByDay) != 5 {
		t.Fatalf("want 5 days, got %d", len(r.ByDay))
	}
	mon := r.ByDay[0]
	if mon.Day != week || mon.AM.OnTime != 1 || mon.AM.Late != 1 || mon.AM.Absent != 1 || mon.PM.OnTime != 3 {
		t.Fatalf("monday %+v", mon)
	}
	if !r.ByDay[1].Holiday {
		t.Error("17th is a holiday")
	}
	if r.ByDay[2].AM.NotYet != 3 {
		t.Errorf("a day with no records shows not yet: %+v", r.ByDay[2])
	}
}

func TestMoodAndReflectionKPIs(t *testing.T) {
	in, _, _, _ := fixture()
	r := Build(in)
	m := r.Mood[0]
	if m.Day != week || m.Comfort != 1 || m.Panic != 1 || m.None != 1 {
		t.Fatalf("mood %+v", m)
	}
	if !near(r.KPIs.Rough.Value, 0.5) {
		t.Errorf("rough %v", r.KPIs.Rough.Value)
	}
	// 3 learners × 4 non-holiday weekdays up to Friday = 12 possible, 2 written.
	if !near(r.KPIs.Reflections.Value, 2.0/12) {
		t.Errorf("reflections sent %v", r.KPIs.Reflections.Value)
	}
}

func TestPartialWeekDoesNotCountFutureDays(t *testing.T) {
	in, _, _, _ := fixture()
	in.Today = "2026-11-18" // Wednesday: Mon + Wed count (Tue is a holiday) = 2 days × 3
	if r := Build(in); !near(r.KPIs.Reflections.Value, 2.0/6) {
		t.Errorf("reflections sent mid-week %v", r.KPIs.Reflections.Value)
	}
}

func TestEmptyWeekHasNoNaN(t *testing.T) {
	r := Build(Input{Week: week, Today: today})
	for name, k := range map[string]KPI{"att": r.KPIs.Attendance, "late": r.KPIs.Lates, "refl": r.KPIs.Reflections, "rough": r.KPIs.Rough} {
		if math.IsNaN(k.Value) || math.IsNaN(k.Prev) || len(k.Trend) != 8 {
			t.Errorf("%s: %+v", name, k)
		}
		for _, v := range k.Trend {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				t.Errorf("%s trend has %v", name, v)
			}
		}
	}
	if r.Support == nil || r.Groups == nil || len(r.Mood) != 5 || r.Summary == "" {
		t.Errorf("empty report must still have empty lists and a summary: %+v", r)
	}
}

func TestSupportNewStillBetter(t *testing.T) {
	in, a, b, c := fixture()
	row := func(u domain.User, why string) wellbeing.SupportRow {
		return wellbeing.SupportRow{Learner: wellbeing.Ref(u), Reasons: []wellbeing.Reason{{Key: "x", Text: why}}}
	}
	in.Support = []wellbeing.SupportRow{row(a, "Leave 6/8"), row(c, "Late 3 days")}
	in.PrevSupport = map[string]bool{b.ID.Hex(): true, c.ID.Hex(): true}
	got := map[string]string{}
	for _, s := range Build(in).Support {
		got[s.Learner.FirstName] = s.Status
	}
	if got["A"] != "new" || got["C"] != "still" || got["B"] != "better" || len(got) != 3 {
		t.Fatalf("support statuses %v", got)
	}
	in.PrevSupport = nil
	for _, s := range Build(in).Support {
		if s.Status != "new" {
			t.Errorf("with no previous report everyone is new, %s is %s", s.Learner.FirstName, s.Status)
		}
	}
}

func TestGroupsTable(t *testing.T) {
	in, _, _, _ := fixture()
	in.Cup = []cup.GroupScore{{Group: "B", Total: 12}, {Group: "C", Total: 3}}
	r := Build(in)
	if len(r.Groups) != 2 {
		t.Fatalf("groups %+v", r.Groups)
	}
	b, c := r.Groups[0], r.Groups[1]
	if b.Group != "B" || !near(b.Attendance, 1) || !near(b.Rough, 0.5) || b.Cup != 12 {
		t.Errorf("group B %+v", b)
	}
	if c.Group != "C" || !near(c.Attendance, 0.5) || c.Rough != 0 || c.Cup != 3 {
		t.Errorf("group C %+v", c)
	}
}

func TestSummary(t *testing.T) {
	in, a, _, c := fixture()
	in.Support = []wellbeing.SupportRow{{Learner: wellbeing.Ref(a)}, {Learner: wellbeing.Ref(c)}}
	in.PrevSupport = map[string]bool{c.ID.Hex(): true}
	s := Build(in).Summary
	for _, want := range []string{"83%", "down 17", "1 late", "2 learners need support", "1 new", "50%"} {
		if !strings.Contains(s, want) {
			t.Errorf("summary %q missing %q", s, want)
		}
	}
	if strings.Contains(s, "A") && strings.Contains(s, "01_A") {
		t.Error("summary must not name learners")
	}
}

func TestBetterIgnoresDropouts(t *testing.T) {
	in, a, b, _ := fixture()
	for i := range in.Users {
		if in.Users[i].ID == b.ID {
			in.Users[i].AttendanceStatus = "dropout"
		}
	}
	in.Support = []wellbeing.SupportRow{{Learner: wellbeing.Ref(a)}}
	in.PrevSupport = map[string]bool{b.ID.Hex(): true}
	for _, s := range Build(in).Support {
		if s.Status == "better" {
			t.Errorf("a learner who dropped out didn't get better: %+v", s)
		}
	}
}

func TestFutureDaysAreNotMissing(t *testing.T) {
	in, _, _, _ := fixture()
	in.Today = "2026-11-18" // Wednesday
	r := Build(in)
	if r.ByDay[3].AM.NotYet != 0 || r.ByDay[4].PM.NotYet != 0 {
		t.Errorf("Thursday and Friday haven't happened yet: %+v %+v", r.ByDay[3], r.ByDay[4])
	}
	if r.ByDay[2].AM.NotYet != 3 {
		t.Errorf("Wednesday (today) with no records still shows no record: %+v", r.ByDay[2])
	}
}

func TestSavedSurvivesMongo(t *testing.T) {
	in, a, _, _ := fixture()
	in.Support = []wellbeing.SupportRow{{Learner: wellbeing.Ref(a), Reasons: []wellbeing.Reason{{Key: "x", Text: "Leave 6/8"}}}}
	r := Build(in)
	now := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)
	raw, err := bson.Marshal(Saved{Cohort: 13, Week: week, Notes: "n", Status: StatusSent, SentAt: &now, PostError: "x", Snapshot: &r})
	if err != nil {
		t.Fatal(err)
	}
	var back Saved
	if err := bson.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	g := back.Snapshot
	if g == nil || g.Summary != r.Summary || len(g.KPIs.Attendance.Trend) != 8 || len(g.Mood) != 5 || len(g.ByDay) != 5 || len(g.Support) != 1 || g.Support[0].Reasons[0] != "Leave 6/8" || back.PostError != "x" || back.SentAt == nil {
		t.Fatalf("a sent report must read back the same from Mongo: %+v", g)
	}
}
