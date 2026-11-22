package jobs

import (
	"errors"
	"testing"
	"time"

	"gofiber-baro/internal/service/report"
)

var bkk = time.FixedZone("BKK", 7*3600)

func at(day, h, m int) time.Time { return time.Date(2026, 11, day, h, m, 0, 0, bkk) } // 20 Nov 2026 is a Friday

func TestReportDue(t *testing.T) {
	cases := []struct {
		now  time.Time
		sent bool
		want bool
	}{
		{at(20, 17, 0), false, true},
		{at(20, 17, 59), false, true},
		{at(20, 16, 59), false, false},
		{at(20, 18, 0), false, false},
		{at(19, 17, 30), false, false}, // Thursday
		{at(21, 17, 30), false, false}, // Saturday
		{at(20, 17, 30), true, false},  // already sent this week
	}
	for _, c := range cases {
		if got := ReportDue(c.now, c.sent); got != c.want {
			t.Errorf("%s sent=%v → %v, want %v", c.now.Format("Mon 15:04"), c.sent, got, c.want)
		}
	}
	// A UTC clock reading must be converted: 10:30 UTC on Friday = 17:30 Bangkok.
	if !ReportDue(time.Date(2026, 11, 20, 10, 30, 0, 0, time.UTC), false) {
		t.Error("must use Bangkok time")
	}
}

func TestMondayOf(t *testing.T) {
	if got := MondayOf(at(20, 17, 0)); got != "2026-11-16" {
		t.Errorf("Friday 20th → %s", got)
	}
	if got := MondayOf(at(16, 0, 5)); got != "2026-11-16" {
		t.Errorf("Monday itself → %s", got)
	}
	if got := MondayOf(at(22, 23, 59)); got != "2026-11-16" {
		t.Errorf("Sunday → %s", got)
	}
}

type fakeReporter struct {
	auto    bool
	cohort  int
	sent    map[string]bool
	sends   int
	sendErr error
}

func (f *fakeReporter) AutoSettings() (bool, int, error) { return f.auto, f.cohort, nil }
func (f *fakeReporter) IsSent(c int, w string) (bool, error) {
	return f.sent[w], nil
}
func (f *fakeReporter) Send(c int, w string) (report.SendResult, error) {
	f.sends++
	if f.sendErr != nil {
		return report.SendResult{}, f.sendErr
	}
	f.sent[w] = true
	return report.SendResult{Status: "sent", SentToSlack: true}, nil
}

func TestTickSendsOncePerWeek(t *testing.T) {
	f := &fakeReporter{auto: true, cohort: 13, sent: map[string]bool{}}
	for i := 0; i < 5; i++ {
		_ = Tick(f, at(20, 17, 5+i))
	}
	if f.sends != 1 {
		t.Fatalf("the Friday job must send once, sent %d times", f.sends)
	}
	next := &fakeReporter{auto: true, cohort: 13, sent: map[string]bool{}}
	_ = Tick(next, at(19, 17, 5)) // Thursday
	_ = Tick(next, at(20, 9, 0))  // Friday morning
	if next.sends != 0 {
		t.Error("must not send outside Friday 17:xx")
	}
}

func TestTickRespectsSettings(t *testing.T) {
	off := &fakeReporter{auto: false, cohort: 13, sent: map[string]bool{}}
	_ = Tick(off, at(20, 17, 5))
	none := &fakeReporter{auto: true, cohort: 0, sent: map[string]bool{}}
	_ = Tick(none, at(20, 17, 5))
	if off.sends != 0 || none.sends != 0 {
		t.Errorf("auto off or no cohort must send nothing: %d %d", off.sends, none.sends)
	}
	failing := &fakeReporter{auto: true, cohort: 13, sent: map[string]bool{}, sendErr: errors.New("db down")}
	if err := Tick(failing, at(20, 17, 5)); err == nil {
		t.Error("a failed send must be reported so it is retried on the next tick")
	}
	if err := Tick(failing, at(20, 17, 6)); err == nil || failing.sends != 2 {
		t.Errorf("a failed week is retried (sends=%d)", failing.sends)
	}
}
