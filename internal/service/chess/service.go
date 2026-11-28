package chess

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	nc "github.com/notnil/chess"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	White, Black                  = "white", "black"
	RoomWhite, RoomBlack, RoomAll = "white", "black", "all"
	StatusActive, StatusEnded     = "active", "ended"
	startFEN                      = "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"
	maxText, maxNote              = 500, 140
)

var (
	ErrNoGame      = errors.New("no game is running")
	ErrNotYourTurn = errors.New("it's not your turn to move")
	ErrNotYourSide = errors.New("only the side to move can do this")
	ErrIllegal     = errors.New("that move isn't legal here")
	ErrNoAccess    = errors.New("you can't use this room")
	ErrConflict    = errors.New("the board changed, try again")
	ErrBadText     = errors.New("message is empty or too long")
)

type Side struct {
	Groups  []string `bson:"groups" json:"groups"`
	Members []string `bson:"members" json:"members"`
}

type Move struct {
	Ply      int       `bson:"ply" json:"ply"`
	UCI      string    `bson:"uci" json:"uci"`
	SAN      string    `bson:"san" json:"san"`
	Side     string    `bson:"side" json:"side"`
	UserID   string    `bson:"user_id" json:"user_id"`
	UserName string    `bson:"user_name" json:"user_name"`
	ByTeam   bool      `bson:"by_team" json:"by_team"`
	At       time.Time `bson:"at" json:"at"`
}

type Turn struct {
	Side         string              `bson:"side" json:"side"`
	PickedUserID string              `bson:"picked_user_id" json:"picked_user_id"`
	PickedAt     time.Time           `bson:"picked_at" json:"picked_at"`
	Deadline     time.Time           `bson:"deadline" json:"deadline"`
	PickedSoFar  map[string][]string `bson:"picked_so_far" json:"-"`
}

type Game struct {
	ID           primitive.ObjectID `bson:"_id" json:"id"`
	CohortNumber int                `bson:"cohort_number" json:"cohort_number"`
	Status       string             `bson:"status" json:"status"`
	Version      int                `bson:"version" json:"-"`
	StartedAt    time.Time          `bson:"started_at" json:"started_at"`
	EndedAt      *time.Time         `bson:"ended_at,omitempty" json:"ended_at,omitempty"`
	Result       string             `bson:"result" json:"result"`
	EndReason    string             `bson:"end_reason" json:"end_reason"`
	Start        string             `bson:"start" json:"-"`
	FEN          string             `bson:"fen" json:"fen"`
	Moves        []Move             `bson:"moves" json:"moves"`
	Teams        map[string]Side    `bson:"teams" json:"-"`
	Turn         Turn               `bson:"turn" json:"turn"`
	Proposals    []Proposal         `bson:"proposals" json:"-"`
}

type Message struct {
	ID       string    `bson:"_id" json:"id"`
	GameID   string    `bson:"game_id" json:"game_id"`
	Room     string    `bson:"room" json:"room"`
	UserID   string    `bson:"user_id" json:"user_id"`
	UserName string    `bson:"user_name" json:"user_name"`
	Text     string    `bson:"text" json:"text"`
	System   bool      `bson:"system" json:"system"`
	At       time.Time `bson:"at" json:"at"`
}

type Person struct {
	ID, Name, Group string
	Cohort          int
	Staff           bool
}

type Store interface {
	Active(ctx context.Context, cohort int) (*Game, error)
	AllActive(ctx context.Context) ([]Game, error)
	Insert(ctx context.Context, g *Game) error
	Save(ctx context.Context, g *Game, version int) (bool, error)
	AddMessage(ctx context.Context, m Message) error
	Messages(ctx context.Context, gameID, room, after string, limit int) ([]Message, error)
}

type People interface {
	Get(id string) (*Person, error)
	Learners(cohort int) ([]Person, error)
}

type Service struct {
	store    Store
	people   People
	settings func(cohort int) (enabled bool, graduatedOn string)
	cohorts  func() []int
	now      func() time.Time
	rng      *rand.Rand
}

func NewService(store Store, people People, settings func(int) (bool, string), cohorts func() []int) *Service {
	return &Service{store: store, people: people, settings: settings, cohorts: cohorts, now: time.Now, rng: rand.New(rand.NewSource(time.Now().UnixNano()))}
}

