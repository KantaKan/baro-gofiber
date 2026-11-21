package handler

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gofiber-baro/internal/domain"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type avUsers struct{ list []domain.User }

func (u avUsers) GetAllUsers(int, string, string, string, string, int, int, int, ...string) ([]domain.User, int, error) {
	return u.list, len(u.list), nil
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
