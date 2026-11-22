package wellbeing

import (
	"testing"

	"gofiber-baro/internal/domain"
)

func TestWeeklyRough(t *testing.T) {
	// Week of Mon 7 Sep 2026; the week before starts Mon 31 Aug.
	a := user("A", "B", refl("2026-09-07", P, ""), refl("2026-09-08", P, ""), refl("2026-09-09", O, ""))
	b := user("B", "C", refl("2026-09-10", O, ""), refl("2026-09-01", P, "")) // rough last week too
	c := user("C", "C", refl("2026-09-07", C, ""), refl("2026-09-08", E, ""))
	d := user("D", "C", refl("2026-09-07", P, ""))
	d.AttendanceStatus = "dropout"
	weeks := WeeklyRough([]domain.User{a, b, c, d}, []string{"2026-09-07", "2026-08-31"}, "2026-09-10", map[string]bool{"2026-09-09": false})
	if len(weeks) != 2 || weeks[0].Week != "2026-09-07" || weeks[1].Week != "2026-08-31" {
		t.Fatalf("newest first: %+v", weeks)
	}
	w := weeks[0]
	if len(w.Learners) != 2 || w.Learners[0].Learner.FirstName != "A" || w.Learners[1].Learner.FirstName != "B" {
		t.Fatalf("rough learners, most panic first; comfort-only and dropouts left out: %+v", w.Learners)
	}
	la := w.Learners[0]
	if la.Panic != 2 || la.Overwhelmed != 1 || la.Nights != [5]string{"panic", "panic", "stretch-overwhelmed", "", ""} || la.Again {
		t.Errorf("A: %+v", la)
	}
	if lb := w.Learners[1]; lb.Overwhelmed != 1 || !lb.Again {
		t.Errorf("B was rough the week before as well: %+v", lb)
	}
	if w.Zones["panic"] != 2 || w.Zones["stretch-overwhelmed"] != 2 || w.Zones["comfort"] != 1 || w.Zones["stretch-enjoying"] != 1 {
		t.Errorf("zones: %v", w.Zones)
	}
	// 3 active learners × Mon–Thu (today is Thursday) = 12 possible; written: A3 + B1 + C2 = 6.
	if w.Possible != 12 || w.Sent != 6 {
		t.Errorf("sent %d of %d", w.Sent, w.Possible)
	}
	if weeks[1].Learners == nil || len(weeks[1].Learners) != 1 {
		t.Errorf("last week: %+v", weeks[1].Learners)
	}
}

func TestWeeklyRoughHolidaysAndEmpty(t *testing.T) {
	a := user("A", "B", refl("2026-09-07", C, ""))
	w := WeeklyRough([]domain.User{a}, []string{"2026-09-07"}, "2026-09-20", map[string]bool{"2026-09-08": true})[0]
	if w.Possible != 4 {
		t.Errorf("a holiday isn't a night anyone missed: %d", w.Possible)
	}
	empty := WeeklyRough(nil, []string{"2026-09-07"}, "2026-09-20", nil)
	if len(empty) != 1 || empty[0].Learners == nil || empty[0].Zones == nil {
		t.Errorf("empty week has empty lists, not nil: %+v", empty)
	}
}
