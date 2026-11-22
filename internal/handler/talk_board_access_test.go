package handler

import (
	"testing"

	"gofiber-baro/internal/service/staff"
)

func TestBoardReadScope(t *testing.T) {
	learner := boardActor{ownCohort: 13}
	owner := boardActor{role: staff.Owner, ownCohort: 0}
	ta := boardActor{role: staff.TA, cohorts: []int{13}, ownCohort: 0}
	cases := []struct {
		name string
		a    boardActor
		req  int
		want int
		ok   bool
	}{
		{"learner always gets their own cohort", learner, 14, 13, true},
		{"owner sees the cohort asked for", owner, 14, 14, true},
		{"owner with no cohort sees all (old pages)", owner, 0, 0, true},
		{"TA sees their cohort", ta, 13, 13, true},
		{"TA can't read another cohort", ta, 14, 0, false},
		{"TA with no cohort gets their first one, not everything", ta, 0, 13, true},
	}
	for _, c := range cases {
		got, ok := boardCohortForRead(c.a, c.req)
		if got != c.want || ok != c.ok {
			t.Errorf("%s: got %d %v, want %d %v", c.name, got, ok, c.want, c.ok)
		}
	}
}

func TestBoardPostTarget(t *testing.T) {
	cases := []struct {
		name string
		a    boardActor
		req  int
		want int
		ok   bool
	}{
		{"learner posts to their own cohort whatever they send", boardActor{ownCohort: 13}, 14, 13, true},
		{"owner posts to the cohort they're looking at", boardActor{role: staff.Owner, ownCohort: 13}, 14, 14, true},
		{"admin posts in scope", boardActor{role: staff.Admin, cohorts: []int{13, 14}}, 14, 14, true},
		{"TA can't post to another cohort", boardActor{role: staff.TA, cohorts: []int{13}}, 14, 0, false},
		{"viewer can't post", boardActor{role: staff.Viewer, cohorts: []int{13}}, 13, 0, false},
		{"staff with no cohort sent fall back to their own, if in scope", boardActor{role: staff.TA, cohorts: []int{13}, ownCohort: 13}, 0, 13, true},
		{"staff with no cohort anywhere can't post", boardActor{role: staff.Admin, cohorts: []int{13}}, 0, 0, false},
	}
	for _, c := range cases {
		got, ok := boardCohortForPost(c.a, c.req)
		if got != c.want || ok != c.ok {
			t.Errorf("%s: got %d %v, want %d %v", c.name, got, ok, c.want, c.ok)
		}
	}
}

func TestBoardModeration(t *testing.T) {
	if !boardMayModerate(boardActor{role: staff.Owner}, 14) || !boardMayModerate(boardActor{role: staff.TA, cohorts: []int{13}}, 13) {
		t.Error("owner anywhere, TA in their cohort")
	}
	for _, a := range []boardActor{{role: staff.Viewer, cohorts: []int{13}}, {role: staff.TA, cohorts: []int{13}}, {ownCohort: 14}} {
		if boardMayModerate(a, 14) {
			t.Errorf("%+v must not moderate JSD14", a)
		}
	}
}
