package chess

import (
	"math/rand"
	"testing"
	"time"
)

func TestSplitTeamsBalancesLearners(t *testing.T) {
	groups := map[string]int{"A": 6, "B": 5, "C": 5, "D": 4, "E": 3}
	for seed := int64(0); seed < 20; seed++ {
		w, b := SplitTeams(groups, rand.New(rand.NewSource(seed)))
		sum := func(gs []string) (n int) {
			for _, g := range gs {
				n += groups[g]
			}
			return
		}
		if len(w)+len(b) != 5 || len(w) == 0 || len(b) == 0 {
			t.Fatalf("seed %d: every group on a side: %v %v", seed, w, b)
		}
		if d := sum(w) - sum(b); d > 3 || d < -3 {
			t.Errorf("seed %d: sides %d vs %d are not balanced", seed, sum(w), sum(b))
		}
	}
}

func TestSplitTeamsNeedsTwoGroups(t *testing.T) {
	if w, b := SplitTeams(map[string]int{"A": 9}, rand.New(rand.NewSource(1))); w != nil || b != nil {
		t.Errorf("one group cannot make two teams: %v %v", w, b)
	}
	if w, b := SplitTeams(map[string]int{"A": 3, "B": 1}, rand.New(rand.NewSource(1))); len(w) != 1 || len(b) != 1 {
		t.Errorf("two groups make two teams: %v %v", w, b)
	}
}

func TestPickNextCyclesWithoutRepeats(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	members := []string{"a", "b", "c", "d"}
	var picked []string
	seen := map[string]bool{}
	for i := 0; i < 4; i++ {
		var p string
		p, picked = PickNext(members, picked, rng)
		if seen[p] {
			t.Fatalf("%s picked twice before everyone moved", p)
		}
		seen[p] = true
	}
	p, picked := PickNext(members, picked, rng)
	if len(picked) != 1 || picked[0] != p {
		t.Errorf("after a full cycle the next pick starts a new cycle: %v", picked)
	}
	if p, _ := PickNext([]string{"solo"}, []string{"solo"}, rng); p != "solo" {
		t.Errorf("a one-person team keeps picking them, got %q", p)
	}
	if p, _ := PickNext(nil, nil, rng); p != "" {
		t.Errorf("no members, no pick: %q", p)
	}
}

func TestDeadlineCountsWorkingHours(t *testing.T) {
	bkk, _ := time.LoadLocation("Asia/Bangkok")
	at := func(d, h, m int) time.Time { return time.Date(2026, 11, d, h, m, 0, 0, bkk) }
	cases := []struct{ from, want time.Time }{
		{at(26, 10, 30), at(26, 14, 30)},
		{at(26, 8, 0), at(26, 13, 0)},
		{at(26, 20, 0), at(27, 12, 0)},
		{at(26, 19, 0), at(27, 11, 0)},
		{at(26, 23, 0), at(27, 13, 0)},
		{at(26, 17, 0), at(26, 21, 0)},
	}
	for _, c := range cases {
		if got := Deadline(c.from); !got.Equal(c.want) {
			t.Errorf("Deadline(%s) = %s, want %s", c.from.Format("02 15:04"), got.In(bkk).Format("02 15:04"), c.want.Format("02 15:04"))
		}
	}
}

func TestTopProposal(t *testing.T) {
	t0 := time.Date(2026, 11, 26, 10, 0, 0, 0, time.UTC)
	ps := []Proposal{
		{ID: "late", Voters: []string{"x", "y"}, At: t0.Add(time.Minute)},
		{ID: "early", Voters: []string{"z", "w"}, At: t0},
		{ID: "few", Voters: []string{"q"}, At: t0.Add(-time.Hour)},
	}
	if p := TopProposal(ps); p == nil || p.ID != "early" {
		t.Errorf("most votes, earliest on tie: %+v", p)
	}
	if TopProposal(nil) != nil {
		t.Error("no proposals, no move")
	}
}
