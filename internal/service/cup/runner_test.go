package cup

import (
	"context"
	"errors"
	"testing"
	"time"

	"gofiber-baro/internal/domain"
	"gofiber-baro/internal/service/giftbox"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type fakeRecords struct {
	recs []domain.AttendanceRecord
}

func (f *fakeRecords) FindRecords(_ interface{}, flt domain.AttendanceRecordFilter, _ interface{}) ([]domain.AttendanceRecord, error) {
	out := []domain.AttendanceRecord{}
	for _, r := range f.recs {
		if r.CohortNumber == flt.Cohort && r.Date == flt.Date {
			out = append(out, r)
		}
	}
	return out, nil
}
func (f *fakeRecords) ActiveCohorts(_ interface{}, _ string) ([]int, error) { return []int{13}, nil }

type fakeGranter struct {
	calls []string
	fail  bool
}

func (g *fakeGranter) GrantAudience(_ context.Context, cohort int, team, adminID, rarity, msg, key, pool string) (*giftbox.CohortGrantResult, error) {
	if g.fail {
		return nil, errors.New("mongo down")
	}
	g.calls = append(g.calls, team+"|"+key+"|"+rarity+"|"+pool+"|"+msg)
	return &giftbox.CohortGrantResult{Total: 1, Created: 1}, nil
}

type rulesOnly struct{}

func (rulesOnly) Get(int) (domain.CohortSettings, error) {
	return domain.CohortSettings{StartDate: "2026-07-06", Rules: domain.DefaultRules()}, nil
}

func weekdays(t time.Time) (bool, error) {
	return t.Weekday() != time.Saturday && t.Weekday() != time.Sunday, nil
}

func rec(u primitive.ObjectID, date string, s domain.AttendanceSession, st domain.AttendanceStatus) domain.AttendanceRecord {
	return domain.AttendanceRecord{UserID: u, CohortNumber: 13, Date: date, Session: s, Status: st}
}

func setup() (*fakeStore, *fakeRecords, *fakeGranter, *Runner, primitive.ObjectID, primitive.ObjectID, primitive.ObjectID) {
	st, recs, g := newFake(), &fakeRecords{}, &fakeGranter{}
	a1, a2, b1 := primitive.NewObjectID(), primitive.NewObjectID(), primitive.NewObjectID()
	st.members["A"] = []primitive.ObjectID{a1, a2}
	st.members["B"] = []primitive.ObjectID{b1}
	return st, recs, g, NewRunner(NewService(st, rulesOnly{}), st, recs, rulesOnly{}, g, weekdays), a1, a2, b1
}

func countCat(st *fakeStore, cat string) int {
	n := 0
	for _, a := range st.awards {
		if a.Category == cat {
			n++
		}
	}
	return n
}

func TestShowingUpBonusOncePerDay(t *testing.T) {
	st, recs, _, r, a1, a2, b1 := setup()
	day := "2026-09-08"
	for _, u := range []primitive.ObjectID{a1, a2} {
		recs.recs = append(recs.recs, rec(u, day, domain.SessionMorning, domain.StatusPresent), rec(u, day, domain.SessionAfternoon, domain.StatusPresent))
	}
	recs.recs = append(recs.recs, rec(b1, day, domain.SessionMorning, domain.StatusLate), rec(b1, day, domain.SessionAfternoon, domain.StatusPresent))

	r.RunDue(at(2026, 9, 8, 14, 0)) // before afternoon closes: nothing yet
	if countCat(st, CatShowingUp) != 0 {
		t.Fatal("bonus before the afternoon closed")
	}
	r.RunDue(at(2026, 9, 8, 14, 31))
	r.RunDue(at(2026, 9, 8, 14, 32))
	r2 := NewRunner(r.svc, st, recs, rulesOnly{}, &fakeGranter{}, weekdays) // server restart
	r2.RunDue(at(2026, 9, 8, 14, 33))
	if n := countCat(st, CatShowingUp); n != 1 {
		t.Fatalf("showing_up awards = %d, want 1 (A only, once)", n)
	}
	if a := st.awards[0]; a.Group != "A" || a.Points != ShowingUpPoints || a.Week != "2026-09-07" {
		t.Errorf("award = %+v", a)
	}
}

func TestBonusWaitsForMissingRecords(t *testing.T) {
	st, recs, _, r, a1, a2, _ := setup()
	day := "2026-09-08"
	recs.recs = append(recs.recs, rec(a1, day, domain.SessionMorning, domain.StatusPresent), rec(a1, day, domain.SessionAfternoon, domain.StatusPresent),
		rec(a2, day, domain.SessionMorning, domain.StatusPresent))
	r.RunDue(at(2026, 9, 8, 14, 31)) // a2's afternoon not written yet by the closer
	recs.recs = append(recs.recs, rec(a2, day, domain.SessionAfternoon, domain.StatusAbsentExcused))
	r.RunDue(at(2026, 9, 8, 14, 32))
	if countCat(st, CatShowingUp) != 1 {
		t.Fatal("group should get the bonus once the missing record arrives")
	}
}

func TestWeekCloseGrantsOnce(t *testing.T) {
	st, _, g, r, a1, a2, _ := setup()
	_, _ = r.svc.Award(admin, 13, "A", CatCoding, 40, "Demo", at(2026, 9, 9, 11, 0))
	_, _ = r.svc.Award(admin, 13, "B", CatSoft, 30, "Class", at(2026, 9, 9, 11, 0))
	r.RunDue(at(2026, 9, 11, 16, 44))
	if len(st.wins) != 0 {
		t.Fatal("closed before Friday 16:45")
	}
	r.RunDue(at(2026, 9, 11, 16, 45))
	r.RunDue(at(2026, 9, 11, 16, 46))
	if len(st.wins) != 1 || len(g.calls) != 1 {
		t.Fatalf("wins=%d grants=%d, want 1/1", len(st.wins), len(g.calls))
	}
	w := st.wins[0]
	if w.Group != "A" || w.Points != 40 || w.WeekNo != 10 || len(w.Members) != 2 || (w.Members[0] != a1 && w.Members[0] != a2) {
		t.Errorf("win = %+v", w)
	}
	if want := "A|cup:13:2026-09-07|Rare|character-box|GenMate Cup · Week 10 champions"; g.calls[0] != want {
		t.Errorf("grant = %q", g.calls[0])
	}
}

func TestMissedFridayClosesLater(t *testing.T) {
	st, _, g, r, _, _, _ := setup()
	_, _ = r.svc.Award(admin, 13, "B", CatSoft, 10, "Class", at(2026, 9, 9, 11, 0))
	r.RunDue(at(2026, 9, 14, 9, 0)) // server was down Friday; Monday morning tick
	if len(st.wins) != 1 || st.wins[0].Week != "2026-09-07" || len(g.calls) != 1 {
		t.Fatalf("missed week not closed: wins=%+v grants=%d", st.wins, len(g.calls))
	}
}

func TestZeroWeekAndFailedGrant(t *testing.T) {
	st, _, g, r, _, _, _ := setup()
	r.RunDue(at(2026, 9, 11, 17, 0))
	if len(st.wins) != 0 || len(g.calls) != 0 {
		t.Fatal("all-zero week must not have a winner or a gift")
	}
	_, _ = r.svc.Award(admin, 13, "A", CatCoding, 5, "x", at(2026, 9, 16, 11, 0))
	g.fail = true
	r.RunDue(at(2026, 9, 18, 17, 0))
	if len(st.wins) != 0 {
		t.Fatal("win must not be recorded when the gift failed")
	}
	g.fail = false
	r.RunDue(at(2026, 9, 18, 17, 1))
	if len(st.wins) != 1 || len(g.calls) != 1 {
		t.Fatalf("retry after failure: wins=%d grants=%d", len(st.wins), len(g.calls))
	}
}
