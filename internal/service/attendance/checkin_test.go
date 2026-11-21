package attendance

import (
	"testing"
	"time"

	"gofiber-baro/internal/domain"
)

func TestDecideCheckIn(t *testing.T) {
	base := func(now time.Time) CheckInInput {
		return CheckInInput{Now: now, Rules: DefaultRules(), IsClassDay: true, IPAllowed: true}
	}
	type want struct {
		s   domain.AttendanceSession
		st  domain.AttendanceStatus
		err error
	}
	cases := []struct {
		name string
		in   CheckInInput
		want want
	}{
		{"before open", base(at(8, 0, 0)), want{err: ErrNotOpenYet}},
		{"morning on time", base(at(9, 10, 0)), want{domain.SessionMorning, domain.StatusPresent, nil}},
		{"morning late", base(at(9, 40, 0)), want{domain.SessionMorning, domain.StatusLate, nil}},
		{"morning closed", base(at(10, 31, 0)), want{err: ErrCheckInClosed}},
		{"afternoon on time", base(at(13, 5, 0)), want{domain.SessionAfternoon, domain.StatusPresent, nil}},
		{"afternoon late", base(at(14, 0, 0)), want{domain.SessionAfternoon, domain.StatusLate, nil}},
		{"no class", func() CheckInInput { i := base(at(9, 10, 0)); i.IsClassDay = false; return i }(), want{err: ErrNoClassToday}},
		{"network", func() CheckInInput { i := base(at(9, 10, 0)); i.IPAllowed = false; return i }(), want{err: ErrNetworkNotAllowed}},
		{"already", func() CheckInInput { i := base(at(9, 10, 0)); i.AlreadyCheckedIn = true; return i }(), want{err: ErrAlreadySubmitted}},
		{"absent leave", func() CheckInInput { i := base(at(9, 10, 0)); i.Leave = domain.StatusAbsentExcused; return i }(), want{err: ErrCoveredByLeave}},
		{"late leave but on time", func() CheckInInput { i := base(at(9, 10, 0)); i.Leave = domain.StatusLateExcused; return i }(), want{domain.SessionMorning, domain.StatusPresent, nil}},
		{"late leave and late", func() CheckInInput { i := base(at(9, 40, 0)); i.Leave = domain.StatusLateExcused; return i }(), want{domain.SessionMorning, domain.StatusLateExcused, nil}},
	}
	for _, c := range cases {
		got, err := DecideCheckIn(c.in)
		if err != c.want.err {
			t.Errorf("%s: err %v want %v", c.name, err, c.want.err)
			continue
		}
		if err == nil && (got.Session != c.want.s || got.Status != c.want.st) {
			t.Errorf("%s: got %s/%s want %s/%s", c.name, got.Session, got.Status, c.want.s, c.want.st)
		}
	}
}

func TestLeaveStatusFor(t *testing.T) {
	am, pm := domain.SessionMorning, domain.SessionAfternoon
	ok := domain.LeaveStatusApproved
	leaves := []domain.LeaveRequest{
		{Date: "2026-09-08", Type: domain.LeaveTypeFullDay, Status: ok},
		{Date: "2026-09-09", Type: domain.LeaveTypeHalfDay, Session: &pm, Status: ok},
		{Date: "2026-09-10", Type: domain.LeaveTypeLate, Session: &am, Status: ok},
		{Date: "2026-09-11", Type: domain.LeaveTypeLate, Status: ok},
		{Date: "2026-09-14", Type: domain.LeaveTypeFullDay, Status: domain.LeaveStatusPending},
	}
	cases := []struct {
		date string
		s    domain.AttendanceSession
		want domain.AttendanceStatus
	}{
		{"2026-09-08", am, domain.StatusAbsentExcused},
		{"2026-09-08", pm, domain.StatusAbsentExcused},
		{"2026-09-09", am, ""},
		{"2026-09-09", pm, domain.StatusAbsentExcused},
		{"2026-09-10", am, domain.StatusLateExcused},
		{"2026-09-10", pm, ""},
		{"2026-09-11", am, domain.StatusLateExcused},
		{"2026-09-14", am, ""},
	}
	for _, c := range cases {
		if got := LeaveStatusFor(leaves, c.date, c.s); got != c.want {
			t.Errorf("%s %s: got %q want %q", c.date, c.s, got, c.want)
		}
	}
}
