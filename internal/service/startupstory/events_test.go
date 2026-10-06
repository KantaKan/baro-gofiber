package startupstory

import (
	"errors"
	"testing"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestWorldEventPoolResolvesInCatalog(t *testing.T) {
	if len(worldEventPool) < 6 {
		t.Fatalf("world event pool needs at least 6 entries, got %d", len(worldEventPool))
	}
	for _, id := range worldEventPool {
		w, ok := findWorldEvent(id)
		if !ok {
			t.Fatalf("world event %q not in catalog", id)
		}
		if w.Name == "" || w.Desc == "" {
			t.Fatalf("world event %q needs a name and desc for the HUD", id)
		}
	}
}

func TestChoiceEventPoolResolvesInCatalog(t *testing.T) {
	if len(choiceEventPool) < 10 {
		t.Fatalf("choice event pool needs at least 10 entries, got %d", len(choiceEventPool))
	}
	for _, id := range choiceEventPool {
		ev, ok := findChoiceEvent(id)
		if !ok {
			t.Fatalf("choice event %q not in catalog", id)
		}
		if ev.Title == "" {
			t.Fatalf("choice event %q needs a title", id)
		}
		for i, opt := range ev.Options {
			if opt.Label == "" {
				t.Fatalf("choice event %q option %d needs a label", id, i)
			}
		}
	}
}

func TestNewActRollsWorldEventDeterministically(t *testing.T) {
	roll := func() string {
		run := NewRun(primitive.NewObjectID(), 12, "learner", domain.StartupModeFree, 42, t0)
		if err := PickFounder(run, 0); err != nil {
			t.Fatal(err)
		}
		return run.WorldEvent
	}
	a, b := roll(), roll()
	if a == "" || a != b {
		t.Fatalf("same seed must give the same world event: %q %q", a, b)
	}
	inPool := false
	for _, id := range worldEventPool {
		if id == a {
			inPool = true
		}
	}
	if !inPool {
		t.Fatalf("world event %q rolled outside the pool", a)
	}
}

func TestWorldDurationAndHireCost(t *testing.T) {
	plain := &domain.StartupRun{}
	if worldDurationMult(plain) != 1 {
		t.Fatalf("no world event means full duration, got %v", worldDurationMult(plain))
	}
	if hireCost(plain, 1000) != 1000 {
		t.Fatalf("no world event means full salary, got %d", hireCost(plain, 1000))
	}
	rainy := &domain.StartupRun{WorldEvent: "rainy-season"}
	if worldDurationMult(rainy) != 1.3 {
		t.Fatalf("rainy season should slow builds to 1.3x, got %v", worldDurationMult(rainy))
	}
	layoff := &domain.StartupRun{WorldEvent: "layoff-season"}
	if hireCost(layoff, 1000) != 600 {
		t.Fatalf("layoff season should cut hiring to 60%%, got %d", hireCost(layoff, 1000))
	}
	if hireCost(layoff, 1001) != 601 {
		t.Fatalf("hire cost should round, got %d", hireCost(layoff, 1001))
	}
}

func TestApplyWorldScoring(t *testing.T) {
	newSc := func(ev string, project *domain.StartupProject) *scoring {
		return &scoring{
			run:       &domain.StartupRun{WorldEvent: ev, Project: project},
			weights:   [4]float64{1, 1, 1, 1},
			powerMult: 1,
			reviewer:  map[string]float64{},
			moneyMult: 1,
			fansMult:  1,
		}
	}

	song := newSc("songkran", nil)
	applyWorldScoring(song)
	if song.powerMult != 0.75 {
		t.Fatalf("songkran should cut power to 0.75, got %v", song.powerMult)
	}

	ai := newSc("ai-hype", &domain.StartupProject{Type: "AI Chatbot"})
	applyWorldScoring(ai)
	if ai.powerMult != 1.5 {
		t.Fatalf("ai hype should give an AI Chatbot +50%% power, got %v", ai.powerMult)
	}

	web := newSc("ai-hype", &domain.StartupProject{Type: "Web App"})
	applyWorldScoring(web)
	if web.powerMult != 1 {
		t.Fatalf("ai hype should not touch other products, got %v", web.powerMult)
	}

	crypto := newSc("crypto-winter", &domain.StartupProject{Theme: "Crypto"})
	applyWorldScoring(crypto)
	if crypto.moneyMult != 0.6 || crypto.fansMult != 0.6 {
		t.Fatalf("crypto winter should shrink money and fans to 0.6, got %v %v", crypto.moneyMult, crypto.fansMult)
	}

	hack := newSc("hackathon", nil)
	applyWorldScoring(hack)
	if hack.bugs != 2 {
		t.Fatalf("hackathon should add 2 bugs, got %d", hack.bugs)
	}

	sponsor := newSc("sponsor-week", nil)
	applyWorldScoring(sponsor)
	if sponsor.moneyMult != 1.25 {
		t.Fatalf("sponsor week should add 25%% money, got %v", sponsor.moneyMult)
	}

	none := newSc("", nil)
	applyWorldScoring(none)
	if none.powerMult != 1 || none.bugs != 0 || none.moneyMult != 1 || none.fansMult != 1 {
		t.Fatalf("no world event must leave scoring untouched: %+v", none)
	}
}

func TestQueueEventStoresEventBeforeShowing(t *testing.T) {
	var run *domain.StartupRun
	for seed := uint64(1); seed <= 200 && run == nil; seed++ {
		r := NewRun(primitive.NewObjectID(), 12, "learner", domain.StartupModeFree, seed, t0)
		if err := PickFounder(r, 0); err != nil {
			t.Fatal(err)
		}
		queueEvent(r)
		if r.PendingEvent != nil {
			run = r
		}
	}
	if run == nil {
		t.Fatal("no seed queued an event in 200 tries at 35% chance")
	}
	if run.Stage != domain.StartupStageEvent || run.ResumeStage != domain.StartupStageHub {
		t.Fatalf("event must interrupt into the event stage, got stage=%s resume=%s", run.Stage, run.ResumeStage)
	}
	if len(run.PendingEvent.Options) != 2 {
		t.Fatalf("choice events need exactly 2 options, got %d", len(run.PendingEvent.Options))
	}
	ev, ok := findChoiceEvent(run.PendingEvent.ID)
	if !ok {
		t.Fatalf("pending event %q not in catalog", run.PendingEvent.ID)
	}
	if run.PendingEvent.Options[0] != ev.Options[0].Label || run.PendingEvent.Options[1] != ev.Options[1].Label {
		t.Fatalf("options must be stored as labels for the FE: %+v", run.PendingEvent.Options)
	}
	before := run.PendingEvent.ID
	queueEvent(run)
	if run.PendingEvent.ID != before {
		t.Fatalf("a refresh must not reroll the pending event: %q -> %q", before, run.PendingEvent.ID)
	}
}

func TestQueueEventSkipsWhenNotEligible(t *testing.T) {
	interrupted := NewRun(primitive.NewObjectID(), 12, "learner", domain.StartupModeFree, 7, t0)
	if err := PickFounder(interrupted, 0); err != nil {
		t.Fatal(err)
	}
	interrupted.ResumeStage = domain.StartupStageItem
	queueEvent(interrupted)
	if interrupted.PendingEvent != nil {
		t.Fatal("a perk or item interrupt must block the event roll")
	}

	pending := NewRun(primitive.NewObjectID(), 12, "learner", domain.StartupModeFree, 8, t0)
	if err := PickFounder(pending, 0); err != nil {
		t.Fatal(err)
	}
	pending.PendingEvent = &domain.StartupPendingEvent{ID: "grant", Options: []string{"a", "b"}}
	queueEvent(pending)
	if pending.PendingEvent.ID != "grant" {
		t.Fatalf("an existing pending event must be kept, got %q", pending.PendingEvent.ID)
	}

	ended := NewRun(primitive.NewObjectID(), 12, "learner", domain.StartupModeFree, 9, t0)
	if err := PickFounder(ended, 0); err != nil {
		t.Fatal(err)
	}
	ended.Status = domain.StartupStatusEnded
	queueEvent(ended)
	if ended.PendingEvent != nil {
		t.Fatal("an ended run must not queue events")
	}
}

func TestPickEventAppliesEffectAndResumes(t *testing.T) {
	setup := func(seed uint64) *domain.StartupRun {
		run := NewRun(primitive.NewObjectID(), 12, "learner", domain.StartupModeFree, seed, t0)
		if err := PickFounder(run, 0); err != nil {
			t.Fatal(err)
		}
		run.Staff[0].Burnout = 95
		run.Stage = domain.StartupStageEvent
		run.ResumeStage = domain.StartupStageHub
		run.PendingEvent = &domain.StartupPendingEvent{
			ID:      "friday-deploy",
			Options: []string{"Do it. YOLO 😈", "Wait for Monday"},
		}
		return run
	}

	yolo := setup(11)
	baseMoney := yolo.Money
	if err := PickEvent(yolo, 0); err != nil {
		t.Fatal(err)
	}
	if yolo.Money != baseMoney+3000 {
		t.Fatalf("friday deploy should pay 3000, got %d", yolo.Money-baseMoney)
	}
	if yolo.Staff[0].Burnout != 100 {
		t.Fatalf("burnout should clamp at 100, got %d", yolo.Staff[0].Burnout)
	}
	if yolo.PendingEvent != nil || yolo.Stage != domain.StartupStageHub || yolo.ResumeStage != "" {
		t.Fatalf("resolving must clear the event and resume the hub: stage=%s resume=%s pending=%+v", yolo.Stage, yolo.ResumeStage, yolo.PendingEvent)
	}

	monday := setup(11)
	monday.Staff[0].Burnout = 50
	baseMoney = monday.Money
	if err := PickEvent(monday, 1); err != nil {
		t.Fatal(err)
	}
	if monday.Money != baseMoney || monday.Staff[0].Burnout != 42 {
		t.Fatalf("wait for monday should calm burnout by 8 without money: money=%d burnout=%d", monday.Money-baseMoney, monday.Staff[0].Burnout)
	}
}

func TestPickEventRejectsWrongStageOrIndex(t *testing.T) {
	run := NewRun(primitive.NewObjectID(), 12, "learner", domain.StartupModeFree, 7, t0)
	if err := PickFounder(run, 0); err != nil {
		t.Fatal(err)
	}
	if err := PickEvent(run, 0); !errors.Is(err, domain.ErrStartupWrongStage) {
		t.Fatalf("hub stage must reject the pick, got %v", err)
	}
	run.Stage = domain.StartupStageEvent
	if err := PickEvent(run, 0); !errors.Is(err, domain.ErrStartupWrongStage) {
		t.Fatalf("a missing pending event must reject the pick, got %v", err)
	}
	run.PendingEvent = &domain.StartupPendingEvent{ID: "grant", Options: []string{"a", "b"}}
	for _, idx := range []int{-1, 2} {
		if err := PickEvent(run, idx); !errors.Is(err, domain.ErrStartupInvalidChoice) {
			t.Fatalf("index %d must be an invalid choice, got %v", idx, err)
		}
	}
}

func TestEventEffectsClamp(t *testing.T) {
	run := &domain.StartupRun{
		Money: 100,
		Fans:  50,
		Staff: []domain.StartupDev{{Burnout: 3}, {Burnout: 98}},
	}
	applyEventEffect(run, eventEffect{Money: -500, Fans: -300, Burnout: 15})
	if run.Money != -400 {
		t.Fatalf("money is allowed to go negative, got %d", run.Money)
	}
	if run.Fans != 0 {
		t.Fatalf("fans must clamp at 0, got %d", run.Fans)
	}
	if run.Staff[0].Burnout != 18 || run.Staff[1].Burnout != 100 {
		t.Fatalf("burnout must clamp to 0..100, got %d %d", run.Staff[0].Burnout, run.Staff[1].Burnout)
	}
	applyEventEffect(run, eventEffect{Burnout: -50})
	if run.Staff[0].Burnout != 0 || run.Staff[1].Burnout != 50 {
		t.Fatalf("negative burnout must clamp at 0, got %d %d", run.Staff[0].Burnout, run.Staff[1].Burnout)
	}
}
