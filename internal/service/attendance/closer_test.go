package attendance

import (
	"errors"
	"testing"
	"time"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson"
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
type fakeUsers struct {
	list  []domain.User
	calls *int
}

func (f fakeUsers) ListCohortLearners(cohort int, _ bson.M) ([]domain.User, error) {
	if f.calls != nil {
		*f.calls++
	}
	var out []domain.User
	for _, u := range f.list {
		if u.CohortNumber == cohort {
			out = append(out, u)
		}
	}
	return out, nil
}

type fakeCloserRecords struct {
	existing  []domain.AttendanceRecord
	upserts   int
	byCohort  map[int]int
	bySession map[domain.AttendanceSession]int
	active    []int
	failOn    domain.AttendanceSession
}

func (f *fakeCloserRecords) FindRecords(_ interface{}, _ domain.AttendanceRecordFilter, _ interface{}) ([]domain.AttendanceRecord, error) {
	return f.existing, nil
}
func (f *fakeCloserRecords) InsertIfAbsent(_ interface{}, r *domain.AttendanceRecord) (bool, error) {
	if f.failOn != "" && r.Session == f.failOn {
		return false, errors.New("boom")
	}
	f.upserts++
	if f.byCohort == nil {
		f.byCohort, f.bySession = map[int]int{}, map[domain.AttendanceSession]int{}
	}
	f.byCohort[r.CohortNumber]++
	f.bySession[r.Session]++
	return true, nil
}

func (f *fakeCloserRecords) ActiveCohorts(_ interface{}, _ string) ([]int, error) {
	if f.active == nil {
		return []int{13}, nil
	}
	return f.active, nil
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

func TestCloseIfDue_OnlyActiveCohorts(t *testing.T) {
	users := fakeUsers{list: []domain.User{
		{ID: primitive.NewObjectID(), CohortNumber: 12, Role: "learner"}, // alumni
		{ID: primitive.NewObjectID(), CohortNumber: 13, Role: "learner"},
	}}
	recs := &fakeCloserRecords{active: []int{13}}
	c := NewSessionCloser(recs, fakeLeaves{}, users, fakeSettings{}, fakeHolidays{})
	_ = c.CloseIfDue(at(10, 31, 0))
	if recs.byCohort[12] != 0 || recs.byCohort[13] != 1 {
		t.Errorf("want only cohort 13 closed, got %v", recs.byCohort)
	}
}

func TestCloseIfDue_ContinuesAfterFailure(t *testing.T) {
	c, recs := newCloser(false)
	recs.failOn = domain.SessionMorning
	_ = c.CloseIfDue(at(14, 31, 0))
	if recs.bySession[domain.SessionAfternoon] != 2 {
		t.Errorf("afternoon should still close after a morning failure, got %v", recs.bySession)
	}
}

func TestCloseIfDue_NoUserLoadUntilDue(t *testing.T) {
	calls := 0
	users := fakeUsers{list: []domain.User{{ID: primitive.NewObjectID(), CohortNumber: 13, Role: "learner"}}, calls: &calls}
	c := NewSessionCloser(&fakeCloserRecords{}, fakeLeaves{}, users, fakeSettings{}, fakeHolidays{})
	_ = c.CloseIfDue(at(9, 0, 0))
	if calls != 0 {
		t.Errorf("loaded learners %d times before any session was due", calls)
	}
	_ = c.CloseIfDue(at(10, 31, 0))
	_ = c.CloseIfDue(at(10, 32, 0))
	if calls != 1 {
		t.Errorf("want one learner load for the morning close, got %d", calls)
	}
}

func TestCloseIfDue_SkipsDroppedLearners(t *testing.T) {
	users := fakeUsers{list: []domain.User{
		{ID: primitive.NewObjectID(), CohortNumber: 13, Role: "learner"},
		{ID: primitive.NewObjectID(), CohortNumber: 13, Role: "learner", AttendanceStatus: "dropout"},
		{ID: primitive.NewObjectID(), CohortNumber: 13, Role: "learner", AttendanceStatus: "dismissed"},
	}}
	recs := &fakeCloserRecords{}
	c := NewSessionCloser(recs, fakeLeaves{}, users, fakeSettings{}, fakeHolidays{})
	_ = c.CloseIfDue(at(10, 31, 0))
	if recs.upserts != 1 {
		t.Errorf("only the active learner should be marked, got %d", recs.upserts)
	}
}
