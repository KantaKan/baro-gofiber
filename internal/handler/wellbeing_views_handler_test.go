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
	h := NewWellbeingViewsHandler(wbUsersFake{[]domain.User{u}}, avRecords{}, avStats{list: []domain.LearnerStats{{UserID: u.ID, AttendanceRate: 0.5, LeaveUsed: 6, LateRun: 3, LastSeen: &seen}}}, avHolidays{}, careReadFake{})
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

func (h holidayOn) IsHoliday(d string) (bool, *domain.Holiday, error) {
	return d == string(h), nil, nil
}

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
	nights := h.classNights(20)
	for url, n := range map[string]int{"/admin/reflections/board?cohort=13": 20, "/admin/reflections/board?cohort=13&range=week": 20, "/admin/reflections/board?cohort=13&range=4w": 30} {
		send(app, "GET", url, "")
		if users.n != n || users.textDay != nights[len(nights)-1] {
			t.Errorf("%s loaded n=%d textDay=%q, want %d and %s", url, users.n, users.textDay, n, nights[len(nights)-1])
		}
	}
}

func TestOverviewDaysBadgesNeighbors(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Bangkok")
	a := domain.User{ID: primitive.NewObjectID(), FirstName: "Aom", JSDNumber: "01", CohortNumber: 13, Role: "learner"}
	sentAt := time.Date(2026, 9, 8, 10, 2, 0, 0, time.UTC)
	replyAt := time.Date(2026, 9, 8, 11, 0, 0, 0, time.UTC)
	b := domain.User{ID: primitive.NewObjectID(), FirstName: "Bee", JSDNumber: "02", CohortNumber: 13, Role: "learner",
		Badges: []domain.Badge{{Name: "Seedling", Emoji: "🌱", AwardedAt: time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)}},
		Reflections: []domain.Reflection{
			{ID: primitive.NewObjectID(), Day: "2026-09-08", CreatedAt: sentAt, AdminFeedback: "nice", AdminFeedbackName: "Nan", AdminFeedbackAt: &replyAt,
				ReflectionData: domain.ReflectionContent{Barometer: "Stretch zone - overwhelmed", TechSessions: domain.SessionDetails{Improve: "react query &amp; cache"}}},
			{ID: primitive.NewObjectID(), Date: time.Date(2026, 9, 7, 10, 0, 0, 0, time.UTC), ReflectionData: domain.ReflectionContent{Barometer: "Comfort Zone"}},
		}}
	c := domain.User{ID: primitive.NewObjectID(), FirstName: "Cat", JSDNumber: "03", CohortNumber: 13, Role: "learner"}
	in := time.Date(2026, 9, 8, 6, 24, 0, 0, time.UTC)
	recs := avRecords{list: []domain.AttendanceRecord{
		{UserID: b.ID, Date: "2026-09-07", Session: domain.SessionMorning, Status: domain.StatusPresent, SubmittedAt: in.AddDate(0, 0, -1)},
		{UserID: b.ID, Date: "2026-09-08", Session: domain.SessionAfternoon, Status: domain.StatusLate, SubmittedAt: in},
	}}
	stats := avStats{list: []domain.LearnerStats{{UserID: a.ID, AttendanceRate: 1}, {UserID: b.ID, AttendanceRate: 0.5}}}
	h := NewWellbeingViewsHandler(wbUsersFake{[]domain.User{c, a, b}}, recs, stats, avHolidays{}, careReadFake{})
	h.now = func() time.Time { return time.Date(2026, 9, 8, 18, 0, 0, 0, loc) }
	app := fiber.New()
	app.Get("/admin/learners/:id/overview", h.Overview)
	_, body := send(app, "GET", "/admin/learners/"+b.ID.Hex()+"/overview", "")
	for _, want := range []string{
		`"date":"2026-09-07"`, `"date":"2026-09-08"`,
		`"afternoon":{"status":"late","at":"2026-09-08T06:24:00Z"`,
		`"zone":"stretch-overwhelmed"`, `"tech_improve":"react query \u0026 cache"`, `"reply":"nice"`, `"reply_name":"Nan"`,
		`"zone":"comfort"`,
		`"cohort_attendance":0.75`,
		`"name":"Seedling"`,
		`"prev":"` + a.ID.Hex() + `"`, `"next":"` + c.ID.Hex() + `"`, `"index":1`, `"total":3`,
		`"best_streak":1`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("overview missing %s\n%s", want, body)
		}
	}
	if strings.Contains(body, `"date":"2026-09-06"`) {
		t.Error("Sunday listed as a class day")
	}
}

