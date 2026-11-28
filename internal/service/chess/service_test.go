package chess

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand"
	"strings"
	"testing"
	"time"
)

type memStore struct {
	games []*Game
	msgs  []Message
}

func (m *memStore) Active(_ context.Context, cohort int) (*Game, error) {
	for _, g := range m.games {
		if g.CohortNumber == cohort && g.Status == StatusActive {
			c := *g
			c.Moves = append([]Move(nil), g.Moves...)
			c.Proposals = append([]Proposal(nil), g.Proposals...)
			return &c, nil
		}
	}
	return nil, nil
}
func (m *memStore) AllActive(context.Context) ([]Game, error) {
	var out []Game
	for _, g := range m.games {
		if g.Status == StatusActive {
			out = append(out, *g)
		}
	}
	return out, nil
}
func (m *memStore) Insert(_ context.Context, g *Game) error {
	for _, x := range m.games {
		if x.CohortNumber == g.CohortNumber && x.Status == StatusActive {
			return ErrConflict
		}
	}
	c := *g
	m.games = append(m.games, &c)
	return nil
}
func (m *memStore) Save(_ context.Context, g *Game, version int) (bool, error) {
	for i, s := range m.games {
		if s.ID == g.ID {
			if s.Version != version {
				return false, nil
			}
			c := *g
			m.games[i] = &c
			return true, nil
		}
	}
	return false, nil
}
func (m *memStore) AddMessage(_ context.Context, msg Message) error {
	m.msgs = append(m.msgs, msg)
	return nil
}
func (m *memStore) Messages(_ context.Context, gameID, room, after string, limit int) ([]Message, error) {
	var out []Message
	seen := after == ""
	found := false
	for _, x := range m.msgs {
		if x.ID == after {
			found = true
		}
	}
	if !found {
		seen = true
	}
	for _, x := range m.msgs {
		if x.GameID != gameID || x.Room != room {
			continue
		}
		if seen {
			out = append(out, x)
		}
		if x.ID == after {
			seen = true
		}
	}
	return out, nil
}

type fakePeople map[string]Person

func (f fakePeople) Get(id string) (*Person, error) {
	if p, ok := f[id]; ok {
		return &p, nil
	}
	return nil, errors.New("no user")
}
func (f fakePeople) Learners(cohort int) ([]Person, error) {
	var out []Person
	for _, p := range f {
		if p.Cohort == cohort {
			out = append(out, p)
		}
	}
	return out, nil
}

func cohort13() fakePeople {
	f := fakePeople{}
	for i, g := range []string{"A", "A", "B", "B", "C", "D"} {
		id := string(rune('a' + i))
		f[id] = Person{ID: id, Name: strings.ToUpper(id), Group: g, Cohort: 13}
	}
	return f
}

func newSvc(people fakePeople) (*Service, *memStore, *time.Time) {
	st := &memStore{}
	now := time.Date(2026, 11, 26, 10, 0, 0, 0, bangkok)
	s := NewService(st, people, func(int) (bool, string) { return true, "" }, func() []int { return []int{13} })
	s.now = func() time.Time { return now }
	s.rng = rand.New(rand.NewSource(3))
	return s, st, &now
}

func picked(t *testing.T, s *Service) (string, *Game) {
	t.Helper()
	g, err := s.Ensure(context.Background(), 13)
	if err != nil || g == nil {
		t.Fatalf("ensure: %v %v", g, err)
	}
	return g.Turn.PickedUserID, g
}

func TestEnsureSplitsTeamsAndPicksWhite(t *testing.T) {
	s, _, _ := newSvc(cohort13())
	p, g := picked(t, s)
	if len(g.Teams[White].Members)+len(g.Teams[Black].Members) != 6 || g.Turn.Side != White {
		t.Fatalf("teams %+v turn %+v", g.Teams, g.Turn)
	}
	if !contains(g.Teams[White].Members, p) {
		t.Errorf("picked %s is not on white", p)
	}
	if g2, _ := s.Ensure(context.Background(), 13); g2.ID != g.ID {
		t.Error("ensure must not create a second game")
	}
}

func TestOneGroupMakesNoGame(t *testing.T) {
	f := fakePeople{"a": {ID: "a", Group: "A", Cohort: 13}, "b": {ID: "b", Group: "A", Cohort: 13}}
	s, _, _ := newSvc(f)
	if g, err := s.Ensure(context.Background(), 13); g != nil || err != nil {
		t.Errorf("one genmate group: no game, got %v %v", g, err)
	}
}

