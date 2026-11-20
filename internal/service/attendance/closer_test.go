package attendance

import (
	"testing"
	"time"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestPlanSessionClose(t *testing.T) {
	a, b, c, d := primitive.NewObjectID(), primitive.NewObjectID(), primitive.NewObjectID(), primitive.NewObjectID()
	got := PlanSessionClose([]primitive.ObjectID{a, b, c, d},
		map[primitive.ObjectID]bool{a: true},
		map[primitive.ObjectID]domain.AttendanceStatus{c: domain.StatusAbsentExcused, d: domain.StatusLateExcused})
	want := map[primitive.ObjectID]domain.AttendanceStatus{b: domain.StatusAbsent, c: domain.StatusAbsentExcused, d: domain.StatusLateExcused}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: got %s want %s", k.Hex(), got[k], v)
		}
	}
}

func TestPlanSessionClose_AllIn(t *testing.T) {
	a := primitive.NewObjectID()
	if got := PlanSessionClose([]primitive.ObjectID{a}, map[primitive.ObjectID]bool{a: true}, nil); len(got) != 0 {
		t.Errorf("expected nothing, got %v", got)
	}
}

// --- fakes ---
type fakeUsers struct{ list []domain.User }

func (f fakeUsers) GetUserByID(id string) (*domain.User, error) { return nil, nil }
func (f fakeUsers) GetAllUsers(cohort int, role, email, search, sort string, sortDir, page, limit int, ex ...string) ([]domain.User, int, error) {
	return f.list, len(f.list), nil
}

type fakeCloserRecords struct {
	existing []domain.AttendanceRecord
	upserts  int
}

func (f *fakeCloserRecords) FindRecords(_ interface{}, _ domain.AttendanceRecordFilter, _ interface{}) ([]domain.AttendanceRecord, error) {
	return f.existing, nil
}
func (f *fakeCloserRecords) UpsertRecord(_ interface{}, r *domain.AttendanceRecord) error {
	f.upserts++
	return nil
}

type fakeLeaves struct{}

func (fakeLeaves) FindAll(_ interface{}, _ domain.LeaveRequestFilter) ([]domain.LeaveRequest, error) {
	return nil, nil
}

type fakeSettings struct{}

func (fakeSettings) Get(c int) (domain.CohortSettings, error) {
	return domain.CohortSettings{CohortNumber: c, Rules: domain.DefaultRules()}, nil
}

type fakeHolidays struct{ holiday bool }

func (f fakeHolidays) IsHoliday(string) (bool, *domain.Holiday, error) { return f.holiday, nil, nil }

func newCloser(holiday bool) (*SessionCloser, *fakeCloserRecords) {
	users := fakeUsers{list: []domain.User{
		{ID: primitive.NewObjectID(), CohortNumber: 13, Role: "learner"},
		{ID: primitive.NewObjectID(), CohortNumber: 13, Role: "learner"},
	}}
	recs := &fakeCloserRecords{}
	return NewSessionCloser(recs, fakeLeaves{}, users, fakeSettings{}, fakeHolidays{holiday}), recs
}

func TestCloseIfDue_Idempotent(t *testing.T) {
	c, recs := newCloser(false)
	now := at(10, 31, 0) // Tuesday 8 Sep 2026
	if err := c.CloseIfDue(now); err != nil {
		t.Fatal(err)
	}
	if err := c.CloseIfDue(now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if recs.upserts != 2 {
		t.Errorf("want 2 upserts (one per learner), got %d", recs.upserts)
	}
}

func TestCloseIfDue_NotYet(t *testing.T) {
	c, recs := newCloser(false)
	_ = c.CloseIfDue(at(10, 29, 0))
	if recs.upserts != 0 {
		t.Errorf("closed too early: %d upserts", recs.upserts)
	}
}

func TestCloseIfDue_Holiday(t *testing.T) {
	c, recs := newCloser(true)
	_ = c.CloseIfDue(at(10, 31, 0))
	if recs.upserts != 0 {
		t.Errorf("holiday should write nothing: %d upserts", recs.upserts)
	}
}
