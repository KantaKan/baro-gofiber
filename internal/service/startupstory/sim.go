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
	return newRunWithPool(ownerID, cohort, role, mode, seed, now, founderPool(nil), nil)
}

func newRunWithPool(ownerID primitive.ObjectID, cohort int, role, mode string, seed uint64, now time.Time, founderCandidates []domain.StartupDev, itemExtras []string) *domain.StartupRun {
	run := &domain.StartupRun{
		OwnerID: ownerID, Cohort: cohort, Role: role, Mode: mode, Seed: int64(seed),
		Status: domain.StartupStatusActive, Stage: domain.StartupStageFounder, Act: 1,
		Money: StartingMoney, Staff: []domain.StartupDev{}, UnlockedItems: itemExtras,
		CreatedAt: now, UpdatedAt: now,
	}
	m := rngFor(run)
	perm := m.Perm(len(themes))
	run.Market = domain.StartupMarket{
		Hot:  []string{themes[perm[0]], themes[perm[1]]},
		Cold: []string{themes[perm[2]]},
	}
	bo := rngFor(run)
	bperm := bo.Perm(len(bossPool))
	run.BossOrder = []string{bossPool[bperm[0]], bossPool[bperm[1]]}
	r := rngFor(run)
	for i, j := range r.Perm(len(founderCandidates))[:FounderOffers] {
		f := founderCandidates[j]
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
	run.Founder = run.FounderOffer[index].Title
	run.FounderOffer = nil
	run.Act = 1
	run.MaxAct = 1
	onNewAct(run)
	run.Candidates = rollCandidates(run, CandidateOffers)
	refreshPitches(run)
	refreshNextBoss(run)
	run.Stage = domain.StartupStageHub
	return nil
}

const genmateDrawChance = 0.25

func usedGenmateIDs(run *domain.StartupRun) map[string]bool {
	used := map[string]bool{}
	for _, s := range run.Staff {
		if s.GenmateID != "" {
			used[s.GenmateID] = true
		}
	}
	return used
}

func rollCandidates(run *domain.StartupRun, count int) []domain.StartupDev {
	r := rngFor(run)
	seq := run.Step
	max := statMax(run.Act)
	used := usedGenmateIDs(run)
	avail := []domain.StartupGenmate{}
	for _, g := range run.GenmatePool {
		if !used[g.UserID.Hex()] {
			avail = append(avail, g)
		}
	}
	out := make([]domain.StartupDev, 0, count)
	for i := 0; i < count; i++ {
		name, genmateID := "", ""
		if len(avail) > 0 && r.Float64() < genmateDrawChance {
			pick := r.IntN(len(avail))
			name, genmateID = avail[pick].Name, avail[pick].UserID.Hex()
			avail = append(avail[:pick], avail[pick+1:]...)
		} else {
			name = founderNames[r.IntN(len(founderNames))]
		}
		role := roles[r.IntN(len(roles))]
		st := [4]int{r.IntN(max) + 1, r.IntN(max) + 1, r.IntN(max) + 1, r.IntN(max) + 1}
		applyRoleFocus(&st, role.ID, max)
		fe, be, design, debug := st[0], st[1], st[2], st[3]
		trait := ""
		if r.IntN(2) == 1 {
			trait = traits[r.IntN(len(traits))]
		}
		out = append(out, domain.StartupDev{
			ID:        fmt.Sprintf("cand-%d-%d", seq, i),
			Name:      name,
			Title:     role.Title,
			Role:      role.ID,
			GenmateID: genmateID,
			Sprite:    "dev",
			Trait:     trait,
			Frontend:  fe,
			Backend:   be,
			Design:    design,
			Debug:     debug,
			Salary:    salaryFor(fe, be, design, debug, trait),
		})
	}
	return out
}

func Hire(run *domain.StartupRun, candidateID string) error {
	if run.Stage != domain.StartupStageHub {
		return domain.ErrStartupWrongStage
	}
	idx := -1
	for i, c := range run.Candidates {
		if c.ID == candidateID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return domain.ErrStartupInvalidChoice
	}
	if len(run.Staff) >= TeamCap(run.Act) {
		return domain.ErrStartupTeamFull
	}
	cand := run.Candidates[idx]
	if run.Money < cand.Salary {
		return domain.ErrStartupNoFunds
	}
	run.Money -= cand.Salary
	run.Staff = append(run.Staff, cand)
	run.Candidates = append(run.Candidates[:idx], run.Candidates[idx+1:]...)
	return nil
}

func Dismiss(run *domain.StartupRun, staffID string) error {
	if run.Stage != domain.StartupStageHub {
		return domain.ErrStartupWrongStage
	}
	if len(run.Staff) <= 1 {
		return domain.ErrStartupInvalidChoice
	}
	idx := -1
	for i, s := range run.Staff {
		if s.ID == staffID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return domain.ErrStartupInvalidChoice
	}
	run.Staff = append(run.Staff[:idx], run.Staff[idx+1:]...)
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
	team, err := teamFor(run, staffIDs)
	if err != nil {
		return err
	}
	boss := ""
	if isBossIndex(run.ProjectIndex) {
		boss = bossForRun(run.ProjectIndex, run.BossOrder)
		onBossStart(run)
	}
	secs := projectDurationSecs(run, team, boss)
	run.Project = &domain.StartupProject{
		Type: typeName, Theme: theme, Boss: boss, StaffIDs: staffIDs,
		StartedAt: now, EndsAt: now.Add(time.Duration(secs) * time.Second),
	}
	run.LastResult = nil
	run.Stage = domain.StartupStageDeveloping
	return nil
}

func projectDurationSecs(run *domain.StartupRun, team []domain.StartupDev, boss string) int {
	base := baseDurationSecs(run.Act)
	if boss != "" {
		base = BossDuration
	}
	mult := 1.0
	for _, d := range team {
		switch d.Trait {
		case TraitNightOwl:
			mult *= 0.85
		case TraitMeeting:
			mult *= 1.2
		case TraitPixelPerf:
			mult *= 1.15
		}
	}
	mult *= effectsOf(run.Items).durMult
	mult *= jobsFor(team).durMult
	mult *= perkDurationMult(run, team)
	secs := int(math.Round(float64(base) * mult))
	if secs < MinDurationSecs {
		secs = MinDurationSecs
	}
	return secs
}

func Ship(run *domain.StartupRun, now time.Time) error {
	if run.Stage != domain.StartupStageDeveloping || run.Project == nil {
		return domain.ErrStartupWrongStage
	}
	if now.Before(run.Project.EndsAt) {
		return domain.ErrStartupTooEarly
	}
	boss := run.Project.Boss
	if boss == BossRequirements {
		swapTheme(run)
	}
	team, err := teamFor(run, run.Project.StaffIDs)
	if err != nil {
		return err
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
	afterShipLevels(run, team, result)
	if boss != "" {
		if result.Total < bossPassMark(run.Act, boss) {
			endRun(run, domain.StartupOutcomePivot, now)
			return nil
		}
		run.BossesPassed++
		run.Money += BossBonusMoney
		run.Fans += BossBonusFans
		result.MoneyDelta += BossBonusMoney
		result.FansDelta += BossBonusFans
	}
	if run.Money < 0 {
		endRun(run, domain.StartupOutcomePivot, now)
		return nil
	}
	if afterShipBurnout(run, team, now) {
		return nil
	}
	if endlessCheckpoint(run, boss, now) {
		return nil
	}
	prevAct := run.Act
	run.Act = actForRun(run)
	run.MaxAct = max(run.MaxAct, run.Act)
	if run.Act != prevAct {
		onNewAct(run)
	}
	run.Candidates = rollCandidates(run, CandidateOffers)
	refreshPitches(run)
	refreshNextBoss(run)
	if boss != "" {
		run.Stage = domain.StartupStageHub
	} else {
		run.ItemOffer = rollItemOffer(run, CandidateOffers)
		run.Stage = domain.StartupStageItem
	}
	queuePerk(run)
	queueEvent(run)
	return nil
}

func interrupt(run *domain.StartupRun, stage string) {
	if run.ResumeStage == "" {
		run.ResumeStage = run.Stage
	}
	run.Stage = stage
}

func resume(run *domain.StartupRun) {
	run.Stage = run.ResumeStage
	run.ResumeStage = ""
}

func swapTheme(run *domain.StartupRun) {
	r := rngFor(run)
	pool := []string{}
	for _, th := range themes {
		if th != run.Project.Theme {
			pool = append(pool, th)
		}
	}
	run.Project.Theme = pool[r.IntN(len(pool))]
}

func Abandon(run *domain.StartupRun, now time.Time) error {
	if run.Stage == domain.StartupStageEnded || run.Status != domain.StartupStatusActive {
		return domain.ErrStartupWrongStage
	}
	endRun(run, domain.StartupOutcomePivot, now)
	return nil
}

func rollItemOffer(run *domain.StartupRun, count int) []string {
	r := rngFor(run)
	catalog := itemPool(run.UnlockedItems)
	offer := []string{}
	inOffer := map[string]bool{}
	for len(offer) < count {
		total := 0
		for _, rw := range rarityWeights {
			for _, it := range catalog {
				if it.Rarity == rw.Rarity && !inOffer[it.ID] {
					total += rw.Weight
					break
				}
			}
		}
		if total == 0 {
			break
		}
		pick := r.IntN(total)
		chosen := ""
		for _, rw := range rarityWeights {
			pool := []string{}
			for _, it := range catalog {
				if it.Rarity == rw.Rarity && !inOffer[it.ID] {
					pool = append(pool, it.ID)
				}
			}
			if len(pool) == 0 {
				continue
			}
			pick -= rw.Weight
			if pick < 0 {
				chosen = pool[r.IntN(len(pool))]
				break
			}
		}
		if chosen == "" {
			break
		}
		inOffer[chosen] = true
		offer = append(offer, chosen)
	}
	return offer
}

func PickItem(run *domain.StartupRun, index int) error {
	if run.Stage != domain.StartupStageItem {
		return domain.ErrStartupWrongStage
	}
	if index == SkipItemForRetreat {
		teamRetreat(run)
	} else if index < 0 || index >= len(run.ItemOffer) {
		return domain.ErrStartupInvalidChoice
	} else {
		run.Items = append(run.Items, run.ItemOffer[index])
	}
	run.ItemOffer = nil
	run.Stage = domain.StartupStageHub
	return nil
}

type itemEffects struct {
	sums         [4]int
	powerMult    float64
	bugs         int
	durMult      float64
	investor     int
	devCommunity int
	burnout      int
}

func effectsOf(items []string) itemEffects {
	fx := itemEffects{powerMult: 1, durMult: 1}
	for _, id := range items {
		it, ok := findItem(id)
		if !ok {
			continue
		}
		fx.sums[0] += it.FE
		fx.sums[1] += it.BE
		fx.sums[2] += it.Design
		fx.sums[3] += it.Debug
		for i := range fx.sums {
			fx.sums[i] += it.AllStats
		}
		fx.powerMult *= it.PowerMult
		fx.bugs += it.Bugs
		fx.durMult *= it.DurationMult
		fx.investor += it.Investor
		fx.devCommunity += it.DevCommunity
		fx.burnout += it.Burnout
	}
	return fx
}

func endRun(run *domain.StartupRun, outcome string, now time.Time) {
	run.Status = domain.StartupStatusEnded
	run.Stage = domain.StartupStageEnded
	run.Outcome = outcome
	score := int(math.Floor(float64(run.Money)/10)) + run.Fans + BossPassBonus*run.BossesPassed
	if outcome == domain.StartupOutcomeIPO {
		score += BossWinBonus
	}
	score += endlessScoreBonus(run)
	run.Score = score
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
	return effectiveStats(d)
}

func effectiveStats(d domain.StartupDev) [4]int {
	s := [4]int{d.Frontend, d.Backend, d.Design, d.Debug}
	switch d.Trait {
	case TraitTabs:
		s[3] += 2
		s[2] -= 1
	case TraitSOSurfer:
		s[1] += 2
	case TraitPixelPerf:
		s[2] += 2
	case TraitTenX:
		for i := range s {
			s[i] += 2
		}
	}
	for i := range s {
		if s[i] < 0 {
			s[i] = 0
		}
	}
	return s
}

func evaluate(run *domain.StartupRun) (*domain.StartupResult, error) {
	p := run.Project
	t, _ := findType(p.Type)
	team, err := teamFor(run, p.StaffIDs)
	if err != nil {
		return nil, err
	}
	combo := comboFor(t, p.Theme)

	weights := t.Weights
	if p.Boss == BossOutage {
		weights = outageWeights
	}
	sc := &scoring{run: run, team: team, combo: combo, weights: weights, powerMult: 1, reviewer: map[string]float64{}, reviewers: reviewers, moneyMult: 1, fansMult: 1}
	applyGimmicks(sc)
	applyPerks(sc)
	applyOSS(sc)
	weights = sc.weights
	jobs := jobsFor(team)
	var sums [4]int
	var power float64
	debug, salaries, traitBugs, investorBonus := 0, 0, 0, 0
	for _, d := range team {
		s := stats(d)
		share := 1.0
		if !isBuilder(d) {
			share = SupportBuildShare
		}
		for i := range s {
			sums[i] += s[i]
			power += share * float64(s[i]) * weights[i]
		}
		debug += s[3]
		salaries += d.Salary
		switch d.Trait {
		case TraitNightOwl:
			traitBugs += 2
		case TraitSOSurfer:
			traitBugs += 2
		case TraitMeeting:
			investorBonus++
		}
	}
	fx := effectsOf(run.Items)
	for i := range sums {
		sums[i] += fx.sums[i]
		power += float64(fx.sums[i]) * weights[i]
	}
	debug += fx.sums[3]
	power *= coordinationMult(len(team), jobs.hasPM)
	if p.Boss == BossOutage {
		power *= 1 + jobs.outageBoost
	}
	power *= comboMultipliers[combo]
	power *= marketMult(run.Market, p.Theme)
	power *= fx.powerMult
	power *= sc.powerMult
	bugs := max(0, 2*len(team)-debug/2+traitBugs+fx.bugs+jobs.bugAdd+sc.bugs-int(math.Round(jobs.bugCut)))

	r := rngFor(run)
	quality := (power - BugPenalty*float64(bugs)) / actScale(run.Act) * (0.9 + 0.2*r.Float64())

	result := &domain.StartupResult{Type: p.Type, Theme: p.Theme, Combo: combo, Bugs: bugs}
	for _, rv := range sc.reviewers {
		bias := 0.0
		if rv.Favor >= 0 {
			bias = (float64(sums[rv.Favor])/float64(len(team)) - 3) * 0.4
		}
		if rv.Name == "Dev Community" {
			bias += map[string]float64{"great": 1, "good": 0, "meh": -1}[combo]
		}
		if rv.Name == "Investor" {
			bias += float64(investorBonus + fx.investor)
			switch marketMult(run.Market, p.Theme) {
			case MarketHotMult:
				bias += 1.5
			case MarketColdMult:
				bias -= 1
			}
		}
		if rv.Name == "Dev Community" {
			bias += float64(fx.devCommunity)
		}
		bias += jobs.reviewer[rv.Name]
		bias += sc.reviewer[rv.Name]
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
		if p.Boss == BossIPO && rv.Name == "Investor" {
			result.Total += score
		}
	}
	result.MoneyDelta = int(math.Round(float64(result.Total*result.Total*MoneyPerPoint)*sc.moneyMult)) - salaries
	result.FansDelta = int(math.Round(float64(result.Total*result.Total*FansPerPoint) * sc.fansMult))
	return result, nil
}

type scoring struct {
	run       *domain.StartupRun
	team      []domain.StartupDev
	combo     string
	weights   [4]float64
	powerMult float64
	bugs      int
	reviewer  map[string]float64
	reviewers []reviewer
	moneyMult float64
	fansMult  float64
}