func TestOnlyThePickedMemberMovesLegally(t *testing.T) {
	s, st, _ := newSvc(cohort13())
	p, g := picked(t, s)
	other := g.Teams[Black].Members[0]
	if err := s.Move(context.Background(), other, "e7e5"); !errors.Is(err, ErrNotYourTurn) {
		t.Errorf("black member moving on white's turn: %v", err)
	}
	if err := s.Move(context.Background(), p, "e2e5"); !errors.Is(err, ErrIllegal) {
		t.Errorf("illegal move: %v", err)
	}
	if err := s.Move(context.Background(), p, "e2e4"); err != nil {
		t.Fatal(err)
	}
	g, _ = st.Active(context.Background(), 13)
	if g.Turn.Side != Black || len(g.Moves) != 1 || g.Moves[0].SAN != "e4" || !contains(g.Teams[Black].Members, g.Turn.PickedUserID) {
		t.Errorf("after e4: %+v", g.Turn)
	}
	if len(st.msgs) == 0 || st.msgs[len(st.msgs)-1].Room != RoomAll {
		t.Error("a move is announced in the cohort room")
	}
}

func TestProposalsStayWithinTheTeam(t *testing.T) {
	s, _, _ := newSvc(cohort13())
	_, g := picked(t, s)
	white, black := g.Teams[White].Members[0], g.Teams[Black].Members[0]
	if err := s.Propose(context.Background(), white, "d2d4", "centre"); err != nil {
		t.Fatal(err)
	}
	if err := s.Propose(context.Background(), black, "e7e5", ""); !errors.Is(err, ErrNotYourSide) {
		t.Errorf("black cannot propose on white's turn: %v", err)
	}
	if v, _ := s.View(context.Background(), black); len(v.Proposals) != 0 {
		t.Error("black must not see white's proposals")
	}
	if v, _ := s.View(context.Background(), white); len(v.Proposals) != 1 || v.Proposals[0].SAN != "d4" {
		t.Errorf("white sees its proposal: %+v", v.Proposals)
	}
	if _, err := s.Messages(context.Background(), black, RoomWhite, ""); !errors.Is(err, ErrNoAccess) {
		t.Errorf("black cannot read white's room: %v", err)
	}
}

func TestStaffWatchesAnyCohort(t *testing.T) {
	f := cohort13()
	f["staff"] = Person{ID: "staff", Name: "TA", Staff: true}
	s, _, _ := newSvc(f)
	_, g := picked(t, s)
	_ = s.Propose(context.Background(), g.Teams[White].Members[0], "d2d4", "")
	ctx := WithCohort(context.Background(), 13)
	v, err := s.View(ctx, "staff")
	if err != nil || v.Game == nil || len(v.Proposals) != 1 || len(v.Teams[White].Members) == 0 {
		t.Fatalf("staff view: %+v %v", v, err)
	}
	if _, err := s.Messages(ctx, "staff", RoomBlack, ""); err != nil {
		t.Errorf("staff reads team rooms: %v", err)
	}
	f["a"] = Person{ID: "a", Group: "A", Cohort: 13}
	if v, _ := s.View(WithCohort(context.Background(), 99), "a"); v.Game == nil {
		t.Error("a learner's cohort override is ignored")
	}
}

func TestTimeoutPlaysTheTopProposalOrRepicks(t *testing.T) {
	s, st, now := newSvc(cohort13())
	p, g := picked(t, s)
	mate := g.Teams[White].Members[0]
	if mate == p && len(g.Teams[White].Members) > 1 {
		mate = g.Teams[White].Members[1]
	}
	_ = s.Propose(context.Background(), mate, "g1f3", "")
	*now = g.Turn.Deadline.Add(time.Minute)
	if err := s.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	g, _ = st.Active(context.Background(), 13)
	if len(g.Moves) != 1 || g.Moves[0].SAN != "Nf3" || !g.Moves[0].ByTeam {
		t.Fatalf("timeout plays the team's choice: %+v", g.Moves)
	}
	before := g.Turn.PickedUserID
	*now = g.Turn.Deadline.Add(time.Minute)
	_ = s.Tick(context.Background())
	g, _ = st.Active(context.Background(), 13)
	if len(g.Moves) != 1 || g.Turn.Side != Black || g.Turn.PickedUserID == "" || (len(g.Teams[Black].Members) > 1 && g.Turn.PickedUserID == before) {
		t.Errorf("no proposal: a new black member is picked: %+v", g.Turn)
	}
}

func TestCheckmateEndsAndStartsTheNextGame(t *testing.T) {
	s, st, _ := newSvc(cohort13())
	for _, mv := range []string{"f2f3", "e7e5", "g2g4", "d8h4"} {
		p, _ := picked(t, s)
		if err := s.Move(context.Background(), p, mv); err != nil {
			t.Fatalf("%s: %v", mv, err)
		}
	}
	if len(st.games) != 2 || st.games[0].Status != StatusEnded || st.games[0].Result != Black {
		t.Fatalf("fool's mate: %+v", st.games[0])
	}
	if st.games[1].Status != StatusActive || len(st.games[1].Moves) != 0 {
		t.Error("a new game starts right away")
	}
}

func TestStaleWriteIsRejected(t *testing.T) {
	s, st, _ := newSvc(cohort13())
	p, _ := picked(t, s)
	st.games[0].Version++ // someone else wrote in between
	stale := *st.games[0]
	stale.Version--
	st.games[0].Version = stale.Version + 1
	g, _ := st.Active(context.Background(), 13)
	g.Version--
	if ok, _ := st.Save(context.Background(), g, g.Version); ok {
		t.Error("a write based on an old version must not apply")
	}
	_ = p
}

