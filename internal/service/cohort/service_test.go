package cohort

import (
	"errors"
	"testing"

	"gofiber-baro/internal/domain"
)

type fakeRepo struct{ m map[int]domain.CohortSettings }

func (f *fakeRepo) FindByCohort(_ interface{}, c int) (*domain.CohortSettings, error) {
	if s, ok := f.m[c]; ok {
		return &s, nil
	}
	return nil, nil
}
func (f *fakeRepo) Upsert(_ interface{}, s *domain.CohortSettings) error {
	f.m[s.CohortNumber] = *s
	return nil
}

func newSvc() (*Service, *fakeRepo) {
	r := &fakeRepo{m: map[int]domain.CohortSettings{}}
	return NewService(r), r
}

func TestGetDefaults(t *testing.T) {
	s, _ := newSvc()
	got, err := s.Get(13)
	if err != nil {
		t.Fatal(err)
	}
	if got.Rules != domain.DefaultRules() || got.CohortNumber != 13 || len(got.AllowedNetworks) != 0 {
		t.Errorf("unexpected defaults: %+v", got)
	}
}

func TestUpdateRejects(t *testing.T) {
	s, r := newSvc()
	bad := domain.CohortSettings{Rules: domain.DefaultRules()}
	bad.Rules.Morning.LateAfter = domain.Clock{H: 8, M: 0}
	if s.Update(13, bad) == nil {
		t.Error("invalid rules accepted")
	}
	if s.Update(13, domain.CohortSettings{Rules: domain.DefaultRules(), AllowedNetworks: []string{"not-an-ip"}}) == nil {
		t.Error("bad network accepted")
	}
	if s.Update(13, domain.CohortSettings{Rules: domain.DefaultRules(), StartDate: "29/06/2026"}) == nil {
		t.Error("bad start date accepted")
	}
	if s.Update(13, domain.CohortSettings{Rules: domain.DefaultRules(), CheckinRegion: "mars"}) == nil {
		t.Error("unknown check-in region accepted")
	}
	if len(r.m) != 0 {
		t.Fatal("nothing should be stored after rejects")
	}
	ok := domain.CohortSettings{Rules: domain.DefaultRules(), AllowedNetworks: []string{"10.0.0.0/8", "203.0.113.7"}}
	if err := s.Update(13, ok); err != nil {
		t.Fatalf("valid settings rejected: %v", err)
	}
	if r.m[13].CohortNumber != 13 || len(r.m[13].AllowedNetworks) != 2 {
		t.Errorf("stored wrong: %+v", r.m[13])
	}
}

func TestCreate(t *testing.T) {
	s, r := newSvc()
	if err := s.Create(14, "2026-12-01", 75); err != nil {
		t.Fatalf("create: %v", err)
	}
	if r.m[14].StartDate != "2026-12-01" || r.m[14].CourseDays != 75 || r.m[14].Rules.Morning.LateAfter.H != 9 {
		t.Errorf("stored %+v", r.m[14])
	}
	if err := s.Create(14, "2027-01-01", 60); !errors.Is(err, ErrCohortExists) || r.m[14].StartDate != "2026-12-01" {
		t.Errorf("existing cohort must give ErrCohortExists and stay unchanged, got %v %+v", err, r.m[14])
	}
	if s.Create(0, "2026-12-01", 75) == nil || s.Create(15, "bad", 75) == nil || s.Create(15, "2026-12-01", 0) == nil {
		t.Error("bad input accepted")
	}
}