type countingHolidays struct{ single, ranged *int }

func (h countingHolidays) IsHoliday(string) (bool, *domain.Holiday, error) {
	*h.single++
	return false, nil, nil
}
func (h countingHolidays) GetHolidayDatesInRange(string, string) (map[string]bool, error) {
	*h.ranged++
	return map[string]bool{}, nil
}

func overviewFor(t *testing.T, h *WellbeingViewsHandler, u domain.User) string {
	t.Helper()
	loc, _ := time.LoadLocation("Asia/Bangkok")
	h.now = func() time.Time { return time.Date(2026, 9, 8, 18, 0, 0, 0, loc) }
	app := fiber.New()
	app.Get("/admin/learners/:id/overview", h.Overview)
	_, body := send(app, "GET", "/admin/learners/"+u.ID.Hex()+"/overview", "")
	return body
}

func TestOverviewLooksUpHolidaysOnce(t *testing.T) {
	u := domain.User{ID: primitive.NewObjectID(), FirstName: "Bee", CohortNumber: 13, Role: "learner"}
	single, ranged := 0, 0
	h := NewWellbeingViewsHandler(wbUsersFake{[]domain.User{u}}, avRecords{}, avStats{}, countingHolidays{&single, &ranged}, careReadFake{})
	h.SetCohortSettings(func(int) (domain.CohortSettings, error) { return domain.CohortSettings{StartDate: "2025-01-06"}, nil })
	body := overviewFor(t, h, u)
	if ranged != 1 {
		t.Errorf("range lookups = %d, want 1", ranged)
	}
	if single > 60 {
		t.Errorf("per-day holiday lookups = %d; course days must not query one by one", single)
	}
	if !strings.Contains(body, `"date":"2026-09-08"`) {
		t.Error("today missing from days")
	}
}

func TestOverviewFallsBackOnBadStartDate(t *testing.T) {
	u := domain.User{ID: primitive.NewObjectID(), FirstName: "Bee", CohortNumber: 13, Role: "learner"}
	recs := avRecords{list: []domain.AttendanceRecord{{UserID: u.ID, Date: "2026-09-07", Session: domain.SessionMorning, Status: domain.StatusPresent}}}
	for _, start := range []string{"2026/09/07", "2027-01-04"} {
		h := NewWellbeingViewsHandler(wbUsersFake{[]domain.User{u}}, recs, avStats{}, avHolidays{}, careReadFake{})
		h.SetCohortSettings(func(int) (domain.CohortSettings, error) { return domain.CohortSettings{StartDate: start}, nil })
		if body := overviewFor(t, h, u); !strings.Contains(body, `"days":[{"date":"2026-09-07"`) {
			t.Errorf("start %q: days lost the learner's history", start)
		}
	}
}

func TestOverviewKeepsWeekendReflectionsAndMarkedBy(t *testing.T) {
	u := domain.User{ID: primitive.NewObjectID(), FirstName: "Bee", CohortNumber: 13, Role: "learner", SelectedPalette: "sunset", SelectedSpecies: "fern",
		Reflections: []domain.Reflection{{ID: primitive.NewObjectID(), Day: "2026-09-06", ReflectionData: domain.ReflectionContent{Barometer: "Comfort Zone", TechSessions: domain.SessionDetails{Happy: "sunday study"}}}}}
	recs := avRecords{list: []domain.AttendanceRecord{{UserID: u.ID, Date: "2026-09-07", Session: domain.SessionMorning, Status: domain.StatusPresent, MarkedBy: domain.MarkedBySelf, SubmittedAt: time.Date(2026, 9, 7, 1, 50, 0, 0, time.UTC)}}}
	h := NewWellbeingViewsHandler(wbUsersFake{[]domain.User{u}}, recs, avStats{}, avHolidays{}, careReadFake{})
	body := overviewFor(t, h, u)
	for _, want := range []string{`"date":"2026-09-06","extra":true`, `"tech_happy":"sunday study"`, `"marked_by":"self"`, `"plant":{"flower":"","leaf":"","palette":"sunset","pot":"","species":"fern","stem":""}`} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %s\n%s", want, body)
		}
	}
}

