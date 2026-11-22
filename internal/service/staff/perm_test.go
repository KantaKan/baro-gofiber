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
	}
	for _, c := range cases {
		if got := Can(c.role, RequiredCap(c.method, c.path)); got != c.ok {
			t.Errorf("%s %s %s = %v, want %v", c.role, c.method, c.path, got, c.ok)
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
