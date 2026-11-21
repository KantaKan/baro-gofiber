package domain

import (
	"errors"
	"time"
)

// Clock is a time of day (hour, minute) in the cohort's local timezone.
type Clock struct {
	H int `bson:"h" json:"h"`
	M int `bson:"m" json:"m"`
}

// On returns this clock time on the same date and location as day, seconds zero.
func (c Clock) On(day time.Time) time.Time {
	return time.Date(day.Year(), day.Month(), day.Day(), c.H, c.M, 0, 0, day.Location())
}

func (c Clock) minutes() int { return c.H*60 + c.M }

// SessionWindow: check-in opens at OpenAt, counts as present while now <= LateAfter,
// late while now <= AbsentAfter, and is closed after that.
type SessionWindow struct {
	OpenAt      Clock `bson:"open_at" json:"open_at"`
	LateAfter   Clock `bson:"late_after" json:"late_after"`
	AbsentAfter Clock `bson:"absent_after" json:"absent_after"`
}

type CheckinRules struct {
	Morning   SessionWindow `bson:"morning" json:"morning"`
	Afternoon SessionWindow `bson:"afternoon" json:"afternoon"`
}

func DefaultRules() CheckinRules {
	return CheckinRules{
		Morning:   SessionWindow{OpenAt: Clock{8, 30}, LateAfter: Clock{9, 15}, AbsentAfter: Clock{10, 30}},
		Afternoon: SessionWindow{OpenAt: Clock{12, 30}, LateAfter: Clock{13, 15}, AbsentAfter: Clock{14, 30}},
	}
}

var ErrInvalidRules = errors.New("check-in rules must open before late, late before absent, and morning must close before afternoon opens")

func (w SessionWindow) valid() bool {
	return w.OpenAt.minutes() < w.LateAfter.minutes() && w.LateAfter.minutes() < w.AbsentAfter.minutes()
}

func (r CheckinRules) Validate() error {
	if !r.Morning.valid() || !r.Afternoon.valid() || r.Morning.AbsentAfter.minutes() >= r.Afternoon.OpenAt.minutes() {
		return ErrInvalidRules
	}
	return nil
}

// SessionAt is afternoon once the afternoon window has opened, otherwise morning.
func (r CheckinRules) SessionAt(now time.Time) AttendanceSession {
	if !now.Before(r.Afternoon.OpenAt.On(now)) {
		return SessionAfternoon
	}
	return SessionMorning
}

func (r CheckinRules) Window(s AttendanceSession) SessionWindow {
	if s == SessionAfternoon {
		return r.Afternoon
	}
	return r.Morning
}

type Phase int

const (
	PhaseNotOpen Phase = iota
	PhaseOnTime
	PhaseLate
	PhaseClosed
)

func (w SessionWindow) PhaseAt(now time.Time) Phase {
	switch {
	case now.Before(w.OpenAt.On(now)):
		return PhaseNotOpen
	case !now.After(w.LateAfter.On(now)):
		return PhaseOnTime
	case !now.After(w.AbsentAfter.On(now)):
		return PhaseLate
	default:
		return PhaseClosed
	}
}
