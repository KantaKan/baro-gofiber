package startupstory

import (
	"strings"
	"testing"

	"gofiber-baro/internal/domain"
)

func TestEveryPooledEventExists(t *testing.T) {
	for _, id := range choiceEventPool {
		if _, ok := findChoiceEvent(id); !ok {
			t.Errorf("event %q is in the pool but not in a catalog", id)
		}
	}
}

func eventRun(t *testing.T, seed uint64, id string) *domain.StartupRun {
	t.Helper()
	run := newHubRun(t, seed)
	run.PendingEvent = &domain.StartupPendingEvent{ID: id, Options: []string{"a", "b"}}
	interrupt(run, domain.StartupStageEvent)
	return run
}

func TestGambleHitsAboutAsOftenAsItSays(t *testing.T) {
	hits := 0
	for seed := uint64(1); seed <= 400; seed++ {
		run := eventRun(t, seed, "leaked-env")
		before := run.Money
		if err := PickEvent(run, 1); err != nil {
			t.Fatal(err)
		}
		if run.Money == before-3000 {
			hits++
		}
		if !strings.Contains(run.Log[len(run.Log)-1], ".env") {
			t.Fatalf("gamble should log its outcome, got %q", run.Log[len(run.Log)-1])
		}
	}
	if hits < 80 || hits > 160 {
		t.Fatalf("a 30%% gamble hit %d/400 times", hits)
	}
}

func TestGambleIsDeterministicPerSeed(t *testing.T) {
	a, b := eventRun(t, 77, "copilot-api"), eventRun(t, 77, "copilot-api")
	_ = PickEvent(a, 1)
	_ = PickEvent(b, 1)
	if a.NextBugs != b.NextBugs {
		t.Fatalf("same seed should roll the same: %d vs %d", a.NextBugs, b.NextBugs)
	}
}

func TestNextProjectEffectsApplyOnceThenClear(t *testing.T) {
	bugs := func(next int) (int, *domain.StartupRun) {
		run := newHubRun(t, 41)
		run.NextBugs = next
		if err := StartProject(run, "Web App", "Education", nil, t0); err != nil {
			t.Fatal(err)
		}
		if err := Ship(run, run.Project.EndsAt); err != nil {
			t.Fatal(err)
		}
		return run.LastResult.Bugs, run
	}
	plain, _ := bugs(0)
	cursed, run := bugs(4)
	if cursed != plain+4 {
		t.Fatalf("+4 next-project bugs should land on the ship: %d vs %d", cursed, plain)
	}
	if run.NextBugs != 0 || run.NextPower != 0 {
		t.Fatalf("next-project effects should clear after one ship: %d %v", run.NextBugs, run.NextPower)
	}
}

func TestEventsDoNotRepeatUntilThePoolIsUsedUp(t *testing.T) {
	run := &domain.StartupRun{SeenEvents: choiceEventPool[1:]}
	if got := unseenEvents(run); len(got) != 1 || got[0] != choiceEventPool[0] {
		t.Fatalf("only the unseen event should remain, got %v", got)
	}
	run.SeenEvents = append([]string{}, choiceEventPool...)
	if got := unseenEvents(run); len(got) != len(choiceEventPool) || run.SeenEvents != nil {
		t.Fatalf("a used-up pool should reset, got %d left, seen %v", len(got), run.SeenEvents)
	}
}

func TestSideQuestOncePerActForDistractedStaff(t *testing.T) {
	for seed := uint64(1); seed <= 200; seed++ {
		run := newHubRun(t, seed)
		run.Staff[0].Burnout = 90
		before := stats(run.Staff[0])
		sideQuests(run, run.Staff)
		if run.Staff[0].QuestAct == 0 {
			continue
		}
		after := stats(run.Staff[0])
		gained := after[0] + after[1] + after[2] + after[3] - before[0] - before[1] - before[2] - before[3]
		if gained != 1 {
			t.Fatalf("a side quest gives exactly +1, got %d", gained)
		}
		for range 50 {
			sideQuests(run, run.Staff)
		}
		if again := stats(run.Staff[0]); again != after {
			t.Fatal("a dev should side-quest at most once per act")
		}
		return
	}
	t.Fatal("no side quest in 200 seeds at 10%")
}

func TestRestedStaffDoNotSideQuest(t *testing.T) {
	for seed := uint64(1); seed <= 100; seed++ {
		run := newHubRun(t, seed)
		run.Staff[0].Burnout = DistractedAt - 1
		sideQuests(run, run.Staff)
		if run.Staff[0].QuestAct != 0 {
			t.Fatal("only distracted staff go on side quests")
		}
	}
}
