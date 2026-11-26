package handler

import (
	"context"
	"io"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"gofiber-baro/internal/domain"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type avUsers struct{ list []domain.User }

func (u avUsers) ListCohortLearners(int, bson.M) ([]domain.User, error) {
	return u.list, nil
}

func (u avUsers) ListCohortLearnersWithReflections(int, bson.M, int, string) ([]domain.User, error) {
	return u.list, nil
}

type avRecords struct{ list []domain.AttendanceRecord }

func (r avRecords) FindRecords(interface{}, domain.AttendanceRecordFilter, interface{}) ([]domain.AttendanceRecord, error) {
	return r.list, nil
}

// avRangeRecords records the filters (and projections) a handler asked for; views query in parallel.
type avRangeRecords struct {
	filters *[]domain.AttendanceRecordFilter
	opts    *[]interface{}
}

var avRangeMu sync.Mutex

func (r avRangeRecords) FindRecords(_ interface{}, f domain.AttendanceRecordFilter, o interface{}) ([]domain.AttendanceRecord, error) {
	avRangeMu.Lock()
	defer avRangeMu.Unlock()
	*r.filters = append(*r.filters, f)
	if r.opts != nil {
		*r.opts = append(*r.opts, o)
	}
	return nil, nil
}

type avSettings struct{}

func (avSettings) Get(int) (domain.CohortSettings, error) {
	return domain.CohortSettings{Rules: domain.DefaultRules()}, nil
}

type avHolidays struct{}

func (avHolidays) IsHoliday(string) (bool, *domain.Holiday, error) { return false, nil, nil }

func avApp() *fiber.App {
	h := NewAdminViewsHandler(avUsers{[]domain.User{{ID: primitive.NewObjectID(), FirstName: "A", GenmateGroup: "F", Role: "learner"}}}, avRecords{}, avStats{}, avSettings{}, avHolidays{})
	loc, _ := time.LoadLocation("Asia/Bangkok")
	h.now = func() time.Time { return time.Date(2026, 9, 8, 9, 0, 0, 0, loc) }
	app := fiber.New()
	app.Get("/admin/today", h.Today)
	app.Get("/admin/attendance/register", h.Register)
	return app
}

func TestAdminToday(t *testing.T) {
	res, _ := avApp().Test(httptest.NewRequest("GET", "/admin/today?cohort=13", nil))
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 || !strings.Contains(string(b), `"learners"`) || !strings.Contains(string(b), `"first_name":"A"`) {
		t.Fatalf("status %d body %s", res.StatusCode, b)
	}
	if res, _ := avApp().Test(httptest.NewRequest("GET", "/admin/today", nil)); res.StatusCode != 400 {
		t.Errorf("missing cohort → %d, want 400", res.StatusCode)
	}
}

func TestAdminRegister(t *testing.T) {
	res, _ := avApp().Test(httptest.NewRequest("GET", "/admin/attendance/register?cohort=13", nil))
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 || !strings.Contains(string(b), `"week":"2026-09-07"`) {
		t.Fatalf("status %d body %s", res.StatusCode, b)
	}
	for _, bad := range []string{"/admin/attendance/register?cohort=13&week=2026-09-08", "/admin/attendance/register?cohort=13&week=nope", "/admin/attendance/register"} {
		if res, _ := avApp().Test(httptest.NewRequest("GET", bad, nil)); res.StatusCode != 400 {
			t.Errorf("%s → %d, want 400", bad, res.StatusCode)
		}
	}
}

type markRecords struct {
	existing []domain.AttendanceRecord
	updated  map[string]interface{}
}

func (m *markRecords) FindRecords(interface{}, domain.AttendanceRecordFilter, interface{}) ([]domain.AttendanceRecord, error) {
	return m.existing, nil
}
func (m *markRecords) UpdateRecord(_ interface{}, id primitive.ObjectID, update interface{}) error {
	if m.updated == nil {
		m.updated = map[string]interface{}{}
	}
	m.updated[id.Hex()] = update
	return nil
}

type fakeMarker struct{ id primitive.ObjectID }

func (f fakeMarker) ManualMarkAttendance(u primitive.ObjectID, date, session string, st domain.AttendanceStatus, by string) (*domain.AttendanceRecord, error) {
	return &domain.AttendanceRecord{ID: f.id, UserID: u, Date: date, Session: domain.AttendanceSession(session), Status: st, MarkedBy: domain.MarkedByAdmin}, nil
}

func markApp(recs *markRecords, id primitive.ObjectID) *fiber.App {
	h := NewAdminMarkHandler(fakeMarker{id}, recs)
	app := fiber.New()
	app.Post("/admin/attendance/mark", h.Mark)
	app.Post("/admin/attendance/:id/restore", h.Restore)
	return app
}

func TestAdminMarkReturnsPrevious(t *testing.T) {
	id := primitive.NewObjectID()
	at := time.Date(2026, 9, 8, 2, 10, 0, 0, time.UTC)
	recs := &markRecords{existing: []domain.AttendanceRecord{{ID: id, Status: domain.StatusPresent, MarkedBy: domain.MarkedBySelf, SubmittedAt: at, CheckinStyle: "cat"}}}
	req := httptest.NewRequest("POST", "/admin/attendance/mark", strings.NewReader(`{"user_id":"`+primitive.NewObjectID().Hex()+`","date":"2026-09-08","session":"morning","status":"late"}`))
	req.Header.Set("Content-Type", "application/json")
	res, _ := markApp(recs, id).Test(req)
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 || !strings.Contains(string(b), `"previous":{`) || !strings.Contains(string(b), `"checkin_style":"cat"`) || !strings.Contains(string(b), `"marked_by":"self"`) {
		t.Fatalf("status %d body %s", res.StatusCode, b)
	}
	recs.existing = nil
	req = httptest.NewRequest("POST", "/admin/attendance/mark", strings.NewReader(`{"user_id":"`+primitive.NewObjectID().Hex()+`","date":"2026-09-08","session":"morning","status":"late"}`))
	req.Header.Set("Content-Type", "application/json")
	res, _ = markApp(recs, id).Test(req)
	b, _ = io.ReadAll(res.Body)
	if !strings.Contains(string(b), `"previous":null`) {
		t.Errorf("no previous record → previous null, got %s", b)
	}
}

func TestAdminRestore(t *testing.T) {
	id := primitive.NewObjectID()
	recs := &markRecords{}
	body := `{"status":"present","marked_by":"self","marked_by_user":"","submitted_at":"2026-09-08T02:10:00Z","checkin_style":"cat"}`
	req := httptest.NewRequest("POST", "/admin/attendance/"+id.Hex()+"/restore", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res, _ := markApp(recs, id).Test(req)
	if res.StatusCode != 200 || recs.updated[id.Hex()] == nil {
		t.Fatalf("restore → %d, updated %v", res.StatusCode, recs.updated)
	}
	bad := httptest.NewRequest("POST", "/admin/attendance/"+id.Hex()+"/restore", strings.NewReader(`{"status":"nope"}`))
	bad.Header.Set("Content-Type", "application/json")
	if res, _ := markApp(recs, id).Test(bad); res.StatusCode != 400 {
		t.Errorf("bad status → %d, want 400", res.StatusCode)
	}
}

func TestLoadCohortRecordsIsHistoryOnly(t *testing.T) {
	var got []domain.AttendanceRecordFilter
	var opts []interface{}
	if _, err := loadCohortRecords(avRangeRecords{&got, &opts}, 13); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != (domain.AttendanceRecordFilter{Cohort: 13, NotDeleted: true}) {
		t.Fatalf("filters = %+v, want the cohort's live records, no date bounds", got)
	}
	if !reflect.DeepEqual(opts[0], historyFields()) {
		t.Errorf("projection = %+v, want historyFields", opts[0])
	}
}

type avStats struct {
	list     []domain.LearnerStats
	withDays *[]bool
}

func (s avStats) FindByCohort(_ context.Context, _ int, withLast10 bool) ([]domain.LearnerStats, error) {
	if s.withDays != nil {
		*s.withDays = append(*s.withDays, withLast10)
	}
	return s.list, nil
}

func TestOnlyTodayReadsLast10(t *testing.T) {
	var got []bool
	h := NewAdminViewsHandler(avUsers{}, avRecords{}, avStats{withDays: &got}, avSettings{}, avHolidays{})
	app := fiber.New()
	app.Get("/admin/today", h.Today)
	app.Get("/admin/attendance/register", h.Register)
	app.Test(httptest.NewRequest("GET", "/admin/today?cohort=13", nil))
	app.Test(httptest.NewRequest("GET", "/admin/attendance/register?cohort=13&week=2026-09-07", nil))
	if len(got) != 2 || !got[0] || got[1] {
		t.Fatalf("withLast10 per call = %v, want [true false] (Today, Register)", got)
	}
}

func TestRegisterLoadsSelectedAndPreviousWeekOnly(t *testing.T) {
	var got []domain.AttendanceRecordFilter
	var opts []interface{}
	h := NewAdminViewsHandler(avUsers{}, avRangeRecords{&got, &opts}, avStats{}, avSettings{}, avHolidays{})
	app := fiber.New()
	app.Get("/admin/attendance/register", h.Register)
	if res, _ := app.Test(httptest.NewRequest("GET", "/admin/attendance/register?cohort=13&week=2026-09-07", nil)); res.StatusCode != 200 {
		t.Fatalf("status %d", res.StatusCode)
	}
	if len(got) != 2 {
		t.Fatalf("filters = %+v, want selected week and previous week", got)
	}
	want := map[string]interface{}{"2026-09-07..2026-09-11": viewRecordFields(), "2026-08-31..2026-09-04": historyFields()}
	for i, f := range got {
		key := f.DateFrom + ".." + f.DateTo
		proj, ok := want[key]
		if !ok || f.Cohort != 13 || !f.NotDeleted {
			t.Errorf("unexpected filter %+v", f)
			continue
		}
		if !reflect.DeepEqual(opts[i], proj) {
			t.Errorf("%s projection = %+v, want %+v", key, opts[i], proj)
		}
		delete(want, key)
	}
}

func TestTodayLoadsOnlyToday(t *testing.T) {
	var got []domain.AttendanceRecordFilter
	h := NewAdminViewsHandler(avUsers{}, avRangeRecords{filters: &got}, avStats{}, avSettings{}, avHolidays{})
	loc, _ := time.LoadLocation("Asia/Bangkok")
	h.now = func() time.Time { return time.Date(2026, 9, 8, 9, 0, 0, 0, loc) }
	app := fiber.New()
	app.Get("/admin/today", h.Today)
	app.Test(httptest.NewRequest("GET", "/admin/today?cohort=13", nil))
	if len(got) != 1 || got[0].Date != "2026-09-08" || got[0].Cohort != 13 {
		t.Fatalf("filters = %+v, want today's records only", got)
	}
}
