package attendance

import (
	"testing"

	"gofiber-baro/internal/domain"
)

var (
	sP, sA, sAE = domain.StatusPresent, domain.StatusAbsent, domain.StatusAbsentExcused
	sL, sLE     = domain.StatusLate, domain.StatusLateExcused
	sH, sN      = domain.StatusHoliday, domain.StatusNoClass
)

// days builds consecutive DayResults; dates only matter for HasPerfectWeek.
func days(st ...domain.AttendanceStatus) []DayResult {
	out := make([]DayResult, len(st))
	for i, s := range st {
		out[i] = DayResult{Date: "", Status: s}
	}
	return out
}

func TestOnTimeStreak(t *testing.T) {
	cases := []struct {
		in   []DayResult
		want int
	}{
		{days(sP, sP, sL, sP, sAE, sH, sP), 2},
		{nil, 0},
		{days(sP, sP, sP), 3},
		{days(sP, sA), 0},
		{days(sP, sLE, sN, sP), 2},
	}
	for i, c := range cases {
		if got := OnTimeStreak(c.in); got != c.want {
			t.Errorf("case %d: got %d want %d", i, got, c.want)
		}
	}
}

func TestLeaveDaysUsed(t *testing.T) {
	if got := LeaveDaysUsed(days(sP, sA, sAE, sL, sLE, sH, sN)); got != 4 {
		t.Errorf("got %d want 4", got)
	}
}

func TestWarningLevel(t *testing.T) {
	want := map[int]string{0: "none", 3: "none", 4: "first", 5: "first", 6: "second", 7: "third", 8: "third", 12: "third"}
	for used, w := range want {
		if got := WarningLevel(used); got != w {
			t.Errorf("%d: got %s want %s", used, got, w)
		}
	}
}

func TestAttendanceRate(t *testing.T) {
	if got := AttendanceRate(days(sP, sP, sL, sA, sH, sN)); got != 0.75 {
		t.Errorf("got %v want 0.75", got)
	}
	if got := AttendanceRate(nil); got != 0 {
		t.Errorf("empty got %v", got)
	}
}

func week(st ...domain.AttendanceStatus) []DayResult {
	dates := []string{"2026-09-07", "2026-09-08", "2026-09-09", "2026-09-10", "2026-09-11"}
	out := []DayResult{}
	for i, s := range st {
		out = append(out, DayResult{Date: dates[i], Status: s})
	}
	return out
}

func TestHasPerfectWeek(t *testing.T) {
	if !HasPerfectWeek(week(sP, sP, sP, sP, sP)) {
		t.Error("all present week should count")
	}
	if HasPerfectWeek(week(sP, sP, sL, sP, sP)) {
		t.Error("a late day breaks it")
	}
	if !HasPerfectWeek(week(sP, sP, sP, sP, sH)) {
		t.Error("holiday Friday should still count")
	}
	if HasPerfectWeek(week(sP, sP, sP)) {
		t.Error("incomplete week should not count")
	}
	if HasPerfectWeek(week(sH, sH, sN, sN, sH)) {
		t.Error("a week with no class days should not count")
	}
}

func TestBuildDays(t *testing.T) {
	am, pm := domain.SessionMorning, domain.SessionAfternoon
	recs := []domain.AttendanceRecord{
		{Date: "2026-09-09", Session: am, Status: sP},
		{Date: "2026-09-08", Session: am, Status: sP},
		{Date: "2026-09-08", Session: pm, Status: sL},
	}
	got := BuildDays(recs)
	if len(got) != 2 || got[0].Date != "2026-09-08" || got[0].Status != sL || got[1].Status != sP {
		t.Errorf("got %+v", got)
	}
}
