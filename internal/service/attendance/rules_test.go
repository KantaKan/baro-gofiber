package attendance

import (
	"testing"
	"time"

	"gofiber-baro/internal/domain"
)

var bkk, _ = time.LoadLocation("Asia/Bangkok")

func at(h, m, s int) time.Time { return time.Date(2026, 9, 8, h, m, s, 0, bkk) }

func TestPhaseAt_MorningBoundaries(t *testing.T) {
	w := DefaultRules().Morning
	cases := []struct {
		t    time.Time
		want Phase
	}{
		{at(8, 29, 59), PhaseNotOpen}, {at(8, 30, 0), PhaseOnTime},
		{at(9, 15, 0), PhaseOnTime}, {at(9, 15, 1), PhaseLate},
		{at(10, 30, 0), PhaseLate}, {at(10, 30, 1), PhaseClosed},
	}
	for _, c := range cases {
		if got := w.PhaseAt(c.t); got != c.want {
			t.Errorf("%v: got %v want %v", c.t.Format("15:04:05"), got, c.want)
		}
	}
}

func TestPhaseAt_AfternoonBoundaries(t *testing.T) {
	w := DefaultRules().Afternoon
	cases := []struct {
		t    time.Time
		want Phase
	}{
		{at(12, 29, 59), PhaseNotOpen}, {at(12, 30, 0), PhaseOnTime},
		{at(13, 15, 0), PhaseOnTime}, {at(13, 15, 1), PhaseLate},
		{at(14, 30, 0), PhaseLate}, {at(14, 30, 1), PhaseClosed},
	}
	for _, c := range cases {
		if got := w.PhaseAt(c.t); got != c.want {
			t.Errorf("%v: got %v want %v", c.t.Format("15:04:05"), got, c.want)
		}
	}
}

func TestSessionAt(t *testing.T) {
	r := DefaultRules()
	if got := r.SessionAt(at(11, 59, 0)); got != domain.SessionMorning {
		t.Errorf("11:59 got %s", got)
	}
	if got := r.SessionAt(at(12, 30, 0)); got != domain.SessionAfternoon {
		t.Errorf("12:30 got %s", got)
	}
}

func TestValidate(t *testing.T) {
	if err := DefaultRules().Validate(); err != nil {
		t.Fatalf("default rules invalid: %v", err)
	}
	r := DefaultRules()
	r.Morning.LateAfter = Clock{H: 8, M: 0}
	if r.Validate() == nil {
		t.Error("LateAfter before OpenAt should fail")
	}
	r = DefaultRules()
	r.Morning.AbsentAfter = Clock{H: 13, M: 0}
	if r.Validate() == nil {
		t.Error("morning overlapping afternoon should fail")
	}
}

func TestCombineDayStatus_SheetTable(t *testing.T) {
	P, A, AE, H, L, LE, N := domain.StatusPresent, domain.StatusAbsent, domain.StatusAbsentExcused, domain.StatusHoliday, domain.StatusLate, domain.StatusLateExcused, domain.StatusNoClass
	rows := [][3]domain.AttendanceStatus{
		{P, P, P}, {P, A, A}, {P, AE, AE}, {P, H, H}, {P, L, L}, {P, LE, LE}, {P, N, P},
		{A, P, A}, {A, A, A}, {A, AE, A}, {A, H, H}, {A, L, A}, {A, LE, A}, {A, N, A},
		{AE, P, AE}, {AE, A, AE}, {AE, AE, AE}, {AE, H, H}, {AE, L, AE}, {AE, LE, AE}, {AE, N, AE},
		{L, P, L}, {L, A, A}, {L, AE, AE}, {L, H, H}, {L, L, L}, {L, LE, L}, {L, N, L},
		{LE, P, LE}, {LE, A, A}, {LE, AE, AE}, {LE, H, H}, {LE, L, L}, {LE, LE, LE}, {LE, N, LE},
		{N, P, P}, {N, A, A}, {N, AE, AE}, {N, H, H}, {N, L, L}, {N, LE, LE}, {N, N, N},
		{H, P, H}, {H, A, H}, {H, AE, H}, {H, H, H}, {H, L, H}, {H, LE, H}, {H, N, H},
	}
	for _, r := range rows {
		if got := CombineDayStatus(r[0], r[1]); got != r[2] {
			t.Errorf("%s+%s: got %s want %s", r[0], r[1], got, r[2])
		}
	}
	if got := CombineDayStatus(domain.StatusDropout, domain.StatusPresent); got != domain.StatusDropout {
		t.Errorf("dropout should win, got %s", got)
	}
	if got := CombineDayStatus(domain.StatusPresent, domain.StatusDismissed); got != domain.StatusDismissed {
		t.Errorf("dismissed should win, got %s", got)
	}
	if got := CombineDayStatus("", domain.StatusPresent); got != domain.StatusPresent {
		t.Errorf("missing morning treated as no_class, got %s", got)
	}
}
