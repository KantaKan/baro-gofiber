package cup

import (
	"testing"
	"time"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

var bkk, _ = time.LoadLocation("Asia/Bangkok")

func at(y int, m time.Month, d, h, min int) time.Time { return time.Date(y, m, d, h, min, 0, 0, bkk) }

func TestWeekStart(t *testing.T) {
	cases := map[string]time.Time{
		"2026-09-07":   at(2026, 9, 7, 0, 0),    // Monday
		"2026-09-07 ":  at(2026, 9, 11, 17, 0),  // Friday after close: still that week
		"2026-09-07  ": at(2026, 9, 13, 23, 59), // Sunday
	}
	for want, in := range cases {
		if got := WeekStart(in); got != want[:10] {
			t.Errorf("WeekStart(%v) = %s, want %s", in, got, want[:10])
		}
	}
	// UTC input late Sunday is already Monday in Bangkok
	if got := WeekStart(time.Date(2026, 9, 13, 20, 0, 0, 0, time.UTC)); got != "2026-09-14" {
		t.Errorf("UTC Sunday 20:00 = %s, want 2026-09-14", got)
	}
}

func TestWeekEnd(t *testing.T) {
	if got := WeekEnd("2026-09-07"); !got.Equal(at(2026, 9, 11, 16, 45)) {
		t.Errorf("WeekEnd = %v", got)
	}
}

func TestWeekNo(t *testing.T) {
	if got := WeekNo("2026-07-06", "2026-09-07"); got != 10 {
		t.Errorf("WeekNo = %d, want 10", got)
	}
	if got := WeekNo("2026-07-08", "2026-07-06"); got != 1 { // start mid-week
		t.Errorf("WeekNo mid-week start = %d, want 1", got)
	}
	if got := WeekNo("", "2026-09-07"); got != 37 { // ISO week fallback
		t.Errorf("WeekNo fallback = %d, want 37", got)
	}
}

func aw(group, cat string, pts int, day string) domain.CupAward {
	return domain.CupAward{Group: group, Category: cat, Points: pts, Day: day}
}

func TestStandings(t *testing.T) {
	awards := []domain.CupAward{
		aw("B", CatSoft, 30, "2026-09-07"),
		aw("A", CatCoding, 10, "2026-09-08"), aw("A", CatSoft, 20, "2026-09-09"),
		aw("C", CatCoding, 20, "2026-09-11"), aw("C", CatHelping, 10, "2026-09-11"),
	}
	got := Standings(awards, []string{"A", "B", "C", "D"}, "2026-09-07")
	order := []string{}
	for _, g := range got {
		order = append(order, g.Group)
	}
	// A, B, C all 30: coding decides C(20) > A(10) > B(0); D last with 0
	if want := []string{"C", "A", "B", "D"}; len(order) != 4 || order[0] != want[0] || order[1] != want[1] || order[2] != want[2] || order[3] != want[3] {
		t.Fatalf("order = %v, want %v", order, want)
	}
	if got[0].ByDay[4] != 30 || got[1].ByDay[1] != 10 || got[1].ByDay[2] != 20 {
		t.Errorf("ByDay wrong: %+v %+v", got[0].ByDay, got[1].ByDay)
	}
	if got[0].ByCategory[CatHelping] != 10 || got[3].Total != 0 {
		t.Errorf("ByCategory/zero wrong: %+v", got)
	}
	if w, ok := Winner(got); !ok || w != "C" {
		t.Errorf("Winner = %s %v", w, ok)
	}
	if _, ok := Winner(Standings(nil, []string{"A", "B"}, "2026-09-07")); ok {
		t.Error("all-zero week must have no winner")
	}
}

func TestWholeGroupOnTime(t *testing.T) {
	a, b := primitive.NewObjectID(), primitive.NewObjectID()
	ok := []domain.AttendanceStatus{domain.StatusPresent, domain.StatusPresent}
	cases := []struct {
		name string
		st   map[primitive.ObjectID][]domain.AttendanceStatus
		want bool
	}{
		{"all present", map[primitive.ObjectID][]domain.AttendanceStatus{a: ok, b: ok}, true},
		{"excused is fine", map[primitive.ObjectID][]domain.AttendanceStatus{a: ok, b: {domain.StatusAbsentExcused, domain.StatusLateExcused}}, true},
		{"late breaks", map[primitive.ObjectID][]domain.AttendanceStatus{a: ok, b: {domain.StatusPresent, domain.StatusLate}}, false},
		{"one session missing breaks", map[primitive.ObjectID][]domain.AttendanceStatus{a: ok, b: {domain.StatusPresent}}, false},
		{"no record breaks", map[primitive.ObjectID][]domain.AttendanceStatus{a: ok}, false},
	}
	for _, c := range cases {
		if got := WholeGroupOnTime([]primitive.ObjectID{a, b}, c.st); got != c.want {
			t.Errorf("%s: got %v", c.name, got)
		}
	}
	if WholeGroupOnTime(nil, nil) {
		t.Error("empty group must not earn the bonus")
	}
}
