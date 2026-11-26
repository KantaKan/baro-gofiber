package handler

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gofiber-baro/internal/domain"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type wbUsersFake struct{ list []domain.User }

func (u wbUsersFake) ListCohortLearners(int, bson.M) ([]domain.User, error) {
	return u.list, nil
}
func (u wbUsersFake) ListCohortLearnersWithReflections(int, bson.M, int, string) ([]domain.User, error) {
	return u.list, nil
}
func (u wbUsersFake) GetUserByID(id string) (*domain.User, error) {
	for _, x := range u.list {
		if x.ID.Hex() == id {
			return &x, nil
		}
	}
	return nil, context.Canceled
}

type careReadFake struct{}

func (careReadFake) Notes(context.Context, primitive.ObjectID) ([]domain.AdminNote, error) {
	return []domain.AdminNote{{Text: "hello"}}, nil
}
func (careReadFake) Items(context.Context, primitive.ObjectID) ([]domain.SupportItem, error) {
	return nil, nil
}
func (careReadFake) ItemsFor(context.Context, []primitive.ObjectID) ([]domain.SupportItem, error) {
	return nil, nil
}
func (careReadFake) AcksFor(context.Context, []primitive.ObjectID, time.Time) ([]domain.SupportAck, error) {
	return nil, nil
}

func wbApp(users []domain.User) *fiber.App {
	h := NewWellbeingViewsHandler(wbUsersFake{users}, avRecords{}, avStats{}, avHolidays{}, careReadFake{})
	loc, _ := time.LoadLocation("Asia/Bangkok")
	h.now = func() time.Time { return time.Date(2026, 9, 9, 18, 0, 0, 0, loc) }
	app := fiber.New()
	app.Get("/admin/reflections/board", h.Board)
	app.Get("/admin/learners", h.Learners)
	app.Get("/admin/learners/:id/overview", h.Overview)
	app.Get("/admin/support", h.Support)
	app.Get("/admin/reflections/weeks", h.Weeks)
	return app
}

func TestWellbeingViews(t *testing.T) {
	u := domain.User{ID: primitive.NewObjectID(), FirstName: "Mint", Role: "learner", Reflections: []domain.Reflection{
		{Day: "2026-09-08", ReflectionData: domain.ReflectionContent{Barometer: "Panic Zone"}},
		{Day: "2026-09-09", ReflectionData: domain.ReflectionContent{Barometer: "Panic Zone"}},
	}}
	app := wbApp([]domain.User{u})
	cases := map[string]string{
		"/admin/reflections/board?cohort=13":          `"date":"2026-09-09"`,
		"/admin/learners?cohort=13":                   `"support":true`,
		"/admin/learners/" + u.ID.Hex() + "/overview": `"panic_run"`,
		"/admin/support?cohort=13":                    `"pattern:panic_run"`,
		"/admin/reflections/weeks?cohort=13&weeks=2":  `"week":"2026-09-07"`,
	}
	for url, want := range cases {
		code, body := send(app, "GET", url, "")
		if code != 200 || !strings.Contains(body, want) {
			t.Errorf("%s → %d, want body containing %s: %s", url, code, want, body)
		}
	}
	for _, bad := range []string{"/admin/reflections/weeks", "/admin/reflections/weeks?cohort=13&weeks=99", "/admin/reflections/board", "/admin/reflections/board?cohort=13&range=year", "/admin/learners/" + primitive.NewObjectID().Hex() + "/overview"} {
		if code, _ := send(app, "GET", bad, ""); code != 400 && code != 404 {
			t.Errorf("%s → %d, want 400/404", bad, code)
		}
	}
	_ = httptest.NewRequest
}

func TestLearnersReadStoredStats(t *testing.T) {
	u := domain.User{ID: primitive.NewObjectID(), FirstName: "Mint", Role: "learner"}
	seen := time.Date(2026, 9, 9, 1, 55, 0, 0, time.UTC)
	h := NewWellbeingViewsHandler(wbUsersFake{[]domain.User{u}}, avRecords{}, avStats{[]domain.LearnerStats{{UserID: u.ID, AttendanceRate: 0.5, LeaveUsed: 6, LateRun: 3, LastSeen: &seen}}}, avHolidays{}, careReadFake{})
	loc, _ := time.LoadLocation("Asia/Bangkok")
	h.now = func() time.Time { return time.Date(2026, 9, 9, 18, 0, 0, 0, loc) }
	app := fiber.New()
	app.Get("/admin/learners", h.Learners)
	app.Get("/admin/support", h.Support)
	_, body := send(app, "GET", "/admin/learners?cohort=13", "")
	for _, want := range []string{`"attendance_rate":0.5`, `"leave_used":6`, `"support":true`, `"last_seen":"2026-09-09T01:55:00Z"`} {
		if !strings.Contains(body, want) {
			t.Errorf("learners body missing %s: %s", want, body)
		}
	}
	if _, body := send(app, "GET", "/admin/support?cohort=13", ""); !strings.Contains(body, `"late_run"`) || !strings.Contains(body, `"Leave 6/8"`) {
		t.Errorf("support body: %s", body)
	}
}

// wbCallUsers records the last (n, textDay) the handler asked for.
type wbCallUsers struct {
	wbUsersFake
	n       int
	textDay string
}

func (u *wbCallUsers) ListCohortLearnersWithReflections(_ int, _ bson.M, n int, textDay string) ([]domain.User, error) {
	u.n, u.textDay = n, textDay
	return u.list, nil
}

type holidayOn string

func (h holidayOn) IsHoliday(d string) (bool, *domain.Holiday, error) { return d == string(h), nil, nil }

func TestTextNightSpansHolidayWeekend(t *testing.T) {
	users := &wbCallUsers{}
	h := NewWellbeingViewsHandler(users, avRecords{}, avStats{}, holidayOn("2026-09-11"), careReadFake{})
	loc, _ := time.LoadLocation("Asia/Bangkok")
	h.now = func() time.Time { return time.Date(2026, 9, 14, 10, 0, 0, 0, loc) }
	app := fiber.New()
	app.Get("/admin/learners", h.Learners)
	app.Get("/admin/support", h.Support)
	app.Get("/admin/reflections/board", h.Board)
	for _, url := range []string{"/admin/support?cohort=13", "/admin/learners?cohort=13"} {
		users.n, users.textDay = 0, ""
		if code, body := send(app, "GET", url, ""); code != 200 {
			t.Fatalf("%s → %d %s", url, code, body)
		}
		if users.n != 20 || users.textDay != "2026-09-10" {
			t.Errorf("%s loaded n=%d textDay=%q, want 20 and Thursday 2026-09-10", url, users.n, users.textDay)
		}
	}
	send(app, "GET", "/admin/reflections/board?cohort=13", "")
	nights := h.classNights(20)
	if users.n != 30 || users.textDay != nights[len(nights)-1] {
		t.Errorf("board loaded n=%d textDay=%q, want 30 and %s", users.n, users.textDay, nights[len(nights)-1])
	}
}