// Ensure returns the cohort's running game, starting one when chess is on and none is running.
func (s *Service) Ensure(ctx context.Context, cohort int) (*Game, error) {
	if g, err := s.store.Active(ctx, cohort); err != nil || g != nil {
		return g, err
	}
	on, grad := s.settings(cohort)
	if !on || (grad != "" && grad < s.now().In(bangkok).Format("2006-01-02")) {
		return nil, nil
	}
	learners, err := s.people.Learners(cohort)
	if err != nil {
		return nil, err
	}
	counts := map[string]int{}
	for _, p := range learners {
		if p.Group != "" {
			counts[p.Group]++
		}
	}
	white, black := SplitTeams(counts, s.rng)
	if white == nil {
		return nil, nil
	}
	teams := map[string]Side{White: {Groups: white}, Black: {Groups: black}}
	for _, p := range learners {
		for side, t := range teams {
			if contains(t.Groups, p.Group) {
				t.Members = append(t.Members, p.ID)
				teams[side] = t
			}
		}
	}
	now := s.now()
	g := &Game{ID: primitive.NewObjectID(), CohortNumber: cohort, Status: StatusActive, StartedAt: now, Start: startFEN, FEN: startFEN, Teams: teams,
		Turn: Turn{Side: White, PickedSoFar: map[string][]string{}}}
	s.pick(g, White, now)
	if err := s.store.Insert(ctx, g); err != nil {
		return nil, err
	}
	s.announce(ctx, g, fmt.Sprintf("เกมใหม่เริ่มแล้ว · ทีมขาว กลุ่ม %s ปะทะ ทีมดำ กลุ่ม %s", strings.Join(white, " "), strings.Join(black, " ")))
	return g, nil
}

func (s *Service) pick(g *Game, side string, now time.Time) {
	p, cycle := PickNext(g.Teams[side].Members, g.Turn.PickedSoFar[side], s.rng)
	if g.Turn.PickedSoFar == nil {
		g.Turn.PickedSoFar = map[string][]string{}
	}
	g.Turn.Side, g.Turn.PickedUserID, g.Turn.PickedAt, g.Turn.Deadline = side, p, now, Deadline(now)
	g.Turn.PickedSoFar[side] = cycle
	g.Proposals = nil
}

func other(side string) string {
	if side == White {
		return Black
	}
	return White
}

func sideOf(g *Game, userID string) string {
	for side, t := range g.Teams {
		if contains(t.Members, userID) {
			return side
		}
	}
	return ""
}

// board replays the game from its start position, so repetition and fifty-move draws see the history.
func board(g *Game) (*nc.Game, error) {
	start := g.Start
	if start == "" {
		start = startFEN
	}
	fen, err := nc.FEN(start)
	if err != nil {
		return nil, err
	}
	b := nc.NewGame(fen, nc.UseNotation(nc.UCINotation{}))
	for _, m := range g.Moves {
		if err := b.MoveStr(m.UCI); err != nil {
			return nil, err
		}
	}
	return b, nil
}

// parse checks a UCI move against the current position and returns it with its SAN.
func parse(b *nc.Game, uci string) (*nc.Move, string, error) {
	pos := b.Position()
	mv, err := nc.UCINotation{}.Decode(pos, strings.ToLower(strings.TrimSpace(uci)))
	if err != nil {
		return nil, "", ErrIllegal
	}
	for _, v := range pos.ValidMoves() {
		if v.String() == mv.String() {
			return v, nc.AlgebraicNotation{}.Encode(pos, v), nil
		}
	}
	return nil, "", ErrIllegal
}

// play applies a move for the side to move, then ends the game or hands the turn over.
func (s *Service) play(ctx context.Context, g *Game, uci, userID string, byTeam bool) error {
	b, err := board(g)
	if err != nil {
		return err
	}
	mv, san, err := parse(b, uci)
	if err != nil {
		return err
	}
	if err := b.Move(mv); err != nil {
		return ErrIllegal
	}
	now := s.now()
	name := s.name(userID)
	g.Moves = append(g.Moves, Move{Ply: len(g.Moves) + 1, UCI: mv.String(), SAN: san, Side: g.Turn.Side, UserID: userID, UserName: name, ByTeam: byTeam, At: now})
	for _, m := range b.EligibleDraws() {
		if m == nc.ThreefoldRepetition || m == nc.FiftyMoveRule {
			_ = b.Draw(m)
			break
		}
	}
	g.FEN = b.FEN()
	mover := g.Turn.Side
	if b.Outcome() != nc.NoOutcome {
		g.Status, g.EndedAt, g.EndReason = StatusEnded, &now, b.Method().String()
		switch b.Outcome() {
		case nc.WhiteWon:
			g.Result = White
		case nc.BlackWon:
			g.Result = Black
		default:
			g.Result = "draw"
		}
	} else {
		s.pick(g, other(mover), now)
	}
	if err := s.save(ctx, g); err != nil {
		return err
	}
	team := map[string]string{White: "ทีมขาว", Black: "ทีมดำ"}[mover]
	who := name
	if byTeam {
		who = "ทีมเลือก (หมดเวลา)"
	}
	s.announce(ctx, g, fmt.Sprintf("%sเดิน %s · %s", team, san, who))
	if g.Status == StatusEnded {
		s.announce(ctx, g, "เกมจบแล้ว · "+map[string]string{White: "ทีมขาวชนะ", Black: "ทีมดำชนะ", "draw": "เสมอ"}[g.Result])
		_, err := s.Ensure(ctx, g.CohortNumber)
		return err
	}
	return nil
}

