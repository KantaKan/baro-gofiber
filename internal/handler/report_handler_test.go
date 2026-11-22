package handler

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"gofiber-baro/internal/service/report"

	"github.com/gofiber/fiber/v2"
)

type stubReport struct {
	err       error
	gotCohort int
	gotWeek   string
	notes     string
}

func (s *stubReport) Get(c int, w string) (report.Report, error) {
	s.gotCohort, s.gotWeek = c, w
	return report.Report{Week: w, Status: "draft", Notes: s.notes}, s.err
}
func (s *stubReport) SaveNotes(c int, w, n string) error {
	s.gotCohort, s.gotWeek, s.notes = c, w, n
	return s.err
}
func (s *stubReport) AutoSend(c int, w string) (report.SendResult, error) { return s.Send(c, w) }
func (s *stubReport) Send(c int, w string) (report.SendResult, error) {
	s.gotCohort, s.gotWeek = c, w
	return report.SendResult{Status: "sent", SentToSlack: true}, s.err
}

func reportApp(s *stubReport) *fiber.App {
	app := fiber.New()
	h := NewReportHandler(s)
	app.Get("/admin/report", h.Get)
	app.Put("/admin/report/notes", h.SaveNotes)
	app.Post("/admin/report/send", h.Send)
	return app
}

func TestReportHandler(t *testing.T) {
	s := &stubReport{}
	app := reportApp(s)
	code, body := slackCall(app, "GET", "/admin/report?cohort=13&week=2026-11-16", "")
	var env struct{ Data report.Report }
	_ = json.Unmarshal([]byte(body), &env)
	if code != 200 || s.gotCohort != 13 || s.gotWeek != "2026-11-16" || env.Data.Week != "2026-11-16" {
		t.Errorf("get %d %s", code, body)
	}
	if code, _ := slackCall(app, "GET", "/admin/report?week=2026-11-16", ""); code != 400 {
		t.Errorf("missing cohort %d", code)
	}
	if code, _ := slackCall(app, "PUT", "/admin/report/notes?cohort=13", `{"week":"2026-11-16","notes":"Good week"}`); code != 200 || s.notes != "Good week" || s.gotCohort != 13 {
		t.Errorf("notes %d %q", code, s.notes)
	}
	if code, body := slackCall(app, "POST", "/admin/report/send?cohort=13", `{"week":"2026-11-16"}`); code != 200 || !strings.Contains(body, `"sent_to_slack":true`) {
		t.Errorf("send %d %s", code, body)
	}
}

func TestReportHandlerErrors(t *testing.T) {
	cases := []struct {
		err  error
		want int
	}{{report.ErrBadWeek, 400}, {report.ErrFutureWeek, 400}, {report.ErrNotesTooLong, 400}, {report.ErrBadCohort, 400}, {report.ErrTooOld, 400}, {report.ErrSent, 409}, {errors.New("db down"), 500}}
	for _, c := range cases {
		s := &stubReport{err: c.err}
		app := reportApp(s)
		if code, _ := slackCall(app, "PUT", "/admin/report/notes?cohort=13", `{"week":"2026-11-16","notes":"x"}`); code != c.want {
			t.Errorf("%v → %d, want %d", c.err, code, c.want)
		}
	}
	s := &stubReport{err: errors.New("mongo: connection refused to 10.0.0.5")}
	if _, body := slackCall(reportApp(s), "GET", "/admin/report?cohort=13&week=2026-11-16", ""); strings.Contains(body, "10.0.0.5") {
		t.Errorf("internal error text must not reach the browser: %s", body)
	}
}
