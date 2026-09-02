package middleware

import "testing"

func TestShouldAudit(t *testing.T) {
	cases := []struct {
		method, path string
		want         bool
	}{
		{"GET", "/admin/users", false},
		{"OPTIONS", "/admin/users", false},
		{"POST", "/admin/holidays", true},
		{"PUT", "/admin/notifications/123", true},
		{"PATCH", "/users/123/plant", true},
		{"DELETE", "/board/posts/123", true},
		{"POST", "/login", false},
		{"POST", "/api/notifications/abc/read", false},
		{"GET", "/api/notifications", false},
	}

	for _, tc := range cases {
		if got := shouldAudit(tc.method, tc.path); got != tc.want {
			t.Errorf("shouldAudit(%q, %q) = %v, want %v", tc.method, tc.path, got, tc.want)
		}
	}
}
