package attendance

import (
	"testing"
	"time"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func rec(u primitive.ObjectID, date string, s domain.AttendanceSession, st domain.AttendanceStatus) domain.AttendanceRecord {
	return domain.AttendanceRecord{UserID: u, CohortNumber: 13, Date: date, Session: s, Status: st}
}

func TestComputeStatsMatchesViewFunctions(t *testing.T) {
	u := primitive.NewObjectID()
	m, a := domain.SessionMorning, domain.SessionAfternoon
	recs := []domain.AttendanceRecord{
		rec(u, "2026-09-01", m, domain.StatusPresent), rec(u, "2026-09-01", a, domain.StatusPresent),
		rec(u, "2026-09-02", m, domain.StatusAbsentExcused), rec(u, "2026-09-02", a, domain.StatusAbsentExcused),
		rec(u, "2026-09-03", m, domain.StatusHoliday), rec(u, "2026-09-03", a, domain.StatusHoliday),
		rec(u, "2026-09-04", m, domain.StatusLate), rec(u, "2026-09-04", a, domain.StatusPresent),
		rec(u, "2026-09-07", m, domain.StatusLate), rec(u, "2026-09-07", a, domain.StatusLate),
		rec(u, "2026-09-08", m, domain.StatusAbsent),
	}
	seen := time.Date(2026, 9, 7, 1, 50, 0, 0, time.UTC)
	recs[9].MarkedBy, recs[9].SubmittedAt = domain.MarkedBySelf, seen
	days := BuildDays(recs)
	got := ComputeStats(recs)
	if got.AttendanceRate != AttendanceRate(days) || got.LeaveUsed != LeaveDaysUsed(days) || got.Streak != OnTimeStreak(days) || got.LateRun != LateRun(days) {
		t.Fatalf("stats %+v disagree with view functions", got)
	}
	if len(got.Last10) != len(days) || got.Last10[len(got.Last10)-1].Date != "2026-09-08" {
		t.Errorf("last10 = %+v", got.Last10)
	}
	if got.LastSeen == nil || !got.LastSeen.Equal(seen) {
		t.Errorf("last seen = %v, want %v", got.LastSeen, seen)
	}
}

func TestComputeStatsEmpty(t *testing.T) {
	got := ComputeStats(nil)
	if got.AttendanceRate != 0 || got.LeaveUsed != 0 || got.Streak != 0 || got.LastSeen != nil || got.Last10 == nil || len(got.Last10) != 0 {
		t.Fatalf("empty history → %+v, want zeros and an empty (non-nil) last10", got)
	}
}

func TestLateRun(t *testing.T) {
	d := []DayResult{{Date: "1", Status: domain.StatusLate}, {Date: "2", Status: domain.StatusHoliday}, {Date: "3", Status: domain.StatusLate}, {Date: "4", Status: domain.StatusLate}}
	if n := LateRun(d); n != 3 {
		t.Errorf("late run = %d, want 3 (holidays are skipped)", n)
	}
	d = append(d, DayResult{Date: "5", Status: domain.StatusPresent})
	if n := LateRun(d); n != 0 {
		t.Errorf("late run after a present day = %d, want 0", n)
	}
}

func TestStatsByUser(t *testing.T) {
	a, b := primitive.NewObjectID(), primitive.NewObjectID()
	got := StatsByUser([]domain.AttendanceRecord{rec(a, "2026-09-01", domain.SessionMorning, domain.StatusLate), rec(b, "2026-09-01", domain.SessionMorning, domain.StatusPresent)})
	if len(got) != 2 || got[a].UserID != a || got[a].CohortNumber != 13 || got[a].LeaveUsed != 1 || got[b].LeaveUsed != 0 {
		t.Fatalf("by user = %+v", got)
	}
}
