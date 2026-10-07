package startupstory

import (
	"errors"
	"strings"
	"testing"

	"gofiber-baro/internal/domain"
)

func TestServerCapacityIsItsWeakestPart(t *testing.T) {
	if got := serverCapacity(domain.StartupServer{CPU: 3, RAM: 1}); got != tierCapacity[1] {
		t.Fatalf("a big CPU with tier-1 RAM should run at tier 1, got %d", got)
	}
}

func TestSecondServerNeedsALoadBalancer(t *testing.T) {
	inf := &domain.StartupInfra{Servers: []domain.StartupServer{{CPU: 1, RAM: 1}, {CPU: 1, RAM: 1}}, DB: "sqlite"}
	if appCapacity(inf) != 300 {
		t.Fatalf("without a load balancer only one server works, got %d", appCapacity(inf))
	}
	inf.Parts = []string{"lb"}
	if appCapacity(inf) != 600 {
		t.Fatalf("with a load balancer both servers work, got %d", appCapacity(inf))
	}
}

func TestInfraActions(t *testing.T) {
	run := newHubRun(t, 51)
	run.Money = 100000
	if err := InfraAction(run, "server", 0, ""); err != nil {
		t.Fatal(err)
	}
	if NextServerPrice(run) != 3000 {
		t.Fatalf("the 3rd server should cost 1.5x, got %d", NextServerPrice(run))
	}
	for range 2 {
		if err := InfraAction(run, "ram", 1, ""); err != nil {
			t.Fatal(err)
		}
	}
	if err := InfraAction(run, "ram", 1, ""); !errors.Is(err, domain.ErrStartupInvalidChoice) {
		t.Fatalf("tier 3 is the max, got %v", err)
	}
	if err := InfraAction(run, "part", 0, "index"); err != nil {
		t.Fatal(err)
	}
	if err := InfraAction(run, "part", 0, "index"); !errors.Is(err, domain.ErrStartupInvalidChoice) {
		t.Fatalf("buying a part twice should fail, got %v", err)
	}
	before := run.Money
	if err := InfraAction(run, "db", 0, "postgres"); err != nil {
		t.Fatal(err)
	}
	if run.Infra.DB != "postgres" || before-run.Money != MigrationPrice || run.NextBugs != MigrationBugs {
		t.Fatalf("migration should switch DB, cost %d and add %d bugs: %+v", MigrationPrice, MigrationBugs, run.Infra)
	}
	if err := InfraAction(run, "db", 0, "oracle"); !errors.Is(err, domain.ErrStartupInvalidChoice) {
		t.Fatalf("unknown DB should fail, got %v", err)
	}
	run.Money = 0
	if err := InfraAction(run, "server", 0, ""); !errors.Is(err, domain.ErrStartupNoFunds) {
		t.Fatalf("expected no funds, got %v", err)
	}
}

func TestCloudBillAndDevOpsCut(t *testing.T) {
	run := &domain.StartupRun{Infra: &domain.StartupInfra{Servers: []domain.StartupServer{{CPU: 2, RAM: 2}}, DB: "postgres", Parts: []string{"lb"}}}
	if got := cloudBill(run); got != 200+300+200 {
		t.Fatalf("bill should be server 200 + postgres 300 + lb 200, got %d", got)
	}
	run.Staff = []domain.StartupDev{{Role: RoleDevOps}}
	if got := cloudBill(run); got != 525 {
		t.Fatalf("DevOps should cut the bill by 25%%, got %d", got)
	}
}

func TestOverloadExplainsTheBottleneck(t *testing.T) {
	run := &domain.StartupRun{Fans: 9000, Infra: &domain.StartupInfra{Servers: []domain.StartupServer{{CPU: 3, RAM: 1}}, DB: "postgres", Parts: []string{"index"}}}
	out := infraEffects(run)
	if out.ratio != 3 || out.bugs != 20-1 || !strings.Contains(out.postmortem, "RAM was the bottleneck") {
		t.Fatalf("900 req/s on a 300 req/s server: ratio %.2f bugs %d %q", out.ratio, out.bugs, out.postmortem)
	}
	run.Infra.Servers = append(run.Infra.Servers, domain.StartupServer{CPU: 3, RAM: 3})
	if out := infraEffects(run); !strings.Contains(out.postmortem, "load balancer") {
		t.Fatalf("an idle 2nd server should point at the load balancer, got %q", out.postmortem)
	}
	run.Infra = &domain.StartupInfra{Servers: []domain.StartupServer{{CPU: 3, RAM: 3}}, DB: "sqlite"}
	if out := infraEffects(run); !strings.Contains(out.postmortem, "indexes") {
		t.Fatalf("an unindexed DB should point at indexes, got %q", out.postmortem)
	}
}

