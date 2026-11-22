package report

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	StatusDraft = "draft"
	StatusSent  = "sent"
	maxNotes    = 4000
)

var (
	ErrBadWeek      = errors.New("week must be a Monday (YYYY-MM-DD)")
	ErrFutureWeek   = errors.New("that week hasn't started yet")
	ErrSent         = errors.New("this report was already sent, so its notes are locked")
	ErrNotesTooLong = errors.New("notes are too long (4000 characters at most)")
	ErrBadCohort    = errors.New("invalid cohort")
	ErrTooOld       = errors.New("only this week or last week can be sent for the first time")
)

// Saved is the stored side of a week: the team's notes, and the frozen report once it's sent.
type Saved struct {
	Cohort   int        `bson:"cohort"`
	Week     string     `bson:"week"`
	Notes    string     `bson:"notes"`
	Status   string     `bson:"status"`
	SentAt   *time.Time `bson:"sent_at,omitempty"`
	Snapshot *Report    `bson:"snapshot,omitempty"`
	// PostError is the last Slack failure for this report ("" = posted, or Slack wasn't set up).
	PostError string `bson:"post_error,omitempty"`
}

type Store interface {
	Get(cohort int, week string) (*Saved, error)
	// SaveNotes stores notes for a week; false (no error) when the week was already sent.
	SaveNotes(cohort int, week, notes string) (bool, error)
	// MarkSent freezes a report; false (no error) when the week was already sent.
	MarkSent(s *Saved) (bool, error)
	// SetPost records the outcome of the Slack post ("" = fine).
	SetPost(cohort int, week, errMsg string) error
}

// Source loads the live data for a week (learners, records, holidays, support, Cup).
type Source interface {
	Load(cohort int, week string) (Input, error)
}

// Poster sends text to the Slack report channel. posted=false with no error means "Slack isn't set up".
type Poster interface {
	PostReport(text string) (posted bool, err error)
}

type Service struct {
	store  Store
	src    Source
	poster Poster
	now    func() time.Time
	appURL string
}

func NewService(st Store, src Source, p Poster) *Service {
	return &Service{store: st, src: src, poster: p, now: time.Now}
}

// WithAppURL sets the link used in Slack posts.
func (s *Service) WithAppURL(u string) *Service {
	s.appURL = strings.TrimRight(u, "/")
	return s
}

// currentMonday is this week's Monday in Bangkok.
func (s *Service) currentMonday() time.Time {
	bkk := s.now().In(time.FixedZone("BKK", 7*3600))
	today := time.Date(bkk.Year(), bkk.Month(), bkk.Day(), 0, 0, 0, 0, time.UTC)
	return today.AddDate(0, 0, -((int(today.Weekday()) + 6) % 7))
}

func (s *Service) check(cohort int, week string) error {
	if cohort <= 0 {
		return ErrBadCohort
	}
	t, err := time.Parse("2006-01-02", week)
	if err != nil || t.Weekday() != time.Monday {
		return ErrBadWeek
	}
	if t.After(s.currentMonday()) {
		return ErrFutureWeek
	}
	return nil
}

func prevWeek(week string) string {
	return parseDay(week).AddDate(0, 0, -7).Format("2006-01-02")
}

// Get returns the frozen report for a sent week, otherwise a live draft.
func (s *Service) Get(cohort int, week string) (Report, error) {
	if err := s.check(cohort, week); err != nil {
		return Report{}, err
	}
	saved, err := s.store.Get(cohort, week)
	if err != nil {
		return Report{}, err
	}
	if saved != nil && saved.Status == StatusSent && saved.Snapshot != nil {
		r := *saved.Snapshot
		r.Notes, r.Status, r.SentAt = saved.Notes, StatusSent, saved.SentAt
		return r, nil
	}
	return s.build(cohort, week, saved)
}

func (s *Service) build(cohort int, week string, saved *Saved) (Report, error) {
	in, err := s.src.Load(cohort, week)
	if err != nil {
		return Report{}, err
	}
	in.Week = week
	if saved != nil {
		in.Notes = saved.Notes
	}
	if p, err := s.store.Get(cohort, prevWeek(week)); err == nil && p != nil && p.Status == StatusSent && p.Snapshot != nil {
		in.PrevSupport = map[string]bool{}
		for _, it := range p.Snapshot.Support {
			if it.Status != "better" {
				in.PrevSupport[it.Learner.ID] = true
			}
		}
	}
	r := Build(in)
	r.Status = StatusDraft
	return r, nil
}

