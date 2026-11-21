package cup

import (
	"context"
	"errors"
	"testing"
	"time"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type fakeStore struct {
	awards  []domain.CupAward
	wins    []domain.CupWin
	members map[string][]primitive.ObjectID // group -> learners
}

func newFake() *fakeStore { return &fakeStore{members: map[string][]primitive.ObjectID{}} }

func (f *fakeStore) Insert(_ context.Context, a *domain.CupAward) error {
	a.ID = primitive.NewObjectID()
	f.awards = append(f.awards, *a)
	return nil
}
func (f *fakeStore) InsertOnce(_ context.Context, a *domain.CupAward) (bool, error) {
	for _, x := range f.awards {
		if x.Key == a.Key {
			return false, nil
		}
	}
	return true, f.Insert(context.Background(), a)
}
func (f *fakeStore) List(_ context.Context, cohort int, week string) ([]domain.CupAward, error) {
	out := []domain.CupAward{}
	for _, a := range f.awards {
		if a.Cohort == cohort && a.Week == week {
			out = append(out, a)
		}
	}
	return out, nil
}
func (f *fakeStore) Delete(_ context.Context, id primitive.ObjectID) error {
	for i, a := range f.awards {
		if a.ID == id {
			f.awards = append(f.awards[:i], f.awards[i+1:]...)
			return nil
		}
	}
	return errors.New("not found")
}
func (f *fakeStore) InsertWinOnce(_ context.Context, w *domain.CupWin) (bool, error) {
	for _, x := range f.wins {
		if x.Cohort == w.Cohort && x.Week == w.Week {
			return false, nil
		}
	}
	f.wins = append(f.wins, *w)
	return true, nil
}
func (f *fakeStore) Wins(_ context.Context, cohort int) ([]domain.CupWin, error) {
	out := []domain.CupWin{}
	for _, w := range f.wins {
		if w.Cohort == cohort {
			out = append(out, w)
		}
	}
	return out, nil
}
func (f *fakeStore) Groups(_ context.Context, cohort int) (map[string][]primitive.ObjectID, error) {
	return f.members, nil
}

type fixedStart string

func (s fixedStart) Get(int) (domain.CohortSettings, error) {
	return domain.CohortSettings{StartDate: string(s)}, nil
}

var admin = primitive.NewObjectID().Hex()

func TestAwardValidation(t *testing.T) {
	f := newFake()
	f.members["A"] = []primitive.ObjectID{primitive.NewObjectID()}
	s := NewService(f, fixedStart("2026-07-06"))
	now := at(2026, 9, 8, 11, 0)
	bad := []struct {
		group, cat string
		pts        int
		reason     string
	}{
		{"A", "dance", 10, "x"},
		{"A", CatShowingUp, 10, "x"}, // server only
		{"A", CatCoding, 0, "x"},
		{"A", CatCoding, 101, "x"},
		{"Z", CatCoding, 10, "x"}, // unknown group
		{"A", CatCoding, 10, string(make([]byte, 141))},
	}
	for _, b := range bad {
		if _, err := s.Award(admin, 13, b.group, b.cat, b.pts, b.reason, now); err == nil {
			t.Errorf("expected error for %+v", b)
		}
	}
	a, err := s.Award(admin, 13, "A", CatCoding, 20, " Demo day ", now)
	if err != nil || a.Week != "2026-09-07" || a.Day != "2026-09-08" || a.Reason != "Demo day" {
		t.Fatalf("Award = %+v, %v", a, err)
	}
}

func TestMe(t *testing.T) {
	f := newFake()
	me, other := primitive.NewObjectID(), primitive.NewObjectID()
	f.members["A"] = []primitive.ObjectID{me}
	f.members["B"] = []primitive.ObjectID{other}
	s := NewService(f, fixedStart("2026-07-06"))
	now := at(2026, 9, 9, 11, 0)
	_, _ = s.Award(admin, 13, "B", CatSoft, 30, "Soft-skill class", now)
	_, _ = s.Award(admin, 13, "A", CatCoding, 10, "Kata", now)
	// last week B won; two weeks ago A won with me as a member (I then moved to... still A)
	f.wins = append(f.wins,
		domain.CupWin{Cohort: 13, Week: "2026-08-31", WeekNo: 9, Group: "B", Points: 50, Members: []primitive.ObjectID{other}},
		domain.CupWin{Cohort: 13, Week: "2026-08-24", WeekNo: 8, Group: "C", Points: 40, Members: []primitive.ObjectID{me}},
	)
	got, err := s.Me(domain.User{ID: me, CohortNumber: 13, GenmateGroup: "A"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if got.Week != "2026-09-07" || got.WeekNo != 10 || !got.EndsAt.Equal(at(2026, 9, 11, 16, 45)) {
		t.Errorf("week fields: %+v", got)
	}
	if len(got.Standings) != 2 || got.Standings[0].Group != "B" || got.MyGroup != "A" {
		t.Errorf("standings/my group: %+v", got)
	}
	if got.ChampionGroup != "B" {
		t.Errorf("champion = %q, want B (last week's winner)", got.ChampionGroup)
	}
	if len(got.MyMedals) != 1 || got.MyMedals[0].WeekNo != 8 {
		t.Errorf("medals follow frozen members: %+v", got.MyMedals)
	}
	if len(got.MyAwards) != 1 || got.MyAwards[0].Reason != "Kata" {
		t.Errorf("my awards: %+v", got.MyAwards)
	}
	if len(got.WallOfFame) != 2 || got.WallOfFame[0].WeekNo != 9 {
		t.Errorf("wall of fame newest first: %+v", got.WallOfFame)
	}

	none, err := s.Me(domain.User{ID: primitive.NewObjectID(), CohortNumber: 13}, now)
	if err != nil || none.MyGroup != "" || len(none.Standings) != 2 {
		t.Errorf("no group: %+v %v", none, err)
	}
}

var _ = time.Now
