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
	calls   int
	sendErr error
	res     report.SendResult
}

func (f *fakeReporter) AutoSettings() (bool, int, error) { return f.auto, f.cohort, nil }
func (f *fakeReporter) AutoSend(c int, w string) (report.SendResult, error) {
	f.calls++
	return f.res, f.sendErr
}

func TestTickOnlyActsOnFridayEvening(t *testing.T) {
	f := &fakeReporter{auto: true, cohort: 13}
	_ = Tick(f, at(19, 17, 5)) // Thursday
	_ = Tick(f, at(20, 9, 0))  // Friday morning
	_ = Tick(f, at(20, 18, 0)) // Friday 18:00
	if f.calls != 0 {
		t.Errorf("must not run outside Friday 17:xx (%d calls)", f.calls)
	}
	_ = Tick(f, at(20, 17, 5))
	if f.calls != 1 {
		t.Errorf("runs on Friday 17:xx (%d calls)", f.calls)
	}
}

func TestTickRespectsSettings(t *testing.T) {
	off := &fakeReporter{auto: false, cohort: 13}
	_ = Tick(off, at(20, 17, 5))
	none := &fakeReporter{auto: true, cohort: 0}
	_ = Tick(none, at(20, 17, 5))
	if off.calls != 0 || none.calls != 0 {
		t.Errorf("auto off or no cohort must do nothing: %d %d", off.calls, none.calls)
	}
}

func TestTickReportsProblemsSoTheyAreLogged(t *testing.T) {
	failing := &fakeReporter{auto: true, cohort: 13, sendErr: errors.New("db down")}
	if err := Tick(failing, at(20, 17, 5)); err == nil {
		t.Error("a failed send must be returned (and logged)")
	}
	slackDown := &fakeReporter{auto: true, cohort: 13, res: report.SendResult{Status: "sent", SlackError: "slack: not_in_channel"}}
	if err := Tick(slackDown, at(20, 17, 5)); err == nil {
		t.Error("a Slack failure on the Friday post must not be silent")
	}
	ok := &fakeReporter{auto: true, cohort: 13, res: report.SendResult{Status: "sent", SentToSlack: true}}
	if err := Tick(ok, at(20, 17, 5)); err != nil {
		t.Errorf("a clean send is not an error: %v", err)
	}
	already := &fakeReporter{auto: true, cohort: 13, res: report.SendResult{Status: "sent", AlreadySent: true}}
	if err := Tick(already, at(20, 17, 5)); err != nil {
		t.Errorf("already sent is not an error: %v", err)
	}
}
