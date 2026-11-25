package handler

import (
	"io"
	"net/http/httptest"
	"strings"
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

type avRecords struct{ list []domain.AttendanceRecord }

func (r avRecords) FindRecords(interface{}, domain.AttendanceRecordFilter, interface{}) ([]domain.AttendanceRecord, error) {
	return r.list, nil
}

type avSettings struct{}

func (avSettings) Get(int) (domain.CohortSettings, error) {
	return domain.CohortSettings{Rules: domain.DefaultRules()}, nil
}

type avHolidays struct{}

func (avHolidays) IsHoliday(string) (bool, *domain.Holiday, error) { return false, nil, nil }

func avApp() *fiber.App {
	h := NewAdminViewsHandler(avUsers{[]domain.User{{ID: primitive.NewObjectID(), FirstName: "A", GenmateGroup: "F", Role: "learner"}}}, avRecords{}, avSettings{}, avHolidays{})
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

type rangeRecords struct{ list []domain.AttendanceRecord }

func (r rangeRecords) FindRecords(_ interface{}, f domain.AttendanceRecordFilter, _ interface{}) ([]domain.AttendanceRecord, error) {
	var out []domain.AttendanceRecord
	for _, rec := range r.list {
		if f.DateFrom != "" && (rec.Date < f.DateFrom || rec.Date > f.DateTo) {
			continue
		}
		if f.DateFrom == "" {
			rec = domain.AttendanceRecord{UserID: rec.UserID, Date: rec.Date, Session: rec.Session, Status: rec.Status}
		}
		out = append(out, rec)
	}
	return out, nil
}

func TestLoadCohortRecordsFullOnlyInRange(t *testing.T) {
	u := primitive.NewObjectID()
	ip := "1.2.3.4"
	src := rangeRecords{[]domain.AttendanceRecord{
		{UserID: u, Date: "2026-11-24", Session: domain.SessionMorning, Status: domain.StatusPresent, IPAddress: ip},
		{UserID: u, Date: "2026-11-25", Session: domain.SessionMorning, Status: domain.StatusLate, IPAddress: ip},
		{UserID: u, Date: "2026-11-25", Session: domain.SessionAfternoon, Status: domain.StatusPresent, IPAddress: ip},
	}}
	recs, err := loadCohortRecords(src, 13, "2026-11-25", "2026-11-25")
	if err != nil || len(recs) != 3 {
		t.Fatalf("want 3 records, got %d (%v)", len(recs), err)
	}
	for _, r := range recs {
		if full := r.IPAddress != ""; full != (r.Date == "2026-11-25") {
			t.Errorf("%s: full record = %v", r.Date, full)
		}
	}
	if hist, _ := loadCohortRecords(src, 13, "", ""); len(hist) != 3 {
		t.Errorf("history only: want 3, got %d", len(hist))
	}
}
