// Package staff holds the admin roles (Owner, Admin, TA, Viewer) and what each may do.
package staff

import (
	"strconv"
	"strings"
)

const (
	Owner  = "owner"
	Admin  = "admin"
	TA     = "ta"
	Viewer = "viewer"
)

// Valid reports whether r can be given to someone by invite or role change.
func Valid(r string) bool { return r == Admin || r == TA || r == Viewer || r == Owner }

type Capability string

const (
	CapView         Capability = "view"
	CapMark         Capability = "mark"
	CapLeave        Capability = "leave"
	CapCare         Capability = "care"
	CapCup          Capability = "cup"
	CapAnnounce     Capability = "announce"
	CapGifts        Capability = "gifts"
	CapCohortAdmin  Capability = "cohort_admin"
	CapReportNotes  Capability = "report_notes"
	CapTeam         Capability = "team"
	CapCreateCohort Capability = "create_cohort"
)

var table = map[string]map[Capability]bool{
	Admin:  {CapView: true, CapMark: true, CapLeave: true, CapCare: true, CapCup: true, CapAnnounce: true, CapGifts: true, CapCohortAdmin: true, CapReportNotes: true},
	TA:     {CapView: true, CapMark: true, CapLeave: true, CapCare: true, CapCup: true, CapAnnounce: true, CapReportNotes: true},
	Viewer: {CapView: true},
}

// Effective is the staff role for a user: "" for non-staff, Owner for staff with no role set
// (accounts that were admins before roles existed).
func Effective(role, staffRole string) string {
	if role != "admin" {
		return ""
	}
	if staffRole == "" {
		return Owner
	}
	return staffRole
}

// Can reports whether a staff role has a capability.
func Can(role string, c Capability) bool {
	if role == Owner {
		return true
	}
	return table[role][c]
}

// writes maps /admin path fragments to the capability a write needs. Order matters: first match wins.
var writes = []struct {
	frag string
	cap  Capability
}{
	{"/team", CapTeam},
	{"/gift-boxes", CapGifts},
	{"/badges", CapGifts},
	{"/care-energy", CapGifts},
	{"/cosmetics", CapGifts},
	{"/character-cosmetics", CapGifts},
	{"/baro-characters", CapGifts},
	{"/plant", CapGifts},
	{"/god-events", CapGifts},
	{"/showcase-lawn", CapGifts},
	{"/reflections/", CapCare},
	{"/notes", CapCare},
	{"/support", CapCare},
	{"/attendance/mark", CapMark},
	{"/attendance/manual", CapMark},
	{"/attendance/bulk", CapMark},
	{"/attendance/lock", CapMark},
	{"/attendance/generate-code", CapMark},
	{"/leave-requests", CapLeave},
	{"/cup/", CapCup},
	{"/notifications", CapAnnounce},
	{"/report/notes", CapReportNotes},
}

// RequiredCap is the capability a request to an /admin path needs.
func RequiredCap(method, path string) Capability {
	p := strings.TrimSuffix(strings.TrimPrefix(path, "/admin"), "/")
	if method == "GET" || method == "HEAD" {
		switch {
		case strings.HasPrefix(p, "/team"), strings.HasPrefix(p, "/settings/slack"):
			return CapTeam
		case strings.HasPrefix(p, "/attendance/export"):
			return CapCohortAdmin
		}
		return CapView
	}
	if method == "POST" && p == "/cohorts" {
		return CapCreateCohort
	}
	if strings.HasPrefix(p, "/settings/slack") {
		return CapTeam
	}
	// Restoring or deleting a single attendance record is marking.
	if strings.HasPrefix(p, "/attendance/") && (strings.HasSuffix(p, "/restore") || method == "DELETE") {
		return CapMark
	}
	for _, w := range writes {
		if strings.Contains(p, w.frag) {
			return w.cap
		}
	}
	return CapCohortAdmin
}

// CohortFromRequest finds the cohort a request is about: /admin/cohorts/:n/... or ?cohort=.
func CohortFromRequest(path string, query func(string) string) (int, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for i := 0; i+1 < len(parts); i++ {
		if parts[i] == "cohorts" {
			if n, err := strconv.Atoi(parts[i+1]); err == nil && n > 0 {
				return n, true
			}
		}
	}
	if n, err := strconv.Atoi(query("cohort")); err == nil && n > 0 {
		return n, true
	}
	return 0, false
}

// InScope: Owners see every cohort; everyone else only their listed cohorts.
func InScope(role string, cohorts []int, n int) bool {
	if role == Owner {
		return true
	}
	for _, c := range cohorts {
		if c == n {
			return true
		}
	}
	return false
}

// Ctx is the signed-in staff member, set by the admin guard.
type Ctx struct {
	UserID  string
	Name    string
	Role    string
	Cohorts []int
	Groups  []string
}
