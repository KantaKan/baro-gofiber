package handler

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"gofiber-baro/internal/service/staff"

	"github.com/gofiber/fiber/v2"
)

func TestMergeCohorts(t *testing.T) {
	got := MergeCohorts(map[int]int{13: 52, 12: 48, 14: 0}, map[int]string{13: "2026-06-29", 14: "2027-01-04", 11: "2025-06-02"})
	if len(got) != 4 || got[0].Cohort != 14 || got[1].Cohort != 13 || got[3].Cohort != 11 {
		t.Fatalf("newest first, from learners and settings: %+v", got)
	}
	if got[1].Learners != 52 || got[1].StartDate != "2026-06-29" || got[3].Learners != 0 || got[2].StartDate != "" {
		t.Errorf("fields: %+v", got)
	}
	if len(MergeCohorts(nil, nil)) != 0 || MergeCohorts(nil, nil) == nil {
		t.Error("empty list, not nil")
	}
}

type stubCohortList struct{}

func (stubCohortList) LearnerCounts() (map[int]int, error) { return map[int]int{13: 52, 14: 40}, nil }
func (stubCohortList) StartDates() (map[int]string, error) {
	return map[int]string{13: "2026-06-29"}, nil
}

func cohortListApp(role string, cohorts []int) *fiber.App {
	app := fiber.New()
	h := NewCohortListHandler(stubCohortList{})
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("staff", staff.Ctx{UserID: "u", Role: role, Cohorts: cohorts})
		return c.Next()
	})
	app.Get("/admin/cohorts", h.List)
	return app
}

func TestCohortListScope(t *testing.T) {
	body := func(app *fiber.App) string {
		res, _ := app.Test(httptest.NewRequest("GET", "/admin/cohorts", nil))
		b, _ := io.ReadAll(res.Body)
		return string(b)
	}
	if b := body(cohortListApp(staff.Owner, nil)); !strings.Contains(b, `"cohort":14`) || !strings.Contains(b, `"cohort":13`) {
		t.Errorf("owner sees every cohort: %s", b)
	}
	if b := body(cohortListApp(staff.TA, []int{13})); strings.Contains(b, `"cohort":14`) || !strings.Contains(b, `"cohort":13`) {
		t.Errorf("a TA only sees their cohorts: %s", b)
	}
}
