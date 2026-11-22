package jobs

import (
	"context"
	"fmt"
	"log"
	"time"

	"gofiber-baro/internal/service/report"
)

var bangkok = time.FixedZone("BKK", 7*3600)

// Reporter is what the Friday job needs: the auto-send setting, and the weekly report's automatic send.
type Reporter interface {
	AutoSettings() (auto bool, cohort int, err error)
	AutoSend(cohort int, week string) (report.SendResult, error)
}

// ReportDue: Friday 17:00–17:59 Bangkok (and the week's report not sent yet).
func ReportDue(now time.Time, alreadySent bool) bool {
	n := now.In(bangkok)
	return !alreadySent && n.Weekday() == time.Friday && n.Hour() == 17
}

// MondayOf is the Monday (YYYY-MM-DD, Bangkok) of the week containing now.
func MondayOf(now time.Time) string {
	n := now.In(bangkok)
	d := time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
	return d.AddDate(0, 0, -((int(d.Weekday()) + 6) % 7)).Format("2006-01-02")
}

// Tick runs the Friday send when auto-send is on and it is 17:xx Friday. AutoSend posts a given week once,
// and retries only a failed Slack post, so ticking every minute is safe. Any problem is returned so it gets
// logged (and, being a Slack failure, retried on the next tick).
func Tick(r Reporter, now time.Time) error {
	if !ReportDue(now, false) {
		return nil // cheap exit: no database work except on Friday 17:xx
	}
	auto, cohort, err := r.AutoSettings()
	if err != nil || !auto || cohort <= 0 {
		return err
	}
	res, err := r.AutoSend(cohort, MondayOf(now))
	if err != nil {
		return err
	}
	if res.SlackError != "" {
		return fmt.Errorf("the weekly report is saved but the Slack post failed: %s", res.SlackError)
	}
	return nil
}

// RunWeeklyReportJob checks every interval (a minute is fine) until ctx ends.
func RunWeeklyReportJob(ctx context.Context, r Reporter, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	log.Println("Weekly report job started")
	for {
		select {
		case <-ctx.Done():
			log.Println("Weekly report job stopped")
			return
		case <-ticker.C:
			if err := Tick(r, time.Now()); err != nil {
				log.Printf("WARNING: weekly report job: %v", err)
			}
		}
	}
}