func (s *Service) save(ctx context.Context, g *Game) error {
	read := g.Version
	g.Version++
	ok, err := s.store.Save(ctx, g, read)
	if err != nil {
		return err
	}
	if !ok {
		return ErrConflict
	}
	return nil
}

func (s *Service) name(userID string) string {
	if p, err := s.people.Get(userID); err == nil && p != nil {
		return p.Name
	}
	return ""
}

func (s *Service) announce(ctx context.Context, g *Game, text string) {
	_ = s.store.AddMessage(ctx, Message{ID: primitive.NewObjectID().Hex(), GameID: g.ID.Hex(), Room: RoomAll, Text: text, System: true, At: s.now()})
}

func (s *Service) gameFor(ctx context.Context, userID string) (*Person, *Game, error) {
	p, err := s.people.Get(userID)
	if err != nil || p == nil {
		return nil, nil, ErrNoAccess
	}
	g, err := s.Ensure(ctx, p.Cohort)
	if err != nil {
		return nil, nil, err
	}
	if g == nil {
		return p, nil, ErrNoGame
	}
	return p, g, nil
}

// Move plays the picked member's move.
func (s *Service) Move(ctx context.Context, userID, uci string) error {
	_, g, err := s.gameFor(ctx, userID)
	if err != nil {
		return err
	}
	if g.Turn.PickedUserID != userID {
		return ErrNotYourTurn
	}
	return s.play(ctx, g, uci, userID, false)
}

// Propose records (or replaces) a teammate's suggested move for the side to move.
func (s *Service) Propose(ctx context.Context, userID, uci, note string) error {
	_, g, err := s.gameFor(ctx, userID)
	if err != nil {
		return err
	}
	if sideOf(g, userID) != g.Turn.Side {
		return ErrNotYourSide
	}
	b, err := board(g)
	if err != nil {
		return err
	}
	mv, san, err := parse(b, uci)
	if err != nil {
		return err
	}
	if note = strings.TrimSpace(note); len([]rune(note)) > maxNote {
		note = string([]rune(note)[:maxNote])
	}
	kept := g.Proposals[:0]
	for _, p := range g.Proposals {
		if p.UserID != userID {
			kept = append(kept, p)
		}
	}
	g.Proposals = append(kept, Proposal{ID: primitive.NewObjectID().Hex(), UserID: userID, UserName: s.name(userID), UCI: mv.String(), SAN: san, Note: note, Voters: []string{userID}, At: s.now()})
	return s.save(ctx, g)
}

// Vote toggles the caller's agreement with a proposal of their own side.
func (s *Service) Vote(ctx context.Context, userID, proposalID string) error {
	_, g, err := s.gameFor(ctx, userID)
	if err != nil {
		return err
	}
	if sideOf(g, userID) != g.Turn.Side {
		return ErrNotYourSide
	}
	for i, p := range g.Proposals {
		if p.ID != proposalID {
			continue
		}
		if contains(p.Voters, userID) {
			g.Proposals[i].Voters = remove(p.Voters, userID)
		} else {
			g.Proposals[i].Voters = append(p.Voters, userID)
		}
		return s.save(ctx, g)
	}
	return ErrNoGame
}

func canUse(g *Game, p *Person, room string) bool {
	switch room {
	case RoomAll:
		return true
	case RoomWhite, RoomBlack:
		return p.Staff || sideOf(g, p.ID) == room
	}
	return false
}

