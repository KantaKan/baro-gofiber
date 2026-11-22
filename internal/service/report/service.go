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
)

// Saved is the stored side of a week: the team's notes, and the frozen report once it's sent.
type Saved struct {
	Cohort   int        `bson:"cohort"`
	Week     string     `bson:"week"`
	Notes    string     `bson:"notes"`
	Status   string     `bson:"status"`
	SentAt   *time.Time `bson:"sent_at,omitempty"`
	Snapshot *Report    `bson:"snapshot,omitempty"`
}

type Store interface {
	Get(cohort int, week string) (*Saved, error)
	// SaveNotes stores notes for a week; false (no error) when the week was already sent.
	SaveNotes(cohort int, week, notes string) (bool, error)
	// MarkSent freezes a report; false (no error) when the week was already sent.
	MarkSent(s *Saved) (bool, error)
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

func (s *Service) check(cohort int, week string) error {
	if cohort <= 0 {
		return ErrBadCohort
	}
	t, err := time.Parse("2006-01-02", week)
	if err != nil || t.Weekday() != time.Monday {
		return ErrBadWeek
	}
	bkk := s.now().In(time.FixedZone("BKK", 7*3600))
	today := time.Date(bkk.Year(), bkk.Month(), bkk.Day(), 0, 0, 0, 0, time.UTC)
	monday := today.AddDate(0, 0, -((int(today.Weekday()) + 6) % 7))
	if t.After(monday) {
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
}

// Send freezes the week's report (first time only) and posts it to Slack. Sending again re-posts the saved report.
func (s *Service) Send(cohort int, week string) (SendResult, error) {
	if err := s.check(cohort, week); err != nil {
		return SendResult{}, err
	}
	saved, err := s.store.Get(cohort, week)
	if err != nil {
		return SendResult{}, err
	}
	if saved == nil || saved.Status != StatusSent || saved.Snapshot == nil {
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
		if !won { // someone else (or the Friday job) sent it a moment ago: post theirs
			if saved, err = s.store.Get(cohort, week); err != nil || saved == nil || saved.Snapshot == nil {
				return SendResult{}, errors.New("couldn't load the sent report")
			}
		} else {
			saved = &Saved{Snapshot: &r}
		}
	}
	res := SendResult{Status: StatusSent}
	posted, err := s.poster.PostReport(SlackText(*saved.Snapshot, cohort, s.link(week)))
	if err != nil {
		res.SlackError = err.Error()
		return res, nil
	}
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
