package handler

import "testing"

func TestWebLinkOnlyAllowsHTTP(t *testing.T) {
	for in, ok := range map[string]bool{
		"":                                  true,
		"  https://github.com/ploy  ":       true,
		"http://linkedin.com/in/x":          true,
		"javascript:alert(document.cookie)": false,
		"JavaScript:alert(1)":               false,
		" javascript:fetch('//evil')":       false,
		"data:text/html,<script>x</script>": false,
		"https://":                          false,
		"//evil.example":                    false,
		"github.com/ploy":                   false,
	} {
		if _, got := webLink(in); got != ok {
			t.Errorf("webLink(%q) = %v, want %v", in, got, ok)
		}
	}
}
