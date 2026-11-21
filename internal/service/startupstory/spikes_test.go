package startupstory

import (
	"errors"
	"testing"

	"gofiber-baro/internal/domain"
)

func TestQueueShrinksEventSpikes(t *testing.T) {
	run := &domain.StartupRun{Fans: 10000, NextTraffic: 2, Infra: &domain.StartupInfra{Servers: []domain.StartupServer{{CPU: 1, RAM: 1}}, DB: "postgres"}}
	if app, _ := trafficLoad(run); app != 3000 {
		t.Fatalf("a 3x spike without a queue should be 3000 req/s, got %d", app)
	}
	run.Infra.Parts = []string{"queue"}
	if app, _ := trafficLoad(run); app != 1667 {
		t.Fatalf("a queue should cut the spike to a third (1667 req/s), got %d", app)
	}
}

func TestAutoscaleCoversUpToDoubleAndBillsUsage(t *testing.T) {
	run := &domain.StartupRun{Fans: 5000, Infra: &domain.StartupInfra{Servers: []domain.StartupServer{{CPU: 1, RAM: 1}}, DB: "postgres", Parts: []string{"autoscale"}}}
	out := infraEffects(run)
	if out.ratio != 1 || out.scaleBill != 2*AutoscalePer100Req {
		t.Fatalf("500 req/s on 300: autoscale should add exactly the 200 overflow for ฿300, got ratio %.2f bill %d", out.ratio, out.scaleBill)
	}
	run.Fans = 9000
	if out := infraEffects(run); out.ratio != 1.5 || out.scaleBill != 3*AutoscalePer100Req {
		t.Fatalf("autoscale caps at 2x capacity: ratio %.2f bill %d", out.ratio, out.scaleBill)
	}
}

func TestSpikePartsUnlockInAct3(t *testing.T) {
	run := newHubRun(t, 61)
	run.Money = 100000
	run.Act = 2
	if err := InfraAction(run, "part", 0, "queue"); !errors.Is(err, domain.ErrStartupInvalidChoice) {
		t.Fatalf("the queue unlocks in act 3, got %v", err)
	}
	run.Act = 3
	if err := InfraAction(run, "part", 0, "autoscale"); err != nil {
		t.Fatal(err)
	}
}