func TestOldRunsSkipInfra(t *testing.T) {
	run := &domain.StartupRun{Fans: 50000}
	if out := infraEffects(run); out.ratio != 0 || out.bugs != 0 || cloudBill(run) != 0 {
		t.Fatalf("a run saved before infra should not overload or pay a bill: %+v", out)
	}
}

func TestBackupsNeutraliseTheInternEvent(t *testing.T) {
	run := eventRun(t, 52, "intern-db")
	run.Infra.Parts = []string{"backups"}
	before := run.Money
	if err := PickEvent(run, 0); err != nil {
		t.Fatal(err)
	}
	if run.Money != before {
		t.Fatalf("backups should make the restore free, lost %d", before-run.Money)
	}
}

func TestReplicaRules(t *testing.T) {
	run := newHubRun(t, 53)
	run.Money = 100000
	run.Act = 2
	if err := InfraAction(run, "replica", 0, ""); !errors.Is(err, domain.ErrStartupInvalidChoice) {
		t.Fatalf("the single-file database can't have replicas, got %v", err)
	}
	run.Infra.DB = "mariadb"
	if NextReplicaPrice(run) != 1300 {
		t.Fatalf("lightweight relational replicas are half price (1250, rounded to 100), got %d", NextReplicaPrice(run))
	}
	if err := InfraAction(run, "replica", 0, ""); err != nil {
		t.Fatal(err)
	}
	if got := dbCapacity(run.Infra); got != 1200+720 {
		t.Fatalf("one replica adds 60%%, got %d", got)
	}
	if err := InfraAction(run, "db", 0, "sqlite"); !errors.Is(err, domain.ErrStartupInvalidChoice) {
		t.Fatalf("can't go back to a single file with replicas, got %v", err)
	}
	run.Act = 1
	if err := InfraAction(run, "replica", 0, ""); !errors.Is(err, domain.ErrStartupInvalidChoice) {
		t.Fatalf("replicas unlock in act 2, got %v", err)
	}
}

func TestSpeedPartsCutLoad(t *testing.T) {
	run := &domain.StartupRun{Fans: 10000, Infra: &domain.StartupInfra{Servers: []domain.StartupServer{{CPU: 1, RAM: 1}}, DB: "postgres", Parts: []string{"cdn", "index", "cache"}}}
	app, db := trafficLoad(run)
	if app != 700 || db != 420 {
		t.Fatalf("CDN -30%% app, index+cache -30%% and -40%% DB: got app %d db %d", app, db)
	}
	if out := infraEffects(run); out.usersBias != CDNUsersBias {
		t.Fatalf("a CDN should please the Users reviewer, got %v", out.usersBias)
	}
}

func TestContainersAndCICD(t *testing.T) {
	run := &domain.StartupRun{Infra: &domain.StartupInfra{Servers: []domain.StartupServer{{CPU: 1, RAM: 1}, {CPU: 1, RAM: 1}}, DB: "sqlite"}}
	plain := NextServerPrice(run)
	run.Infra.Parts = []string{"containers", "cicd"}
	if got := NextServerPrice(run); got != plain*7/10 {
		t.Fatalf("containers make servers 30%% cheaper: %d vs %d", got, plain)
	}
	if out := infraEffects(run); out.bugs != -2 {
		t.Fatalf("containers and CI/CD should each cut a bug, got %d", out.bugs)
	}
}

func TestDBPostmortemPointsAtCacheThenReplicas(t *testing.T) {
	run := &domain.StartupRun{Fans: 40000, Infra: &domain.StartupInfra{Servers: []domain.StartupServer{{CPU: 3, RAM: 3}}, DB: "postgres", Parts: []string{"index", "lb"}}}
	run.Infra.Servers = append(run.Infra.Servers, run.Infra.Servers[0], run.Infra.Servers[0], run.Infra.Servers[0])
	if out := infraEffects(run); !strings.Contains(out.postmortem, "cache") {
		t.Fatalf("expected a cache hint, got %q", out.postmortem)
	}
	run.Infra.Parts = append(run.Infra.Parts, "cache")
	run.Fans = 80000
	if out := infraEffects(run); !strings.Contains(out.postmortem, "replica") {
		t.Fatalf("expected a replica hint, got %q", out.postmortem)
	}
}
