package cohort

import (
	"context"
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
	in.CohortNumber = cohort
	if in.AllowedNetworks == nil {
		in.AllowedNetworks = []string{}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.repo.Upsert(ctx, &in)
}
