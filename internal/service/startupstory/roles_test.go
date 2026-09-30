package startupstory

import (
	"testing"

	"gofiber-baro/internal/domain"
)

func dev(id, role string, level int) domain.StartupDev {
	return domain.StartupDev{ID: id, Role: role, Frontend: level, Backend: level, Design: level, Debug: level}
}

func TestSupportRolesDoTheirJobs(t *testing.T) {
	builders := []domain.StartupDev{dev("a", RoleFE, 4), dev("b", RoleBE, 4)}
	bare := jobsFor(builders)
	if bare.bugAdd != MissingTestingBugs {
		t.Fatalf("no QA or SA should add %d bugs, got %d", MissingTestingBugs, bare.bugAdd)
	}
	withQA := jobsFor(append(builders, dev("q", RoleQA, 4)))
	if withQA.bugAdd != 0 || withQA.bugCut <= 0 {
		t.Fatalf("QA should catch bugs: %+v", withQA)
	}
	withSA := jobsFor(append(builders, dev("s", RoleSA, 4)))
	if withSA.bugCut <= 0 || withSA.reviewer["Tech Lead"] <= 0 {
		t.Fatalf("SA should cut bugs and please the Tech Lead: %+v", withSA)
	}
	withPO := jobsFor(append(builders, dev("p", RolePO, 4)))
	if withPO.reviewer["Users"] <= 0 || withPO.reviewer["Investor"] <= 0 {
		t.Fatalf("PO should win over Users and Investors: %+v", withPO)
	}
	withPM := jobsFor(append(builders, dev("m", RolePM, 4)))
	if !withPM.hasPM || withPM.durMult >= 1 {
		t.Fatalf("PM should speed up the build: %+v", withPM)
	}
	withOps := jobsFor(append(builders, dev("o", RoleDevOps, 4)))
	if withOps.durMult >= 1 || withOps.outageBoost <= 0 {
		t.Fatalf("DevOps should speed deploys and help in outages: %+v", withOps)
	}
}

func TestOnlyTheBestOfEachRoleWorks(t *testing.T) {
	one := jobsFor([]domain.StartupDev{dev("q1", RoleQA, 5)})
	two := jobsFor([]domain.StartupDev{dev("q1", RoleQA, 5), dev("q2", RoleQA, 2)})
	if one.bugCut != two.bugCut {
		t.Fatalf("a second QA should not stack the QA job: %v vs %v", one.bugCut, two.bugCut)
	}
}

func TestBigTeamsNeedAPM(t *testing.T) {
	if coordinationMult(2, false) != 1 || coordinationMult(6, true) != 1 {
		t.Fatal("small teams and teams with a PM have no coordination drag")
	}
	if coordinationMult(6, false) >= coordinationMult(4, false) {
		t.Fatal("drag should grow with team size")
	}
}

func TestFoundersAlwaysBuildButSupportHiresMostlyDont(t *testing.T) {
	if !isBuilder(domain.StartupDev{ID: "founder-0", Role: RolePM}) {
		t.Fatal("founders build at full share whatever their role")
	}
	if isBuilder(domain.StartupDev{ID: "cand-1-0", Role: RolePM}) {
		t.Fatal("a hired PM is support, not a builder")
	}
	if !isBuilder(domain.StartupDev{ID: "legacy"}) {
		t.Fatal("devs without a role (older runs) keep building")
	}
}
