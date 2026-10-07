package startupstory

import (
	"context"
	"testing"
	"time"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestFullWeeklySeedJourney(t *testing.T) {
	ctx := context.Background()
	pool := []domain.StartupGenmate{}
	for _, name := range []string{"Ploy", "Nat", "Kan"} {
		pool = append(pool, domain.StartupGenmate{UserID: primitive.NewObjectID(), Name: name})
	}
	store := &fakeStore{pool: pool}
	svc := &Service{store: store, now: func() time.Time { return t0 }}
	player := Player{ID: primitive.NewObjectID().Hex(), Cohort: 12, Role: "learner"}

	if _, err := svc.Overview(ctx, player); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.StartRun(ctx, player, domain.StartupModeRanked); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.PickFounder(ctx, player, 0); err != nil {
		t.Fatal(err)
	}

	ships := 0
	for {
		active, err := store.FindActiveRun(ctx, primitive.NilObjectID)
		if err != nil {
			t.Fatal(err)
		}
		if active == nil {
			break
		}
		switch active.Stage {
		case domain.StartupStageHub:
			if len(active.Candidates) > 0 && len(active.Staff) < len(active.Desks) && active.Money >= active.Candidates[0].Salary {
				if _, err := svc.Hire(ctx, player, active.Candidates[0].ID); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := svc.StartProject(ctx, player, "Web App", "Education", nil); err != nil {
				t.Fatalf("ship %d start: %v", ships, err)
			}
		case domain.StartupStageItem:
			if _, err := svc.PickItem(ctx, player, 0); err != nil {
				t.Fatal(err)
			}
			continue
		case domain.StartupStagePerk:
			if _, err := svc.PickPerk(ctx, player, 0); err != nil {
				t.Fatal(err)
			}
			continue
		case domain.StartupStageEvent:
			if _, err := svc.PickEvent(ctx, player, 0); err != nil {
				t.Fatal(err)
			}
			continue
		default:
			t.Fatalf("unexpected stage %s", active.Stage)
		}
		store.run.Project.EndsAt = t0.Add(-time.Second)
		if _, err := svc.Ship(ctx, player); err != nil {
			t.Fatalf("ship %d: %v", ships, err)
		}
		ships++
		if ships > ProjectsPerRun+2 {
			t.Fatal("run should end after 9 ships")
		}
	}

	studio := store.studio
	if studio == nil || len(studio.HallOfFame) != 1 {
		t.Fatalf("run should settle exactly once: %+v", studio)
	}
	final := studio.HallOfFame[0]
	if final.Outcome != domain.StartupOutcomeIPO && final.Outcome != domain.StartupOutcomePivot {
		t.Fatalf("warm ending required, got %q", final.Outcome)
	}
	if studio.Fame < 1 || len(studio.DiscoveredCombos) == 0 {
		t.Fatalf("settle should grant fame + combos: %+v", studio)
	}
	t.Logf("journey: %d ships, outcome %s, score %d, fame %d", ships, final.Outcome, final.Score, studio.Fame)
}
