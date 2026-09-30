package startupstory

import (
	"context"
	"errors"
	"math"
	"reflect"
	"testing"
	"time"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var t0 = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

func playProject(t *testing.T, seed uint64, typeName, theme string) *domain.StartupRun {
	t.Helper()
	run := NewRun(primitive.NewObjectID(), 12, "learner", domain.StartupModeFree, seed, t0)
	run.Market = domain.StartupMarket{}
	if err := PickFounder(run, 0); err != nil {
		t.Fatal(err)
	}
	if err := StartProject(run, typeName, theme, nil, t0); err != nil {
		t.Fatal(err)
	}
	if err := Ship(run, run.Project.EndsAt); err != nil {
		t.Fatal(err)
	}
	return run
}

func TestSameSeedAndChoicesGiveSameRun(t *testing.T) {
	a := playProject(t, 42, "Game", "Thai Culture")
	b := playProject(t, 42, "Game", "Thai Culture")
	if !reflect.DeepEqual(a.Staff, b.Staff) || !reflect.DeepEqual(a.LastResult, b.LastResult) || a.Money != b.Money {
		t.Fatalf("runs diverged:\n%+v\n%+v", a.LastResult, b.LastResult)
	}
}

func TestGreatComboBeatsMeh(t *testing.T) {
	for seed := uint64(1); seed <= 50; seed++ {
		great := playProject(t, seed, "Game", "Thai Culture")
		meh := playProject(t, seed, "Game", "Fintech")
		if great.LastResult.Combo != "great" || meh.LastResult.Combo != "meh" {
			t.Fatal("combo table changed")
		}
		if great.LastResult.Total <= meh.LastResult.Total {
			t.Fatalf("seed %d: great %d <= meh %d", seed, great.LastResult.Total, meh.LastResult.Total)
		}
	}
}

func TestRunEndsAfterLastProject(t *testing.T) {
	run := NewRun(primitive.NewObjectID(), 12, "learner", domain.StartupModeFree, 7, t0)
	_ = PickFounder(run, 1)
	for i := 0; i < ProjectsPerRun; i++ {
		if err := StartProject(run, "Web App", "Education", nil, t0); err != nil {
			t.Fatal(err)
		}
		if err := Ship(run, run.Project.EndsAt); err != nil {
			t.Fatal(err)
		}
		if run.Stage == domain.StartupStageItem {
			if err := PickItem(run, 0); err != nil {
				t.Fatal(err)
			}
		}
	}
	wantScore := run.Money/10 + run.Fans + BossPassBonus*run.BossesPassed + BossWinBonus
	if run.Status != domain.StartupStatusEnded || run.Outcome != domain.StartupOutcomeIPO || run.BossesPassed != 3 || run.Score != wantScore {
		t.Fatalf("run not ended correctly: %+v", run)
	}
	if err := StartProject(run, "Web App", "Education", nil, t0); !errors.Is(err, domain.ErrStartupWrongStage) {
		t.Fatalf("expected wrong stage after end, got %v", err)
	}
}

func TestStartProjectRejectsBadChoices(t *testing.T) {
	run := NewRun(primitive.NewObjectID(), 12, "learner", domain.StartupModeFree, 7, t0)
	_ = PickFounder(run, 0)
	for _, c := range []struct {
		typ, theme string
		staff      []string
	}{
		{"Spaceship", "Education", nil},
		{"Web App", "Moon", nil},
		{"Web App", "Education", []string{"nobody"}},
		{"Web App", "Education", []string{"founder-0", "founder-0"}},
	} {
		if err := StartProject(run, c.typ, c.theme, c.staff, t0); !errors.Is(err, domain.ErrStartupInvalidChoice) {
			t.Fatalf("%+v: expected invalid choice, got %v", c, err)
		}
	}
}

type fakeStore struct {
	studio *domain.StartupStudio
	run    *domain.StartupRun
	all    []*domain.StartupRun

	boardRuns    []*domain.StartupRun
	boardStudios []*domain.StartupStudio
	boardNames   map[string]string
	boardRoles   map[string]string

	pool    []domain.StartupGenmate
	optOuts map[string]bool
}

func (f *fakeStore) FindStudio(context.Context, primitive.ObjectID) (*domain.StartupStudio, error) {
	return f.studio, nil
}
func (f *fakeStore) InsertStudio(_ context.Context, s domain.StartupStudio) error {
	f.studio = &s
	return nil
}
func (f *fakeStore) FindActiveRun(context.Context, primitive.ObjectID) (*domain.StartupRun, error) {
	if f.run == nil || f.run.Status != domain.StartupStatusActive {
		return nil, nil
	}
	cp := *f.run
	return &cp, nil
}
func (f *fakeStore) InsertRun(_ context.Context, run *domain.StartupRun) error {
	if f.run != nil && f.run.Status == domain.StartupStatusActive {
		return domain.ErrStartupRunActive
	}
	cp := *run
	f.run = &cp
	f.all = append(f.all, &cp)
	return nil
}
func (f *fakeStore) CountRankedRuns(_ context.Context, _ primitive.ObjectID, weekKey string) (int, error) {
	n := 0
	for _, r := range f.all {
		if r.Mode == domain.StartupModeRanked && r.WeekKey == weekKey {
			n++
		}
	}
	return n, nil
}
func (f *fakeStore) ListGenmates(_ context.Context, _ int, excludeID primitive.ObjectID) ([]domain.StartupGenmate, error) {
	out := []domain.StartupGenmate{}
	for _, g := range f.pool {
		if g.UserID == excludeID || f.optOuts[g.UserID.Hex()] {
			continue
		}
		out = append(out, g)
	}
	return out, nil
}
func (f *fakeStore) SetStartupOptOut(_ context.Context, userID primitive.ObjectID, optOut bool) error {
	if f.optOuts == nil {
		f.optOuts = map[string]bool{}
	}
	f.optOuts[userID.Hex()] = optOut
	return nil
}
func (f *fakeStore) FindStartupOptOut(_ context.Context, userID primitive.ObjectID) (bool, error) {
	return f.optOuts[userID.Hex()], nil
}
func (f *fakeStore) SaveRun(_ context.Context, run *domain.StartupRun, expected int) error {
	if f.run.Version != expected {
		return domain.ErrStartupRunConflict
	}
	cp := *run
	f.run = &cp
	return nil
}
func (f *fakeStore) AddDiscoveredCombo(_ context.Context, _ primitive.ObjectID, key string) error {
	if f.studio == nil {
		f.studio = &domain.StartupStudio{}
	}
	for _, k := range f.studio.DiscoveredCombos {
		if k == key {
			return nil
		}
	}
	f.studio.DiscoveredCombos = append(f.studio.DiscoveredCombos, key)
	return nil
}
func (f *fakeStore) SettleRun(_ context.Context, ownerID primitive.ObjectID, entry domain.StartupHallEntry, fameGain int, founders, items []string, skin string) error {
	if f.studio == nil {
		f.studio = &domain.StartupStudio{OwnerID: ownerID}
	}
	for _, h := range f.studio.HallOfFame {
		if h.RunID == entry.RunID {
			return nil
		}
	}
	f.studio.Fame += fameGain
	f.studio.HallOfFame = append(f.studio.HallOfFame, entry)
	if len(f.studio.HallOfFame) > 20 {
		f.studio.HallOfFame = f.studio.HallOfFame[len(f.studio.HallOfFame)-20:]
	}
	for _, title := range founders {
		found := false
		for _, t := range f.studio.UnlockedFounders {
			if t == title {
				found = true
			}
		}
		if !found {
			f.studio.UnlockedFounders = append(f.studio.UnlockedFounders, title)
		}
	}
	for _, id := range items {
		found := false
		for _, t := range f.studio.UnlockedItems {
			if t == id {
				found = true
			}
		}
		if !found {
			f.studio.UnlockedItems = append(f.studio.UnlockedItems, id)
		}
	}
	if skin != "" {
		f.studio.OfficeSkin = skin
	}
	return nil
}

func TestServiceGuardsShipAndConflicts(t *testing.T) {
	ctx := context.Background()
	store := &fakeStore{}
	clock := t0
	svc := &Service{store: store, now: func() time.Time { return clock }}
	player := Player{ID: primitive.NewObjectID().Hex(), Cohort: 12, Role: "learner"}

	if _, err := svc.StartRun(ctx, player, domain.StartupModeFree); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.StartRun(ctx, player, domain.StartupModeFree); !errors.Is(err, domain.ErrStartupRunActive) {
		t.Fatalf("expected one active run, got %v", err)
	}
	if _, err := svc.PickFounder(ctx, player, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.StartProject(ctx, player, "API/SaaS", "Fintech", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Ship(ctx, player); !errors.Is(err, domain.ErrStartupTooEarly) {
		t.Fatalf("expected too early, got %v", err)
	}

	clock = t0.Add(ProjectDuration * time.Second)
	stale, _ := store.FindActiveRun(ctx, primitive.NilObjectID)
	shipped, err := svc.Ship(ctx, player)
	if err != nil {
		t.Fatal(err)
	}
	if err := Ship(stale, clock); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveRun(ctx, stale, stale.Version); !errors.Is(err, domain.ErrStartupRunConflict) {
		t.Fatalf("expected double ship to conflict, got %v", err)
	}
	if shipped.LastResult == nil || shipped.Stage != domain.StartupStageItem {
		t.Fatalf("unexpected state after ship: %+v", shipped)
	}
	if len(shipped.ItemOffer) != CandidateOffers {
		t.Fatalf("expected %d item offers, got %+v", CandidateOffers, shipped.ItemOffer)
	}
	drafted, err := svc.PickItem(ctx, player, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(drafted.Items) != 1 || drafted.Stage != domain.StartupStageHub {
		t.Fatalf("unexpected state after draft: %+v", drafted)
	}
}

func TestHugeSeedStillEncodesToBSON(t *testing.T) {
	run := NewRun(primitive.NewObjectID(), 12, "learner", domain.StartupModeFree, math.MaxUint64-1, t0)
	if _, err := bson.Marshal(run); err != nil {
		t.Fatal(err)
	}
	again := NewRun(primitive.NewObjectID(), 12, "learner", domain.StartupModeFree, math.MaxUint64-1, t0)
	if !reflect.DeepEqual(run.FounderOffer, again.FounderOffer) {
		t.Fatal("huge seed is not deterministic")
	}
}
