package attendance

import (
	"errors"
	"testing"
	"time"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type oneUser struct{ u domain.User }

func (o oneUser) GetUserByID(string) (*domain.User, error) { u := o.u; return &u, nil }
func (o oneUser) GetAllUsers(int, string, string, string, string, int, int, int, ...string) ([]domain.User, int, error) {
	return []domain.User{o.u}, 1, nil
}

type fakeCheckinRecords struct{ inserted bool }

func (f fakeCheckinRecords) CountRecords(interface{}, domain.AttendanceRecordFilter) (int64, error) {
	return 0, nil
}
func (f fakeCheckinRecords) InsertIfAbsent(interface{}, *domain.AttendanceRecord) (bool, error) {
	return f.inserted, nil
}

type noLeaves struct{}

func (noLeaves) FindByUserID(interface{}, primitive.ObjectID) ([]domain.LeaveRequest, error) {
	return nil, nil
}

func TestCheckIn_RaceBecomesAlreadySubmitted(t *testing.T) {
	s := NewCheckinService(fakeCheckinRecords{inserted: false}, noLeaves{}, oneUser{domain.User{CohortNumber: 13}},
		fakeSettings{}, fakeHolidays{}, func([]string, string) bool { return true })
	s.now = func() time.Time { return at(9, 5, 0) }
	_, err := s.CheckIn(primitive.NewObjectID(), "1.2.3.4", "slide")
	if !errors.Is(err, ErrAlreadySubmitted) {
		t.Errorf("want ErrAlreadySubmitted when a live record already exists, got %v", err)
	}
}

func TestCheckIn_Inserts(t *testing.T) {
	s := NewCheckinService(fakeCheckinRecords{inserted: true}, noLeaves{}, oneUser{domain.User{CohortNumber: 13}},
		fakeSettings{}, fakeHolidays{}, func([]string, string) bool { return true })
	s.now = func() time.Time { return at(9, 5, 0) }
	rec, err := s.CheckIn(primitive.NewObjectID(), "1.2.3.4", "cat")
	if err != nil || rec.Status != domain.StatusPresent || rec.CheckinStyle != "cat" {
		t.Errorf("got %+v, %v", rec, err)
	}
}

type userRecs struct{}

func (userRecs) FindRecords(interface{}, domain.AttendanceRecordFilter, interface{}) ([]domain.AttendanceRecord, error) {
	return nil, nil
}

func TestSummaryIncludesOwnPreferences(t *testing.T) {
	p := NewProgressService(userRecs{}, oneUser{domain.User{CohortNumber: 13, CheckinStyle: "cat", CheckinRandom: true}}, fakeSettings{}, fakeHolidays{})
	sum, err := p.Summary(primitive.NewObjectID())
	if err != nil {
		t.Fatal(err)
	}
	if sum.Preferences.CheckinStyle != "cat" || !sum.Preferences.CheckinRandom {
		t.Errorf("preferences missing: %+v", sum.Preferences)
	}
}

func TestSummaryClassToday(t *testing.T) {
	p := NewProgressService(userRecs{}, oneUser{domain.User{CohortNumber: 13}}, fakeSettings{}, fakeHolidays{holiday: true})
	p.now = func() time.Time { return at(9, 0, 0) } // Tuesday, but a holiday
	sum, err := p.Summary(primitive.NewObjectID())
	if err != nil {
		t.Fatal(err)
	}
	if sum.ClassToday {
		t.Error("holiday should not be a class day")
	}
	p = NewProgressService(userRecs{}, oneUser{domain.User{CohortNumber: 13}}, fakeSettings{}, fakeHolidays{})
	p.now = func() time.Time { return at(9, 0, 0) }
	if sum, _ = p.Summary(primitive.NewObjectID()); !sum.ClassToday {
		t.Error("a normal Tuesday is a class day")
	}
}

func TestSummaryChampion(t *testing.T) {
	p := NewProgressService(userRecs{}, oneUser{domain.User{CohortNumber: 13, GenmateGroup: "A"}}, fakeSettings{}, fakeHolidays{})
	sum, _ := p.Summary(primitive.NewObjectID())
	if sum.Champion {
		t.Fatal("champion without a checker")
	}
	p.SetChampionCheck(func(u domain.User) bool { return u.GenmateGroup == "A" })
	if sum, _ = p.Summary(primitive.NewObjectID()); !sum.Champion {
		t.Error("summary should report champion from the Cup")
	}
}
