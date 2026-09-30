package startupstory

import (
	"fmt"
	"math"
	"math/rand/v2"
	"time"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func rngFor(run *domain.StartupRun) *rand.Rand {
	r := rand.New(rand.NewPCG(uint64(run.Seed), uint64(run.Step)))
	run.Step++
	return r
}

func NewRun(ownerID primitive.ObjectID, cohort int, role, mode string, seed uint64, now time.Time) *domain.StartupRun {
	run := &domain.StartupRun{
		OwnerID: ownerID, Cohort: cohort, Role: role, Mode: mode, Seed: int64(seed),
		Status: domain.StartupStatusActive, Stage: domain.StartupStageFounder,
		Money: StartingMoney, Staff: []domain.StartupDev{}, CreatedAt: now, UpdatedAt: now,
	}
	r := rngFor(run)
	for i, j := range r.Perm(len(founders))[:FounderOffers] {
		f := founders[j]
		f.ID = fmt.Sprintf("founder-%d", i)
		f.Name = founderNames[r.IntN(len(founderNames))]
		run.FounderOffer = append(run.FounderOffer, f)
	}
	return run
}

func PickFounder(run *domain.StartupRun, index int) error {
	if run.Stage != domain.StartupStageFounder {
		return domain.ErrStartupWrongStage
	}
	if index < 0 || index >= len(run.FounderOffer) {
		return domain.ErrStartupInvalidChoice
	}
	run.Staff = []domain.StartupDev{run.FounderOffer[index]}
	run.FounderOffer = nil
	run.Stage = domain.StartupStageHub
	return nil
}

func StartProject(run *domain.StartupRun, typeName, theme string, staffIDs []string, now time.Time) error {
	if run.Stage != domain.StartupStageHub {
		return domain.ErrStartupWrongStage
	}
	if _, ok := findType(typeName); !ok || !validTheme(theme) {
		return domain.ErrStartupInvalidChoice
	}
	if len(staffIDs) == 0 {
		for _, s := range run.Staff {
			staffIDs = append(staffIDs, s.ID)
		}
	}
	if _, err := teamFor(run, staffIDs); err != nil {
		return err
	}
	run.Project = &domain.StartupProject{
		Type: typeName, Theme: theme, StaffIDs: staffIDs,
		StartedAt: now, EndsAt: now.Add(ProjectDuration * time.Second),
	}
	run.LastResult = nil
	run.Stage = domain.StartupStageDeveloping
	return nil
}

func Ship(run *domain.StartupRun, now time.Time) error {
	if run.Stage != domain.StartupStageDeveloping || run.Project == nil {
		return domain.ErrStartupWrongStage
	}
	if now.Before(run.Project.EndsAt) {
		return domain.ErrStartupTooEarly
	}
	result, err := evaluate(run)
	if err != nil {
		return err
	}
	run.Money += result.MoneyDelta
	run.Fans += result.FansDelta
	run.LastResult = result
	run.Project = nil
	run.ProjectIndex++
	if run.ProjectIndex >= ProjectsPerRun {
		endRun(run, "finished", now)
		return nil
	}
	run.Stage = domain.StartupStageHub
	return nil
}

func endRun(run *domain.StartupRun, outcome string, now time.Time) {
	run.Status = domain.StartupStatusEnded
	run.Stage = domain.StartupStageEnded
	run.Outcome = outcome
	run.Score = max(0, run.Money/10) + run.Fans
	run.EndedAt = &now
}

func teamFor(run *domain.StartupRun, staffIDs []string) ([]domain.StartupDev, error) {
	seen := map[string]bool{}
	team := []domain.StartupDev{}
	for _, id := range staffIDs {
		if seen[id] {
			return nil, domain.ErrStartupInvalidChoice
		}
		seen[id] = true
		found := false
		for _, s := range run.Staff {
			if s.ID == id {
				team = append(team, s)
				found = true
			}
		}
		if !found {
			return nil, domain.ErrStartupInvalidChoice
		}
	}
	if len(team) == 0 {
		return nil, domain.ErrStartupInvalidChoice
	}
	return team, nil
}

func stats(d domain.StartupDev) [4]int {
	return [4]int{d.Frontend, d.Backend, d.Design, d.Debug}
}

func evaluate(run *domain.StartupRun) (*domain.StartupResult, error) {
	p := run.Project
	t, _ := findType(p.Type)
	team, err := teamFor(run, p.StaffIDs)
	if err != nil {
		return nil, err
	}
	combo := comboFor(t, p.Theme)

	var power float64
	var sums [4]int
	debug, salaries := 0, 0
	for _, d := range team {
		s := stats(d)
		for i := range s {
			power += float64(s[i]) * t.Weights[i]
			sums[i] += s[i]
		}
		debug += d.Debug
		salaries += d.Salary
	}
	power *= comboMultipliers[combo]
	bugs := max(0, 2*len(team)-debug/2)

	r := rngFor(run)
	quality := (power - 0.3*float64(bugs)) * (0.9 + 0.2*r.Float64())

	result := &domain.StartupResult{Type: p.Type, Theme: p.Theme, Combo: combo, Bugs: bugs}
	for _, rv := range reviewers {
		bias := 0.0
		if rv.Favor >= 0 {
			bias = (float64(sums[rv.Favor])/float64(len(team)) - 3) * 0.4
		}
		if rv.Name == "Dev Community" {
			bias += map[string]float64{"great": 1, "good": 0, "meh": -1}[combo]
		}
		score := int(math.Round(quality*ReviewScale + bias + (2*r.Float64() - 1)))
		score = min(10, max(1, score))
		tier := 2
		if score <= 4 {
			tier = 0
		} else if score <= 7 {
			tier = 1
		}
		lines := rv.Lines[tier]
		result.Reviews = append(result.Reviews, domain.StartupReview{Reviewer: rv.Name, Score: score, Line: lines[r.IntN(len(lines))]})
		result.Total += score
	}
	result.MoneyDelta = result.Total*result.Total*MoneyPerPoint - salaries
	result.FansDelta = result.Total * result.Total * FansPerPoint
	return result, nil
}
