package domain

import "testing"

func TestInSessionEndsAfterGraduation(t *testing.T) {
	running := CohortSettings{}
	if !running.InSession("2030-01-01") {
		t.Error("a cohort with no graduation date is always in session")
	}
	grad := CohortSettings{GraduatedOn: "2026-11-21"}
	for date, want := range map[string]bool{"2026-11-20": true, "2026-11-21": true, "2026-11-24": false} {
		if got := grad.InSession(date); got != want {
			t.Errorf("InSession(%s) = %v, want %v", date, got, want)
		}
	}
}
