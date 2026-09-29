package main

import (
	"os"
	"strings"
	"testing"
)

func TestCareEnergyRoutesHaveNoOldCompatibilityPaths(t *testing.T) {
	source, err := os.ReadFile("routes.go")
	if err != nil {
		t.Fatal(err)
	}
	routes := string(source)
	for _, path := range []string{
		"/:id/care-energy/protect",
		"/:id/care-energy/feed",
		"/:id/care-energy/gift",
		"/:id/care-energy/rescue",
		"/:id/care-energy/character-care",
		"/users/:id/care-energy",
		"/care-energy/bulk",
	} {
		if !strings.Contains(routes, path) {
			t.Fatalf("missing Care Energy route %s", path)
		}
	}
	if strings.Contains(routes, "/"+migrationLegacyResourceName()) {
		t.Fatal("old compatibility routes are still registered")
	}
}

func migrationLegacyResourceName() string {
	return "ferti" + "lizer"
}
