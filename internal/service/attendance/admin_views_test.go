package attendance

import (
	"encoding/json"
	"strings"
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
	v := BuildTodayRoster([]domain.User{a, b, c, d}, recs, StatsByUser(recs), bkkAt(today, 9, 10), domain.DefaultRules(), true, false)
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
	v := BuildTodayRoster([]domain.User{a}, nil, StatsByUser(nil), bkkAt("2026-09-12", 10, 0), domain.DefaultRules(), false, false)
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

func TestBuildRegister(t *testing.T) {
	a := learner("A", "F", "01")
	rec := func(d string, s domain.AttendanceSession, st domain.AttendanceStatus) domain.AttendanceRecord {
		return domain.AttendanceRecord{ID: primitive.NewObjectID(), UserID: a.ID, Date: d, Session: s, Status: st, MarkedBy: domain.MarkedBySelf}
	}
	M, A := domain.SessionMorning, domain.SessionAfternoon
	recs := []domain.AttendanceRecord{
		rec("2026-08-17", M, domain.StatusPresent), rec("2026-08-17", A, domain.StatusPresent),
		rec("2026-08-18", M, domain.StatusLate), rec("2026-08-18", A, domain.StatusPresent),
		rec("2026-08-20", M, domain.StatusAbsent), rec("2026-08-20", A, domain.StatusAbsentExcused),
	}
	v := BuildRegister([]domain.User{a}, recs, StatsByUser(recs), "2026-08-17", map[string]bool{"2026-08-19": true})
	if v.Week != "2026-08-17" || v.Dates[4] != "2026-08-21" || len(v.Holidays) != 1 || v.Holidays[0] != "2026-08-19" {
		t.Fatalf("week header = %+v", v)
	}
	l := v.Learners[0]
	if l.Days[0][0].Status != domain.StatusPresent || l.Days[0][1].Status != domain.StatusPresent || l.Days[2][0].Status != "" {
		t.Errorf("days = %+v", l.Days)
	}
	if l.WeekRate < 0.666 || l.WeekRate > 0.667 { // attended 4 of 6 recorded sessions (Wed holiday, Fri not yet)
		t.Errorf("week rate = %v, want 4/6", l.WeekRate)
	}
	if v.KPIs.Lates != 1 || v.KPIs.Absences != 1 || v.KPIs.Excused != 1 || l.LeaveUsed != 2 {
		t.Errorf("kpis = %+v leave %d", v.KPIs, l.LeaveUsed)
	}
}

func TestBuildRegisterPreviousWeek(t *testing.T) {
	a := learner("A", "F", "01")
	recs := []domain.AttendanceRecord{{ID: primitive.NewObjectID(), UserID: a.ID, Date: "2026-08-12", Session: domain.SessionMorning, Status: domain.StatusLate}}
	v := BuildRegister([]domain.User{a}, recs, StatsByUser(recs), "2026-08-17", nil)
	if v.KPIs.Lates != 0 || v.Previous.Lates != 1 {
		t.Errorf("kpis %+v previous %+v", v.KPIs, v.Previous)
	}
}

func TestCountsSkipNonClassStatuses(t *testing.T) {
	a, b := learner("A", "F", "01"), learner("B", "F", "02")
	today := "2026-09-08"
	recs := []domain.AttendanceRecord{
		{ID: primitive.NewObjectID(), UserID: a.ID, Date: today, Session: domain.SessionMorning, Status: domain.StatusHoliday},
		{ID: primitive.NewObjectID(), UserID: b.ID, Date: today, Session: domain.SessionMorning, Status: domain.StatusPresent, MarkedBy: domain.MarkedBySelf},
	}
	v := BuildTodayRoster([]domain.User{a, b}, recs, StatsByUser(recs), bkkAt(today, 9, 0), domain.DefaultRules(), true, false)
	if got := v.Counts[domain.SessionMorning]; got.Total != 1 || got.OnTime != 1 {
		t.Errorf("holiday record must not count as checked in: %+v", got)
	}
}

func TestSharedIPFlag(t *testing.T) {
	a, b, c := learner("A", "F", "01"), learner("B", "F", "02"), learner("C", "F", "03")
	today := "2026-09-08"
	self := func(u domain.User, ip string) domain.AttendanceRecord {
		return domain.AttendanceRecord{ID: primitive.NewObjectID(), UserID: u.ID, Date: today, Session: domain.SessionMorning, Status: domain.StatusPresent, MarkedBy: domain.MarkedBySelf, IPAddress: ip, SubmittedAt: bkkAt(today, 9, 0)}
	}
	recs := []domain.AttendanceRecord{self(a, "1.1.1.1"), self(b, "1.1.1.1"), self(c, "2.2.2.2")}
	v := BuildTodayRoster([]domain.User{a, b, c}, recs, StatsByUser(recs), bkkAt(today, 9, 5), domain.DefaultRules(), true, true)
	if v.Learners[0].Morning.Flag != "shared_ip" || v.Learners[1].Morning.Flag != "shared_ip" || v.Learners[2].Morning.Flag != "" {
		t.Errorf("flags = %q %q %q", v.Learners[0].Morning.Flag, v.Learners[1].Morning.Flag, v.Learners[2].Morning.Flag)
	}
	off := BuildTodayRoster([]domain.User{a, b, c}, recs, StatsByUser(recs), bkkAt(today, 9, 5), domain.DefaultRules(), true, false)
	if off.Learners[0].Morning.Flag != "" {
		t.Error("flagging off → no flags")
	}
}

func TestBuildTodayRosterLearnerWithoutStats(t *testing.T) {
	a := learner("A", "F", "01")
	v := BuildTodayRoster([]domain.User{a}, nil, map[primitive.ObjectID]domain.LearnerStats{}, bkkAt("2026-09-08", 9, 0), domain.DefaultRules(), true, false)
	l := v.Learners[0]
	if l.Streak != 0 || l.LeaveUsed != 0 || l.AttendanceRate != 0 || l.Last10 == nil {
		t.Fatalf("learner with no stats doc → %+v, want zeros and empty last10", l)
	}
	b, _ := json.Marshal(l)
	if !strings.Contains(string(b), `"last10":[]`) {
		t.Errorf("last10 must serialize as [] not null: %s", b)
	}
}
