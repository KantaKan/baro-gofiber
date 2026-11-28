// Package chess runs the cohort team chess game: two sides made of genmate groups, a random
// picked mover each turn, team proposals, and chat.
package chess

import (
	"math/rand"
	"sort"
	"time"
)

// TurnHours is how long a picked member has, counted only inside the working window.
const TurnHours = 4

const dayStart, dayEnd = 9, 21

var bangkok = func() *time.Location {
	if l, err := time.LoadLocation("Asia/Bangkok"); err == nil {
		return l
	}
	return time.FixedZone("ICT", 7*3600)
}()

// Proposal is a teammate's suggested move and who agreed with it.
type Proposal struct {
	ID       string    `bson:"id" json:"id"`
	UserID   string    `bson:"user_id" json:"user_id"`
	UserName string    `bson:"user_name" json:"user_name"`
	UCI      string    `bson:"uci" json:"uci"`
	SAN      string    `bson:"san" json:"san"`
	Note     string    `bson:"note" json:"note"`
	Voters   []string  `bson:"voters" json:"voters"`
	At       time.Time `bson:"at" json:"at"`
}

// SplitTeams shuffles genmate groups onto two sides with learner counts as even as possible,
// largest group first onto the smaller side. Fewer than two groups cannot make a game.
func SplitTeams(groups map[string]int, rng *rand.Rand) (white, black []string) {
	if len(groups) < 2 {
		return nil, nil
	}
	names := make([]string, 0, len(groups))
	for g := range groups {
		names = append(names, g)
	}
	sort.Strings(names)
	rng.Shuffle(len(names), func(i, j int) { names[i], names[j] = names[j], names[i] })
	sort.SliceStable(names, func(i, j int) bool { return groups[names[i]] > groups[names[j]] })
	var sides [2][]string
	var sizes [2]int
	for _, g := range names {
		s := 0
		if sizes[1] < sizes[0] {
			s = 1
		}
		sides[s] = append(sides[s], g)
		sizes[s] += groups[g]
	}
	if rng.Intn(2) == 0 {
		return sides[0], sides[1]
	}
	return sides[1], sides[0]
}

// PickNext chooses a random member not yet picked this cycle and returns the updated cycle;
// once everyone has moved the cycle starts over.
func PickNext(members, picked []string, rng *rand.Rand) (string, []string) {
	if len(members) == 0 {
		return "", picked
	}
	done := map[string]bool{}
	for _, p := range picked {
		done[p] = true
	}
	var open []string
	for _, m := range members {
		if !done[m] {
			open = append(open, m)
		}
	}
	if len(open) == 0 {
		open, picked = members, nil
	}
	p := open[rng.Intn(len(open))]
	return p, append(picked, p)
}

// Deadline is TurnHours after from, counting only 09:00-21:00 Bangkok time.
func Deadline(from time.Time) time.Time {
	t := from.In(bangkok)
	left := TurnHours * time.Hour
	for {
		start := time.Date(t.Year(), t.Month(), t.Day(), dayStart, 0, 0, 0, bangkok)
		end := time.Date(t.Year(), t.Month(), t.Day(), dayEnd, 0, 0, 0, bangkok)
		if t.Before(start) {
			t = start
		}
		if !t.Before(end) {
			t = start.AddDate(0, 0, 1)
			continue
		}
		if avail := end.Sub(t); left <= avail {
			return t.Add(left)
		} else {
			left -= avail
			t = start.AddDate(0, 0, 1)
		}
	}
}

// TopProposal is the proposal with the most votes, the earliest on a tie.
func TopProposal(ps []Proposal) *Proposal {
	var best *Proposal
	for i := range ps {
		p := &ps[i]
		if best == nil || len(p.Voters) > len(best.Voters) || (len(p.Voters) == len(best.Voters) && p.At.Before(best.At)) {
			best = p
		}
	}
	return best
}