// SaveNotes stores the team's notes; they lock when the report is sent.
func (s *Service) SaveNotes(cohort int, week, notes string) error {
	if err := s.check(cohort, week); err != nil {
		return err
	}
	if len([]rune(notes)) > maxNotes {
		return ErrNotesTooLong
	}
	ok, err := s.store.SaveNotes(cohort, week, notes)
	if err != nil {
		return err
	}
	if !ok {
		return ErrSent
	}
	return nil
}

type SendResult struct {
	Status      string `json:"status"`
	SentToSlack bool   `json:"sent_to_slack"`
	SlackError  string `json:"slack_error,omitempty"`
	// AlreadySent: nothing was posted because the report had already gone out (or another sender won the race).
	AlreadySent bool `json:"already_sent,omitempty"`
}

// Send freezes the week's report (first time only) and posts it to Slack. Pressing Send on a week that was
// already sent re-posts the saved report; the numbers never change.
func (s *Service) Send(cohort int, week string) (SendResult, error) {
	return s.send(cohort, week, false)
}

// AutoSend is the Friday job's send: it never re-posts a report that already went out, but does retry a
// post that failed.
func (s *Service) AutoSend(cohort int, week string) (SendResult, error) {
	return s.send(cohort, week, true)
}

func (s *Service) send(cohort int, week string, auto bool) (SendResult, error) {
	if err := s.check(cohort, week); err != nil {
		return SendResult{}, err
	}
	saved, err := s.store.Get(cohort, week)
	if err != nil {
		return SendResult{}, err
	}
	if saved != nil && saved.Status == StatusSent && saved.Snapshot != nil {
		if auto && saved.PostError == "" {
			return SendResult{Status: StatusSent, AlreadySent: true}, nil
		}
		return s.post(cohort, week, saved.Snapshot)
	}
	// First send: freezes today's data as this week's report, so only recent weeks qualify.
	if week < prevWeek(s.currentMonday().Format("2006-01-02")) {
		return SendResult{}, ErrTooOld
	}
	r, err := s.build(cohort, week, saved)
	if err != nil {
		return SendResult{}, err
	}
	now := s.now()
	r.Status, r.SentAt = StatusSent, &now
	won, err := s.store.MarkSent(&Saved{Cohort: cohort, Week: week, Notes: r.Notes, Status: StatusSent, SentAt: &now, Snapshot: &r})
	if err != nil {
		return SendResult{}, err
	}
	if !won { // another sender (a double click, the Friday job, another server) got there first; they post
		return SendResult{Status: StatusSent, AlreadySent: true}, nil
	}
	return s.post(cohort, week, &r)
}

// post sends the saved report to Slack and remembers whether that worked. A Slack problem never loses the report.
func (s *Service) post(cohort int, week string, r *Report) (SendResult, error) {
	res := SendResult{Status: StatusSent}
	posted, err := s.poster.PostReport(SlackText(*r, cohort, s.link(week)))
	if err != nil {
		res.SlackError = err.Error()
		_ = s.store.SetPost(cohort, week, err.Error())
		return res, nil
	}
	_ = s.store.SetPost(cohort, week, "")
	res.SentToSlack = posted
	return res, nil
}

func (s *Service) link(week string) string {
	if s.appURL == "" {
		return ""
	}
	return s.appURL + "/admin/report?week=" + week
}

// SlackText is the channel message: numbers only (no learner names, no notes) plus a link.
func SlackText(r Report, cohort int, link string) string {
	a, b := parseDay(r.Week), parseDay(r.WeekEnd)
	span := fmt.Sprintf("%d–%d %s", a.Day(), b.Day(), b.Format("Jan"))
	if a.Month() != b.Month() {
		span = fmt.Sprintf("%d %s–%d %s", a.Day(), a.Format("Jan"), b.Day(), b.Format("Jan"))
	}
	text := fmt.Sprintf("📊 *Weekly report · JSD%d · %s*\n%s\nAttendance %d%% · Late check-ins %d · Reflections sent %d%% · Overwhelmed or panic %d%%",
		cohort, span, r.Summary, pct(r.KPIs.Attendance.Value), int(r.KPIs.Lates.Value), pct(r.KPIs.Reflections.Value), pct(r.KPIs.Rough.Value))
	if link != "" {
		text += "\nFull report: " + link
	}
	return text
}
