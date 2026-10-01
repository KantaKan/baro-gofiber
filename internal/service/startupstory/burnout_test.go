package startupstory

import (
	"strings"
	"testing"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func burnoutRun(staff ...domain.StartupDev) *domain.StartupRun {
	return &domain.StartupRun{OwnerID: primitive.NewObjectID(), Seed: 9, Act: 1, ProjectIndex: 1, Status: domain.StartupStatusActive, Stage: domain.StartupStageDeveloping, Staff: staff}
}

func person(id, role string, burnout int) domain.StartupDev {
	return domain.StartupDev{ID: id, Name: id, Role: role, Burnout: burnout, Frontend: 3, Backend: 3, Design: 3, Debug: 3}
}

func TestWorkBurnsAndRestHeals(t *testing.T) {
	worker, rester := person("cand-1", RoleFE, 10), person("cand-2", RoleBE, 40)
	run := burnoutRun(worker, rester)
	afterShipBurnout(run, []domain.StartupDev{worker}, t0)
	if run.Staff[0].Burnout <= 10 {
		t.Fatalf("working should add burnout, got %d", run.Staff[0].Burnout)
	}
	if run.Staff[1].Burnout != 40-BurnoutRest {
		t.Fatalf("sitting out should heal %d, got %d", BurnoutRest, run.Staff[1].Burnout)
	}
}

func TestPMAndRetreatSoftenBurnout(t *testing.T) {
	dev := person("cand-1", RoleFE, 0)
	solo := burnoutRun(dev)
	afterShipBurnout(solo, []domain.StartupDev{dev}, t0)
	pm := person("cand-2", RolePM, 0)
	managed := burnoutRun(dev, pm)
	afterShipBurnout(managed, []domain.StartupDev{dev, pm}, t0)
	if managed.Staff[0].Burnout >= solo.Staff[0].Burnout {
		t.Fatalf("a PM should slow burnout: %d vs %d", managed.Staff[0].Burnout, solo.Staff[0].Burnout)
	}
	funded := burnoutRun(dev)
	funded.Items = []string{"team-retreat"}
	afterShipBurnout(funded, []domain.StartupDev{dev}, t0)
	if funded.Staff[0].Burnout >= solo.Staff[0].Burnout {
		t.Fatal("the Team Retreat Fund item should slow burnout")
	}
	crunched := burnoutRun(dev)
	crunched.Items = []string{"crunch-culture"}
	afterShipBurnout(crunched, []domain.StartupDev{dev}, t0)
	if crunched.Staff[0].Burnout <= solo.Staff[0].Burnout {
		t.Fatal("Crunch Culture should burn people out faster")
	}
}

func TestBurnedOutPeopleQuitWithAMessage(t *testing.T) {
	quitter, genmate, fine := person("cand-1", RoleFE, 99), person("cand-2", RoleQA, 99), person("cand-3", RoleBE, 0)
	genmate.GenmateID = "u42"
	run := burnoutRun(quitter, genmate, fine)
	if afterShipBurnout(run, []domain.StartupDev{quitter, genmate}, t0) {
		t.Fatal("a staff quit should not end the run")
	}
	if len(run.Staff) != 1 || run.Staff[0].ID != "cand-3" {
		t.Fatalf("burned-out people should leave: %+v", run.Staff)
	}
	if len(run.Log) != 2 || !strings.Contains(run.Log[0], "cand-1") {
		t.Fatalf("each quit should leave a message: %v", run.Log)
	}
	gentle := false
	for _, g := range gentleBreaks {
		if run.Log[1] == strings.ReplaceAll(g, "{name}", "cand-2") {
			gentle = true
		}
	}
	if !gentle {
		t.Fatalf("real genmates only ever get a gentle message, got %q", run.Log[1])
	}
}

func TestFounderBurnoutEndsTheRun(t *testing.T) {
	founder := person("founder-0", RolePM, 99)
	run := burnoutRun(founder, person("cand-1", RoleFE, 0))
	if !afterShipBurnout(run, []domain.StartupDev{founder}, t0) {
		t.Fatal("founder burnout should end the run")
	}
	if run.Status != domain.StartupStatusEnded || run.Outcome != domain.StartupOutcomePivot || len(run.Log) != 1 {
		t.Fatalf("founder burnout should pivot with a message: %+v", run)
	}
}

func TestSkippingAnItemBuysATeamRetreat(t *testing.T) {
	run := shipToDraft(t, 33)
	run.Staff[0].Burnout = 60
	before := len(run.Items)
	if err := PickItem(run, SkipItemForRetreat); err != nil {
		t.Fatal(err)
	}
	if run.Staff[0].Burnout != 60-BurnoutRetreat || len(run.Items) != before || run.Stage != domain.StartupStageHub {
		t.Fatalf("skip should heal everyone, add no item and return to the hub: %+v", run)
	}
}
