package startupstory

import (
	"strings"

	"gofiber-baro/internal/domain"
)

const (
	RoleFE       = "fe_dev"
	RoleBE       = "be_dev"
	RoleDesigner = "designer"
	RoleQA       = "qa"
	RoleDevOps   = "devops"
	RolePO       = "po"
	RolePM       = "pm"
	RoleSA       = "sa"
)

type RoleInfo struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Job     string `json:"job"`
	Builder bool   `json:"builder"`
	Focus   int    `json:"-"`
}

var roles = []RoleInfo{
	{ID: RoleFE, Title: "Frontend Dev", Job: "Builds the screens. Full build power.", Builder: true, Focus: 0},
	{ID: RoleBE, Title: "Backend Dev", Job: "Builds APIs and data. Full build power.", Builder: true, Focus: 1},
	{ID: RoleDesigner, Title: "Designer", Job: "Designs the UX. Full build power, and Users love it.", Builder: true, Focus: 2},
	{ID: RoleQA, Title: "QA", Job: "Tests everything and catches bugs before ship.", Focus: 3},
	{ID: RoleDevOps, Title: "DevOps", Job: "Automates deploys for faster builds and saves the day in outages.", Focus: 3},
	{ID: RolePO, Title: "Product Owner", Job: "Writes user stories that fit the market. Users and Investors notice.", Focus: -1},
	{ID: RolePM, Title: "Project Manager", Job: "Runs standups and the timeline: faster builds, no big-team chaos.", Focus: -1},
	{ID: RoleSA, Title: "System Analyst", Job: "Designs the system up front: fewer bugs, and the Tech Lead approves.", Focus: -1},
}

const (
	SupportBuildShare  = 0.35
	CoordinationDrag   = 0.08
	MissingTestingBugs = 3
)

func roleInfo(id string) (RoleInfo, bool) {
	for _, r := range roles {
		if r.ID == id {
			return r, true
		}
	}
	return RoleInfo{}, false
}

func isBuilder(d domain.StartupDev) bool {
	if d.Wildcard == WildDuck {
		return false
	}
	if strings.HasPrefix(d.ID, "founder-") {
		return true
	}
	info, ok := roleInfo(d.Role)
	return !ok || info.Builder
}

func skillOf(d domain.StartupDev) float64 {
	s := stats(d)
	return float64(s[0]+s[1]+s[2]+s[3]) / 4
}

type teamJobs struct {
	bugCut      float64
	bugAdd      int
	durMult     float64
	hasPM       bool
	outageBoost float64
	reviewer    map[string]float64
}

func jobsFor(team []domain.StartupDev) teamJobs {
	best := map[string]domain.StartupDev{}
	for _, d := range team {
		if cur, ok := best[d.Role]; !ok || skillOf(d) > skillOf(cur) {
			best[d.Role] = d
		}
	}
	jobs := teamJobs{durMult: 1, reviewer: map[string]float64{}}
	if po, ok := best[RolePO]; ok {
		jobs.reviewer["Users"] += 0.3 * skillOf(po)
		jobs.reviewer["Investor"] += 0.3 * skillOf(po)
	}
	if pm, ok := best[RolePM]; ok {
		jobs.hasPM = true
		jobs.durMult *= max(0.7, 1-0.04*skillOf(pm))
	}
	if sa, ok := best[RoleSA]; ok {
		jobs.bugCut += 1 + 0.6*skillOf(sa)
		jobs.reviewer["Tech Lead"] += 0.25 * skillOf(sa)
	}
	if qa, ok := best[RoleQA]; ok {
		jobs.bugCut += 1 + 0.7*float64(stats(qa)[3])
	}
	if ops, ok := best[RoleDevOps]; ok {
		jobs.durMult *= 0.9
		jobs.bugCut++
		jobs.outageBoost = 0.08 * float64(stats(ops)[3])
	}
	if designer, ok := best[RoleDesigner]; ok {
		jobs.reviewer["Users"] += 0.15 * float64(stats(designer)[2])
	}
	_, hasQA := best[RoleQA]
	_, hasSA := best[RoleSA]
	if !hasQA && !hasSA {
		jobs.bugAdd = MissingTestingBugs
	}
	return jobs
}

func coordinationMult(teamSize int, hasPM bool) float64 {
	if hasPM || teamSize <= 2 {
		return 1
	}
	return 1 / (1 + CoordinationDrag*float64(teamSize-2))
}

func applyRoleFocus(s *[4]int, role string, max int) {
	info, ok := roleInfo(role)
	if !ok || info.Focus < 0 {
		return
	}
	s[info.Focus] = min(max+2, s[info.Focus]+2)
}