func TestPromotionIsRecorded(t *testing.T) {
	s, st, _ := newSvc(cohort13())
	_, g := picked(t, s)
	g.FEN = "8/4P3/8/8/8/8/k7/4K3 w - - 0 1"
	g.Moves = nil
	st.games[0].FEN = g.FEN
	st.games[0].Start = g.FEN
	p := st.games[0].Turn.PickedUserID
	if err := s.Move(context.Background(), p, "e7e8q"); err != nil {
		t.Fatal(err)
	}
	if got := st.games[0].Moves[0].SAN; got != "e8=Q" && got != "e8=Q+" {
		t.Errorf("promotion SAN %q", got)
	}
}

func TestNewTurnSendsAnEmptyProposalList(t *testing.T) {
	s, st, _ := newSvc(cohort13())
	p, _ := picked(t, s)
	if err := s.Move(context.Background(), p, "e2e4"); err != nil {
		t.Fatal(err)
	}
	g, _ := st.Active(context.Background(), 13)
	v, _ := s.View(context.Background(), g.Turn.PickedUserID)
	if b, _ := json.Marshal(v); !strings.Contains(string(b), `"proposals":[]`) {
		t.Errorf("proposals must be [] not null: %s", b)
	}
}

// racyStore hides the active game from the first Active call, as when two requests run Ensure together.
type racyStore struct {
	*memStore
	hide bool
}

func (r *racyStore) Active(ctx context.Context, cohort int) (*Game, error) {
	if r.hide {
		r.hide = false
		return nil, nil
	}
	return r.memStore.Active(ctx, cohort)
}

func TestEnsureRaceKeepsOneGame(t *testing.T) {
	s, st, _ := newSvc(cohort13())
	first, _ := picked(t, s)
	_ = first
	r := &racyStore{memStore: st, hide: true}
	s.store = r
	g, err := s.Ensure(context.Background(), 13)
	if err != nil || g == nil || g.ID != st.games[0].ID || len(st.games) != 1 {
		t.Fatalf("a racing Ensure must reuse the game: %v %v games=%d", g, err, len(st.games))
	}
}

type brokenPeople struct{ fakePeople }

func (b brokenPeople) Learners(cohort int) ([]Person, error) {
	if cohort == 99 {
		return nil, errors.New("mongo down")
	}
	return b.fakePeople.Learners(cohort)
}

func TestTickKeepsGoingPastABrokenCohort(t *testing.T) {
	s, st, _ := newSvc(cohort13())
	s.people = brokenPeople{cohort13()}
	s.cohorts = func() []int { return []int{99, 13} }
	_ = s.Tick(context.Background())
	if len(st.games) != 1 || st.games[0].CohortNumber != 13 {
		t.Errorf("cohort 13 still gets its game: %d", len(st.games))
	}
}

func TestDisabledOrGraduatedCohortIsFrozen(t *testing.T) {
	s, st, now := newSvc(cohort13())
	p, g := picked(t, s)
	s.settings = func(int) (bool, string) { return false, "" }
	if err := s.Move(context.Background(), p, "e2e4"); !errors.Is(err, ErrNoGame) {
		t.Errorf("chess off: %v", err)
	}
	s.settings = func(int) (bool, string) { return true, "2026-11-01" }
	if err := s.Move(context.Background(), p, "e2e4"); !errors.Is(err, ErrNoGame) {
		t.Errorf("graduated: %v", err)
	}
	if on, mine := s.Me(context.Background(), p); on || mine {
		t.Error("no nav item or turn card for a frozen game")
	}
	*now = g.Turn.Deadline.Add(time.Hour)
	_ = s.Tick(context.Background())
	if st.games[0].Turn.PickedUserID != p || len(st.games[0].Moves) != 0 {
		t.Error("the tick leaves a frozen game alone")
	}
}

type flakyStore struct {
	*memStore
	fails int
}

func (f *flakyStore) Save(ctx context.Context, g *Game, version int) (bool, error) {
	if f.fails > 0 {
		f.fails--
		return false, nil
	}
	return f.memStore.Save(ctx, g, version)
}

func TestVoteRetriesAConflictAndRejectsAnUnknownProposal(t *testing.T) {
	s, st, _ := newSvc(cohort13())
	_, g := picked(t, s)
	w := g.Teams[White].Members
	_ = s.Propose(context.Background(), w[0], "d2d4", "")
	id := st.games[0].Proposals[0].ID
	s.store = &flakyStore{memStore: st, fails: 1}
	voter := w[len(w)-1]
	if err := s.Vote(context.Background(), voter, id); err != nil {
		t.Fatalf("a vote that loses one race is retried: %v", err)
	}
	if !contains(st.games[0].Proposals[0].Voters, voter) {
		t.Error("vote recorded")
	}
	if err := s.Vote(context.Background(), voter, "nope"); !errors.Is(err, ErrConflict) {
		t.Errorf("unknown proposal: %v", err)
	}
}
