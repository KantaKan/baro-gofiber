package handler

import (
	"bytes"
	"errors"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"strings"
	"testing"

	"gofiber-baro/internal/service/calendar"
	"gofiber-baro/internal/service/staff"
	"gofiber-baro/pkg/middleware"

	"github.com/gofiber/fiber/v2"
)

type stubCal struct {
	err       error
	gotSave   bool
	gotSheet  string
	gotStart  string
	gotCohort int
	gotVer    *int
	gotN      int
	gotPub    bool
	gotZoom   string
	needs     []string
	learner   calendar.LearnerCalendar
}

func (s *stubCal) ImportAt(cohort int, start string, data []byte, sheet string, save bool, by string, v *int) (calendar.Preview, error) {
	s.gotCohort, s.gotStart, s.gotSheet, s.gotSave, s.gotVer = cohort, start, sheet, save, v
	return calendar.Preview{Weeks: 3, Warnings: []calendar.Warning{}, Diff: []calendar.WeekDiff{}, NeedsSheet: s.needs, Saved: save}, s.err
}
func (s *stubCal) Get(cohort int) (*calendar.Calendar, error) {
	s.gotCohort = cohort
	return &calendar.Calendar{Cohort: cohort, Weeks: []calendar.Week{}}, s.err
}
func (s *stubCal) Publish(cohort, n int, p bool) error {
	s.gotCohort, s.gotN, s.gotPub = cohort, n, p
	return s.err
}
func (s *stubCal) SetZoom(cohort int, url string) error { s.gotZoom = url; return s.err }
func (s *stubCal) ForLearner(cohort int) (calendar.LearnerCalendar, error) {
	s.gotCohort = cohort
	return s.learner, s.err
}

func calApp(s *stubCal) *fiber.App {
	app := fiber.New()
	h := NewCalendarHandler(s, func(cohort int) (string, error) { return "2026-06-29", nil })
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", &middleware.Claims{UserID: "u1", Role: "admin", Cohort: 13})
		c.Locals("staff", staff.Ctx{UserID: "u1", Name: "Kan", Role: staff.Owner})
		return c.Next()
	})
	app.Get("/admin/calendar", h.Get)
	app.Post("/admin/calendar/import", h.Import)
	app.Put("/admin/calendar/weeks/:n/publish", h.Publish)
	app.Put("/admin/calendar/settings", h.Settings)
	app.Get("/calendar/me", h.Mine)
	return app
}

func upload(app *fiber.App, url, filename string, content []byte) (int, string) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if filename != "" {
		part, _ := w.CreateFormFile("file", filename)
		_, _ = part.Write(content)
	}
	_ = w.Close()
	req := httptest.NewRequest("POST", url, &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	res, _ := app.Test(req)
	b, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(b)
}

func TestCalendarImportHandler(t *testing.T) {
	s := &stubCal{}
	app := calApp(s)
	if code, _ := upload(app, "/admin/calendar/import?cohort=13", "", nil); code != 400 {
		t.Errorf("no file: %d", code)
	}
	if code, _ := upload(app, "/admin/calendar/import?cohort=13", "calendar.pdf", []byte("x")); code != 400 {
		t.Errorf("not an xlsx: %d", code)
	}
	if code, _ := upload(app, "/admin/calendar/import", "c.xlsx", []byte("x")); code != 400 {
		t.Errorf("no cohort: %d", code)
	}
	code, body := upload(app, "/admin/calendar/import?cohort=13&sheet=JSD13_Ver1", "JSD13 Calendar.XLSX", []byte("PK"))
	if code != 200 || s.gotSave || s.gotSheet != "JSD13_Ver1" || s.gotCohort != 13 || s.gotStart != "2026-06-29" || !strings.Contains(body, `"weeks":3`) {
		t.Errorf("preview: %d %s save=%v sheet=%q start=%q", code, body, s.gotSave, s.gotSheet, s.gotStart)
	}
	if code, _ := upload(app, "/admin/calendar/import?cohort=13&save=1&version=4", "c.xlsx", []byte("PK")); code != 200 || !s.gotSave || s.gotVer == nil || *s.gotVer != 4 {
		t.Errorf("save: %d save=%v ver=%v", code, s.gotSave, s.gotVer)
	}
	if code, _ := upload(app, "/admin/calendar/import?cohort=13&save=1&version=abc", "c.xlsx", []byte("PK")); code != 400 {
		t.Errorf("bad version: %d", code)
	}
	s.needs = []string{"JSD13_Ver1", "JSD13_Ver2"}
	if code, body := upload(app, "/admin/calendar/import?cohort=13", "c.xlsx", []byte("PK")); code != 200 || !strings.Contains(body, "JSD13_Ver2") {
		t.Errorf("needs a sheet: %d %s", code, body)
	}
}

func TestCalendarImportErrors(t *testing.T) {
	cases := []struct {
		err  error
		want int
	}{
		{calendar.ErrBadFile, 400},
		{&calendar.UserError{Msg: "sheet \"X\" not found in this file"}, 400},
		{calendar.ErrConflict, 409},
		{errors.New("mongo: connection refused 10.0.0.5"), 500},
	}
	for _, c := range cases {
		s := &stubCal{err: c.err}
		code, body := upload(calApp(s), "/admin/calendar/import?cohort=13&save=1", "c.xlsx", []byte("PK"))
		if code != c.want {
			t.Errorf("%v → %d, want %d", c.err, code, c.want)
		}
		if strings.Contains(body, "10.0.0.5") {
			t.Errorf("internal error text must not reach the browser: %s", body)
		}
	}
}

func TestCalendarPublishSettingsAndLearner(t *testing.T) {
	s := &stubCal{learner: calendar.LearnerCalendar{Weeks: []calendar.LearnerWeek{{N: 2}}}}
	app := calApp(s)
	if code, _ := slackCall(app, "PUT", "/admin/calendar/weeks/2/publish?cohort=13", `{"published":true}`); code != 200 || s.gotN != 2 || !s.gotPub || s.gotCohort != 13 {
		t.Errorf("publish: %d %+v", code, s)
	}
	if code, _ := slackCall(app, "PUT", "/admin/calendar/weeks/abc/publish?cohort=13", `{"published":true}`); code != 400 {
		t.Errorf("bad week: %d", code)
	}
	s.err = calendar.ErrNoWeek
	if code, _ := slackCall(app, "PUT", "/admin/calendar/weeks/9/publish?cohort=13", `{"published":true}`); code != 404 {
		t.Errorf("unknown week: %d", code)
	}
	s.err = nil
	if code, _ := slackCall(app, "PUT", "/admin/calendar/settings?cohort=13", `{"zoom_url":"https://zoom.us/j/1"}`); code != 200 || s.gotZoom != "https://zoom.us/j/1" {
		t.Errorf("zoom: %d %q", code, s.gotZoom)
	}
	s.err = calendar.ErrBadURL
	if code, _ := slackCall(app, "PUT", "/admin/calendar/settings?cohort=13", `{"zoom_url":"http://x"}`); code != 400 {
		t.Errorf("bad zoom: %d", code)
	}
	s.err = nil
	code, body := slackCall(app, "GET", "/calendar/me", "")
	if code != 200 || s.gotCohort != 13 || !strings.Contains(body, `"n":2`) {
		t.Errorf("learner: %d %s cohort=%d", code, body, s.gotCohort)
	}
}
