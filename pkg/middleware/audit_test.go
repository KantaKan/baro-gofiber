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
		{"POST", "/api/admin/users/123/baro-characters", true},
		{"PUT", "/api/admin/users/123/baro-characters/equipped", true},
		{"PUT", "/api/admin/users/123/baro-characters/pinned", true},
		{"POST", "/admin/users/123/character-cosmetics/character_prop:flower", true},
		{"PUT", "/character-cosmetics/equipment/character_prop", true},
		{"POST", "/gift-boxes/123/transfer", true},
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

func TestAuditTargetsKeepsWhoAndSafeFactsOnly(t *testing.T) {
	mint, ploy := "6b0438bb46b0813c186aaf4b", "6b0438bb46b0813c186aaf49"
	body := []byte(`{"user_id":"` + mint + `","user_ids":["` + ploy + `","` + mint + `"],"date":"2026-11-24","session":"morning","status":"late","reason":"private note","password":"x","points":15}`)
	targets, detail := auditTargets(map[string]string{"id": "not-an-id"}, body)
	if len(targets) != 2 || targets[0] != mint || targets[1] != ploy {
		t.Fatalf("targets: %v", targets)
	}
	if detail["status"] != "late" || detail["session"] != "morning" || detail["date"] != "2026-11-24" || detail["points"] != "15" {
		t.Fatalf("detail: %v", detail)
	}
	if _, ok := detail["reason"]; ok {
		t.Fatal("free text must not be logged")
	}
	if _, ok := detail["password"]; ok {
		t.Fatal("secrets must not be logged")
	}
	if targets, _ := auditTargets(map[string]string{"userId": mint}, nil); len(targets) != 1 {
		t.Fatalf("route param target: %v", targets)
	}
}
