package cohort

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gofiber-baro/internal/domain"
)

type Service struct {
	repo domain.CohortSettingsRepository
}

func NewService(repo domain.CohortSettingsRepository) *Service { return &Service{repo: repo} }

// Get returns stored settings, or defaults when the cohort has none.
func (s *Service) Get(cohort int) (domain.CohortSettings, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	stored, err := s.repo.FindByCohort(ctx, cohort)
	if err != nil {
		return domain.CohortSettings{}, err
	}
	if stored == nil {
		return domain.CohortSettings{CohortNumber: cohort, Rules: domain.DefaultRules(), AllowedNetworks: []string{}}, nil
	}
	if stored.AllowedNetworks == nil {
		stored.AllowedNetworks = []string{}
	}
	return *stored, nil
}

func (s *Service) Update(cohort int, in domain.CohortSettings) error {
	if err := in.Rules.Validate(); err != nil {
		return err
	}
	for _, n := range in.AllowedNetworks {
		if !validNetworkEntry(n) {
			return fmt.Errorf("allowed network %q is not an IP address or CIDR range", n)
		}
	}
	if in.CheckinRegion != "" && in.CheckinRegion != "anywhere" && in.CheckinRegion != "thailand" {
		return fmt.Errorf("check-in region must be anywhere or thailand")
	}
	if in.StartDate != "" {
		if _, err := time.Parse("2006-01-02", in.StartDate); err != nil {
			return fmt.Errorf("start date must be YYYY-MM-DD")
		}
	}
	if in.GraduatedOn != "" {
		if _, err := time.Parse("2006-01-02", in.GraduatedOn); err != nil {
			return fmt.Errorf("graduation date must be YYYY-MM-DD")
		}
	}
	in.CohortNumber = cohort
	if in.AllowedNetworks == nil {
		in.AllowedNetworks = []string{}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.repo.Upsert(ctx, &in)
}

// ErrCohortExists: the cohort already has settings; creating it again must not overwrite them.
var ErrCohortExists = errors.New("this cohort already exists")

// Create starts a new cohort with default check-in rules.
func (s *Service) Create(cohort int, startDate string, courseDays int) error {
	if cohort <= 0 || courseDays <= 0 || courseDays > 400 {
		return fmt.Errorf("cohort number and course length are required")
	}
	if _, err := time.Parse("2006-01-02", startDate); err != nil {
		return fmt.Errorf("start date must be YYYY-MM-DD")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if stored, err := s.repo.FindByCohort(ctx, cohort); err != nil {
		return err
	} else if stored != nil {
		return ErrCohortExists
	}
	return s.repo.Upsert(ctx, &domain.CohortSettings{CohortNumber: cohort, Rules: domain.DefaultRules(), AllowedNetworks: []string{}, StartDate: startDate, CourseDays: courseDays, CheckinRegion: "anywhere"})
}
