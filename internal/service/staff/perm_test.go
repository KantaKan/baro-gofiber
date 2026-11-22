package staff

import "testing"

func TestEffective(t *testing.T) {
	if Effective("learner", "") != "" || Effective("admin", "") != Owner || Effective("admin", "ta") != TA || Effective("disabled", "admin") != "" {
		t.Fatal("effective role wrong")
	}
}

func TestAllowed(t *testing.T) {
	cases := []struct {
		role, method, path string
		ok                 bool
	}{
		{Viewer, "GET", "/admin/today", true},
		{Viewer, "POST", "/admin/attendance/mark", false},
		{Viewer, "PATCH", "/admin/leave-requests/abc", false},
		{Viewer, "DELETE", "/admin/notes/x", false},
		{TA, "POST", "/admin/attendance/mark", true},
		{TA, "POST", "/admin/learners/x/notes", true},
		{TA, "POST", "/admin/cup/awards", true},
		{TA, "POST", "/admin/notifications", true},
		{TA, "POST", "/admin/holidays", false},
		{TA, "GET", "/admin/attendance/export/salesforce", false},
		{TA, "PUT", "/admin/cohorts/13/groups", false},
		{TA, "POST", "/admin/users/x/gift-boxes", false},
		{Admin, "PUT", "/admin/cohorts/13/settings", true},
		{Admin, "POST", "/admin/cohorts/13/gift-boxes", true},
		{Admin, "GET", "/admin/team", false},
		{Admin, "POST", "/admin/cohorts", false},
		{Admin, "POST", "/admin/something-new", true},
		{TA, "POST", "/admin/something-new", false},
		{Owner, "GET", "/admin/team", true},
		{Owner, "POST", "/admin/cohorts", true},
		{"", "GET", "/admin/today", false},
		{TA, "POST", "/admin/nudges/ping", true},
		{Viewer, "GET", "/admin/nudges/status", true},
		{Viewer, "POST", "/admin/nudges/ping", false},
		{Admin, "GET", "/admin/settings/slack", false},
		{Admin, "PUT", "/admin/settings/slack/settings", false},
		{Admin, "POST", "/admin/settings/slack/connect", false},
		{TA, "DELETE", "/admin/settings/slack", false},
		{Owner, "POST", "/admin/settings/slack/connect", true},
		{Owner, "GET", "/admin/settings/slack/log", true},
	}
	for _, c := range cases {
		if got := Can(c.role, RequiredCap(c.method, c.path)); got != c.ok {
			t.Errorf("%s %s %s = %v, want %v", c.role, c.method, c.path, got, c.ok)
		}
	}
}

func TestPathCaseDoesNotMatter(t *testing.T) {
	// Fiber routes are case-insensitive, so the permission table must be too.
	if RequiredCap("GET", "/admin/TEAM") != CapTeam || RequiredCap("GET", "/admin/Attendance/Export/Salesforce") != CapCohortAdmin {
		t.Error("GET paths must match ignoring case")
	}
	if RequiredCap("POST", "/ADMIN/cohorts") != CapCreateCohort || RequiredCap("POST", "/admin/TEAM/invites/x/renew") != CapTeam {
		t.Error("write paths must match ignoring case")
	}
	if n, ok := CohortFromRequest("/admin/Cohorts/14/Settings", func(string) string { return "" }); !ok || n != 14 {
		t.Error("cohort in path must match ignoring case")
	}
}

func TestLearnerFromRequest(t *testing.T) {
	cases := map[string]string{
		"/admin/learners/abc123/overview":  "abc123",
		"/admin/Learners/abc123/notes":     "abc123",
		"/admin/users/abc123/badges":       "abc123",
		"/admin/userreflections/abc123":    "abc123",
		"/admin/attendance/student/abc123": "abc123",
	}
	for path, want := range cases {
		if got, ok := LearnerFromPath(path); !ok || got != want {
			t.Errorf("%s = %q %v, want %q", path, got, ok, want)
		}
	}
	for _, p := range []string{"/admin/learners", "/admin/today", "/admin/users/bulk-register", "/admin/notes/abc"} {
		if _, ok := LearnerFromPath(p); ok {
			t.Errorf("%s should name no learner", p)
		}
	}
}

func TestCohortScope(t *testing.T) {
	q := func(v string) func(string) string { return func(string) string { return v } }
	if n, ok := CohortFromRequest("/admin/cohorts/14/settings", q("")); !ok || n != 14 {
		t.Error("path cohort")
	}
	if n, ok := CohortFromRequest("/admin/today", q("13")); !ok || n != 13 {
		t.Error("query cohort")
	}
	if _, ok := CohortFromRequest("/admin/today", q("")); ok {
		t.Error("no cohort")
	}
	if _, ok := CohortFromRequest("/admin/cohorts", q("")); ok {
		t.Error("create has no cohort")
	}
	if !InScope(Owner, nil, 99) || !InScope(TA, []int{13}, 13) || InScope(TA, []int{13}, 14) || InScope(Admin, nil, 13) {
		t.Error("scope")
	}
}
