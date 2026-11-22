package calendar

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

var (
	ErrBadFile    = errors.New("that doesn't look like an .xlsx spreadsheet")
	ErrConflict   = errors.New("the calendar changed while you were reviewing it; upload the file again to see the latest differences")
	ErrNoCalendar = errors.New("no calendar has been imported for this cohort yet")
	ErrNoWeek     = errors.New("there is no such week in the calendar")
	ErrBadURL     = errors.New("the Zoom link must be an https:// address")
)

// UserError is a problem with what the admin uploaded (safe to show as is).
type UserError struct{ Msg string }

func (e *UserError) Error() string { return e.Msg }

// Calendar is a cohort's saved blueprint. Version changes on every write, so a stale import can't
// overwrite a newer publish.
type Calendar struct {
	Cohort     int       `bson:"cohort" json:"cohort"`
	Sheet      string    `bson:"sheet" json:"sheet"`
	ZoomURL    string    `bson:"zoom_url" json:"zoom_url"`
	Weeks      []Week    `bson:"weeks" json:"weeks"`
	ImportedAt time.Time `bson:"imported_at" json:"imported_at"`
	ImportedBy string    `bson:"imported_by" json:"imported_by"`
	Version    int       `bson:"version" json:"version"`
}

type Store interface {
	Get(cohort int) (*Calendar, error)
	// Save replaces the calendar if its stored version is still expectVersion (0 = none yet).
	Save(c *Calendar, expectVersion int) error
	SetPublished(cohort, n int, published bool, at time.Time) (bool, error)
	SetZoom(cohort int, url string) error
}

type Service struct {
	store Store
	now   func() time.Time
}

func NewService(st Store) *Service { return &Service{store: st, now: time.Now} }

// Preview is what an import shows the admin before (or after) saving.
type Preview struct {
	Sheet       string     `json:"sheet"`
	NeedsSheet  []string   `json:"needs_sheet,omitempty"`
	Weeks       int        `json:"weeks"`
	Sessions    int        `json:"sessions"`
	Warnings    []Warning  `json:"warnings"`
	Diff        []WeekDiff `json:"diff"`
	ZoomURL     string     `json:"zoom_url"`
	HasExisting bool       `json:"has_existing"`
	Saved       bool       `json:"saved"`
	Version     int        `json:"version"`
}

// pickSheet chooses JSD<cohort>_… automatically when exactly one matches; otherwise the admin picks.
func pickSheet(names []string, cohort int, want string) (string, []string) {
	if want != "" {
		return want, nil
	}
	prefix := strings.ToLower(fmt.Sprintf("jsd%d_", cohort))
	var cands []string
	for _, n := range names {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(n)), prefix) {
			cands = append(cands, n)
		}
	}
	if len(cands) == 1 {
		return cands[0], nil
	}
	if len(cands) > 1 {
		return "", cands
	}
	if len(names) > 60 {
		names = names[:60]
	}
	return "", names
}

func (s *Service) Import(cohort int, startDate string, data []byte, sheet string, save bool, by string) (Preview, error) {
	return s.ImportAt(cohort, startDate, data, sheet, save, by, nil)
}

// ImportAt is Import with the calendar version the admin previewed against (nil = whatever is stored now).
func (s *Service) ImportAt(cohort int, startDate string, data []byte, sheet string, save bool, by string, expectVersion *int) (Preview, error) {
	if len(data) == 0 {
		return Preview{}, ErrBadFile
	}
	f, err := excelize.OpenReader(bytes.NewReader(data), excelize.Options{UnzipSizeLimit: 150 << 20, UnzipXMLSizeLimit: 24 << 20})
	if err != nil {
		return Preview{}, ErrBadFile
	}
	defer f.Close()
	chosen, needs := pickSheet(f.GetSheetList(), cohort, sheet)
	existing, err := s.store.Get(cohort)
	if err != nil {
		return Preview{}, err
	}
	pv := Preview{Warnings: []Warning{}, Diff: []WeekDiff{}, HasExisting: existing != nil}
	if existing != nil {
		pv.Version = existing.Version
	}
	if chosen == "" {
		pv.NeedsSheet = needs
		return pv, nil
	}
	parsed, err := ParseSheet(f, chosen, startDate)
	if err != nil {
		return Preview{}, &UserError{Msg: err.Error()}
	}
	pv.Sheet, pv.Weeks, pv.Warnings, pv.ZoomURL = chosen, len(parsed.Weeks), parsed.Warnings, parsed.ZoomURL
	for _, w := range parsed.Weeks {
		pv.Sessions += countSessions(w)
	}
	var old []Week
	if existing != nil {
		old = existing.Weeks
		if pv.ZoomURL == "" {
			pv.ZoomURL = existing.ZoomURL
		}
		// Published state is the admin's decision, not the file's: carry it over per week.
		pub := map[int]Week{}
		for _, w := range old {
			pub[w.N] = w
		}
		for i := range parsed.Weeks {
			if o, ok := pub[parsed.Weeks[i].N]; ok {
				parsed.Weeks[i].Published, parsed.Weeks[i].PublishedAt = o.Published, o.PublishedAt
			}
		}
	}
	pv.Diff = Diff(old, parsed.Weeks)
	if !save {
		return pv, nil
	}
	expect := 0
	if existing != nil {
		expect = existing.Version
	}
	if expectVersion != nil {
		expect = *expectVersion
	}
	cal := &Calendar{Cohort: cohort, Sheet: chosen, ZoomURL: pv.ZoomURL, Weeks: parsed.Weeks, ImportedAt: s.now(), ImportedBy: by}
	if err := s.store.Save(cal, expect); err != nil {
		return Preview{}, err
	}
	pv.Saved, pv.Version = true, expect+1
	return pv, nil
}

// Get returns the saved calendar for the admin (all weeks, with publish state).
func (s *Service) Get(cohort int) (*Calendar, error) { return s.store.Get(cohort) }

// Publish shows or hides one week to learners.
func (s *Service) Publish(cohort, n int, published bool) error {
	ok, err := s.store.SetPublished(cohort, n, published, s.now())
	if err != nil {
		return err
	}
	if !ok {
		return ErrNoWeek
	}
	return nil
}

// SetZoom stores the Join Zoom link ("" clears it).
func (s *Service) SetZoom(cohort int, url string) error {
	url = strings.TrimSpace(url)
	if url != "" && (!strings.HasPrefix(url, "https://") || len(url) > 500 || strings.ContainsAny(url, " \n\r\t")) {
		return ErrBadURL
	}
	return s.store.SetZoom(cohort, url)
}

// What learners get: published weeks only, no flags, warnings or file details.
type LearnerWeek struct {
	N      int    `json:"n"`
	Name   string `json:"name"`
	Monday string `json:"monday"`
	Days   []Day  `json:"days"`
}

type LearnerCalendar struct {
	ZoomURL string        `json:"zoom_url"`
	Weeks   []LearnerWeek `json:"weeks"`
}

func (s *Service) ForLearner(cohort int) (LearnerCalendar, error) {
	out := LearnerCalendar{Weeks: []LearnerWeek{}}
	c, err := s.store.Get(cohort)
	if err != nil || c == nil {
		return out, err
	}
	out.ZoomURL = c.ZoomURL
	for _, w := range c.Weeks {
		if w.Published {
			out.Weeks = append(out.Weeks, LearnerWeek{N: w.N, Name: w.Name, Monday: w.Monday, Days: w.Days})
		}
	}
	return out, nil
}
