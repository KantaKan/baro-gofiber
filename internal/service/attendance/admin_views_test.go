package attendance

import (
	"testing"
	"time"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

var bkkLoc, _ = time.LoadLocation("Asia/Bangkok")

func bkkAt(d string, h, m int) time.Time {
	t, _ := time.ParseInLocation("2006-01-02", d, bkkLoc)
	return t.Add(time.Duration(h)*time.Hour + time.Duration(m)*time.Minute)
}

func learner(name, group, num string) domain.User {
	return domain.User{ID: primitive.NewObjectID(), FirstName: name, ZoomName: num + "_" + name, JSDNumber: num, GenmateGroup: group, CohortNumber: 13, Role: "learner"}
}

func TestBuildTodayRoster(t *testing.T) {
	a, b, c, d := learner("A", "F", "02"), learner("B", "C", "05"), learner("C", "C", "01"), learner("D", "F", "03")
	d.AttendanceStatus = "dropout"
	a.Reflections = []domain.Reflection{
		{Day: "2026-09-04", ReflectionData: domain.ReflectionContent{Barometer: "Comfort Zone"}},
		{Day: "2026-09-07", ReflectionData: domain.ReflectionContent{Barometer: "Stretch zone - Overwhelmed"}},
	}
	today := "2026-09-08"
	recs := []domain.AttendanceRecord{
		{ID: primitive.NewObjectID(), UserID: a.ID, Date: today, Session: domain.SessionMorning, Status: domain.StatusPresent, MarkedBy: domain.MarkedBySelf, SubmittedAt: bkkAt(today, 9, 2), CheckinStyle: "cat"},
		{ID: primitive.NewObjectID(), UserID: a.ID, Date: "2026-09-07", Session: domain.SessionMorning, Status: domain.StatusLate, MarkedBy: domain.MarkedBySelf},
		{ID: primitive.NewObjectID(), UserID: c.ID, Date: today, Session: domain.SessionMorning, Status: domain.StatusAbsentExcused, MarkedBy: domain.MarkedByAdmin, MarkedByUser: "system"},
	}
	v := BuildTodayRoster([]domain.User{a, b, c, d}, recs, bkkAt(today, 9, 10), domain.DefaultRules(), true)
	if len(v.Learners) != 3 {
		t.Fatalf("learners = %d, want 3 (dropout excluded)", len(v.Learners))
	}
	if got := v.Counts[domain.SessionMorning]; got != (SessionCounts{OnTime: 1, Leave: 1, NotYet: 1, Total: 3}) {
		t.Errorf("morning counts = %+v", got)
	}
	if v.Session != domain.SessionMorning || v.Date != today || !v.ClassToday {
		t.Errorf("header = %s %s %v", v.Session, v.Date, v.ClassToday)
	}
	// sorted by group then number: C(01), B(05), A(02)
	if v.Learners[0].FirstName != "C" || v.Learners[1].FirstName != "B" || v.Learners[2].FirstName != "A" {
		t.Errorf("order = %s %s %s", v.Learners[0].FirstName, v.Learners[1].FirstName, v.Learners[2].FirstName)
	}
	la := v.Learners[2]
	if la.Morning.Status != domain.StatusPresent || la.Morning.Style != "cat" || la.Morning.MarkedBy != "self" || la.Morning.CheckedInAt == nil || la.Morning.RecordID == "" {
		t.Errorf("A morning = %+v", la.Morning)
	}
	if la.LeaveUsed != 1 || la.WarningLevel != "none" || la.MoodYesterday != "stretch-overwhelmed" {
		t.Errorf("A stats = leave %d %s mood %q", la.LeaveUsed, la.WarningLevel, la.MoodYesterday)
	}
	if lb := v.Learners[1]; lb.Morning.Status != "" || lb.Morning.CheckedInAt != nil || lb.MoodYesterday != "" {
		t.Errorf("B morning = %+v", lb.Morning)
	}
	if lc := v.Learners[0]; lc.Morning.MarkedBy != "system" || lc.Morning.CheckedInAt != nil {
		t.Errorf("C morning = %+v", lc.Morning)
	}
}

func TestBuildTodayRosterWeekend(t *testing.T) {
	a := learner("A", "F", "01")
	v := BuildTodayRoster([]domain.User{a}, nil, bkkAt("2026-09-12", 10, 0), domain.DefaultRules(), false)
	if v.ClassToday || v.Counts[domain.SessionMorning].NotYet != 1 || v.Counts[domain.SessionAfternoon].Total != 1 {
		t.Errorf("weekend view = %+v", v)
	}
}

func TestZoneOf(t *testing.T) {
	for in, want := range map[string]string{
		"Stretch zone - Overwhelmed": "stretch-overwhelmed", "comfort": "comfort", "Comfort Zone": "comfort",
		"Stretch zone - Enjoying the challenges": "stretch-enjoying", "Panic Zone": "panic", "???": "",
	} {
		if got := ZoneOf(in); got != want {
			t.Errorf("ZoneOf(%q) = %q, want %q", in, got, want)
		}
	}
}