func journeyBody(t *testing.T, u domain.User, graduated string) (int, string) {
	t.Helper()
	loc, _ := time.LoadLocation("Asia/Bangkok")
	h := NewWellbeingViewsHandler(wbUsersFake{[]domain.User{u}}, avRecords{}, avStats{}, avHolidays{}, careReadFake{})
	h.now = func() time.Time { return time.Date(2026, 12, 20, 10, 0, 0, 0, loc) }
	h.SetCohortSettings(func(int) (domain.CohortSettings, error) {
		return domain.CohortSettings{StartDate: "2026-12-07", GraduatedOn: graduated}, nil
	})
	app := fiber.New()
	app.Get("/users/me/journey", func(c *fiber.Ctx) error { c.Locals("userID", u.ID.Hex()); return c.Next() }, h.Journey)
	return send(app, "GET", "/users/me/journey", "")
}

func TestJourneyUnlocksAfterGraduation(t *testing.T) {
	u := domain.User{ID: primitive.NewObjectID(), FirstName: "Fern", CohortNumber: 13, Role: "learner"}
	if _, body := journeyBody(t, u, "2026-12-21"); !strings.Contains(body, `"unlocked":false`) || strings.Contains(body, `"name"`) {
		t.Errorf("before graduation: %s", body)
	}
	if _, body := journeyBody(t, u, ""); !strings.Contains(body, `"unlocked":false`) {
		t.Errorf("no graduation date: %s", body)
	}
	_, body := journeyBody(t, u, "2026-12-18")
	if !strings.Contains(body, `"unlocked":true`) || !strings.Contains(body, `"name":"Fern"`) || !strings.Contains(body, `"class_days":10`) || !strings.Contains(body, `"last_day":"2026-12-18"`) {
		t.Errorf("after graduation: %s", body)
	}
	admin := domain.User{ID: primitive.NewObjectID(), Role: "admin", CohortNumber: 13}
	if code, _ := journeyBody(t, admin, "2026-12-18"); code != 404 {
		t.Errorf("admins have no journey, got %d", code)
	}
}

func TestJourneyForACohortThatGraduatedLongAgo(t *testing.T) {
	u := domain.User{ID: primitive.NewObjectID(), FirstName: "Old", CohortNumber: 5, Role: "learner",
		Reflections: []domain.Reflection{{ID: primitive.NewObjectID(), Day: "2024-03-04", ReflectionData: domain.ReflectionContent{Barometer: "Comfort Zone"}}}}
	loc, _ := time.LoadLocation("Asia/Bangkok")
	h := NewWellbeingViewsHandler(wbUsersFake{[]domain.User{u}}, avRecords{}, avStats{}, avHolidays{}, careReadFake{})
	h.now = func() time.Time { return time.Date(2026, 12, 20, 10, 0, 0, 0, loc) }
	h.SetCohortSettings(func(int) (domain.CohortSettings, error) {
		return domain.CohortSettings{StartDate: "2024-03-04", GraduatedOn: "2024-03-15"}, nil
	})
	app := fiber.New()
	app.Get("/j", func(c *fiber.Ctx) error { c.Locals("userID", u.ID.Hex()); return c.Next() }, h.Journey)
	_, body := send(app, "GET", "/j", "")
	if !strings.Contains(body, `"class_days":10`) || !strings.Contains(body, `"first_day":"2024-03-04"`) {
		t.Errorf("a cohort that graduated in 2024 must keep its class days: %s", body)
	}
}