// Messages returns a room's messages after the cursor (the latest when the cursor is unknown).
func (s *Service) Messages(ctx context.Context, userID, room, after string) ([]Message, error) {
	p, g, err := s.gameFor(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !canUse(g, p, room) {
		return nil, ErrNoAccess
	}
	return s.store.Messages(ctx, g.ID.Hex(), room, after, 100)
}

// Post adds a chat message to a room the caller may use.
func (s *Service) Post(ctx context.Context, userID, room, text string) (*Message, error) {
	p, g, err := s.gameFor(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !canUse(g, p, room) {
		return nil, ErrNoAccess
	}
	if text = strings.TrimSpace(text); text == "" || len([]rune(text)) > maxText {
		return nil, ErrBadText
	}
	m := Message{ID: primitive.NewObjectID().Hex(), GameID: g.ID.Hex(), Room: room, UserID: p.ID, UserName: p.Name, Text: text, At: s.now()}
	return &m, s.store.AddMessage(ctx, m)
}

// Tick starts missing games and handles turns past their deadline.
func (s *Service) Tick(ctx context.Context) error {
	for _, c := range s.cohorts() {
		if _, err := s.Ensure(ctx, c); err != nil {
			return err
		}
	}
	games, err := s.store.AllActive(ctx)
	if err != nil {
		return err
	}
	now := s.now()
	for i := range games {
		g := &games[i]
		if now.Before(g.Turn.Deadline) {
			continue
		}
		if top := TopProposal(g.Proposals); top != nil {
			if err := s.play(ctx, g, top.UCI, top.UserID, true); err != nil && !errors.Is(err, ErrConflict) {
				return err
			}
			continue
		}
		s.pick(g, g.Turn.Side, now)
		if err := s.save(ctx, g); err != nil && !errors.Is(err, ErrConflict) {
			return err
		}
	}
	return nil
}

// End stops the cohort's running game as a draw; the next tick starts a new one.
func (s *Service) End(ctx context.Context, cohort int) error {
	g, err := s.store.Active(ctx, cohort)
	if err != nil || g == nil {
		return err
	}
	now := s.now()
	g.Status, g.EndedAt, g.Result, g.EndReason = StatusEnded, &now, "draw", "ended by staff"
	if err := s.save(ctx, g); err != nil {
		return err
	}
	s.announce(ctx, g, "ทีมงานจบเกมนี้แล้ว (เสมอ)")
	return nil
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

func remove(xs []string, x string) []string {
	out := xs[:0]
	for _, v := range xs {
		if v != x {
			out = append(out, v)
		}
	}
	return out
}

type Member struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Group string `json:"group"`
}

type TeamView struct {
	Groups  []string `json:"groups"`
	Members []Member `json:"members"`
}

// View is the running game as one learner may see it: their own side's proposals only.
type View struct {
	Game         *Game               `json:"game"`
	MySide       string              `json:"my_side"`
	YouArePicked bool                `json:"you_are_picked"`
	PickedName   string              `json:"picked_name"`
	Teams        map[string]TeamView `json:"teams"`
	Proposals    []Proposal          `json:"proposals"`
}

func (s *Service) View(ctx context.Context, userID string) (*View, error) {
	_, g, err := s.gameFor(ctx, userID)
	if errors.Is(err, ErrNoGame) {
		return &View{Proposals: []Proposal{}}, nil
	}
	if err != nil {
		return nil, err
	}
	v := &View{Game: g, MySide: sideOf(g, userID), YouArePicked: g.Turn.PickedUserID == userID, PickedName: s.name(g.Turn.PickedUserID), Teams: map[string]TeamView{}, Proposals: []Proposal{}}
	for side, t := range g.Teams {
		tv := TeamView{Groups: t.Groups}
		for _, id := range t.Members {
			if p, err := s.people.Get(id); err == nil && p != nil {
				tv.Members = append(tv.Members, Member{ID: id, Name: p.Name, Group: p.Group})
			}
		}
		v.Teams[side] = tv
	}
	if v.MySide != "" && v.MySide == g.Turn.Side {
		v.Proposals = g.Proposals
	}
	return v, nil
}

// Me is the cheap "is it my turn" check for the nav dot and the Today card.
func (s *Service) Me(ctx context.Context, userID string) (enabled, myTurn bool) {
	p, err := s.people.Get(userID)
	if err != nil || p == nil {
		return false, false
	}
	on, _ := s.settings(p.Cohort)
	g, err := s.store.Active(ctx, p.Cohort)
	return on, err == nil && g != nil && g.Turn.PickedUserID == userID
}
