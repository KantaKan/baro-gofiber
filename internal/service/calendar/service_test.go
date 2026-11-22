package calendar

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

type memStore struct{ cal *Calendar }

func (m *memStore) Get(cohort int) (*Calendar, error) {
	if m.cal == nil {
		return nil, nil
	}
	cp := *m.cal
	cp.Weeks = append([]Week(nil), m.cal.Weeks...)
	return &cp, nil
}
func (m *memStore) Save(c *Calendar, expect int) error {
	if (m.cal == nil && expect != 0) || (m.cal != nil && m.cal.Version != expect) {
		return ErrConflict
	}
	cp := *c
	cp.Version = expect + 1
	m.cal = &cp
	return nil
}
func (m *memStore) SetPublished(cohort, n int, p bool, at time.Time) (bool, error) {
	if m.cal == nil {
		return false, nil
	}
	for i := range m.cal.Weeks {
		if m.cal.Weeks[i].N == n {
			m.cal.Weeks[i].Published = p
			if p {
				m.cal.Weeks[i].PublishedAt = &at
			} else {
				m.cal.Weeks[i].PublishedAt = nil
			}
			m.cal.Version++
			return true, nil
		}
	}
	return false, nil
}
func (m *memStore) SetZoom(cohort int, url string) error {
	if m.cal == nil {
		return ErrNoCalendar
	}
	m.cal.ZoomURL = url
	m.cal.Version++
	return nil
}

func bookBytes(t *testing.T, f *excelize.File) []byte {
	t.Helper()
	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func newSvc() (*Service, *memStore) {
	st := &memStore{}
	s := NewService(st)
	s.now = func() time.Time { return time.Date(2026, 11, 22, 3, 0, 0, 0, time.UTC) }
	return s, st
}

const start = "2026-06-29"

func TestPreviewSavesNothingAndSaveStores(t *testing.T) {
	s, st := newSvc()
	data := bookBytes(t, fixture(t))
	pv, err := s.Import(13, start, data, sheet, false, "kan")
	if err != nil || pv.Saved || st.cal != nil || pv.Weeks != 3 || pv.Sessions == 0 || len(pv.Warnings) == 0 || pv.HasExisting {
		t.Fatalf("preview: %v %+v", err, pv)
	}
	if len(pv.Diff) != 3 || pv.Diff[0].Status != "new" {
		t.Errorf("first import: every week is new: %+v", pv.Diff)
	}
	pv, err = s.Import(13, start, data, sheet, true, "kan")
	if err != nil || !pv.Saved || st.cal == nil || len(st.cal.Weeks) != 3 || st.cal.Sheet != sheet || st.cal.ImportedBy != "kan" || st.cal.ZoomURL == "" {
		t.Fatalf("save: %v %+v %+v", err, pv, st.cal)
	}
	for _, w := range st.cal.Weeks {
		if w.Published {
			t.Error("imported weeks start unpublished")
		}
	}
}

func TestSheetAutoPick(t *testing.T) {
	s, _ := newSvc()
	f := fixture(t)
	f.NewSheet("JSD12_Ver1")
	data := bookBytes(t, f)
	pv, err := s.Import(13, start, data, "", false, "kan")
	if err != nil || pv.Sheet != sheet || len(pv.NeedsSheet) != 0 {
		t.Fatalf("one JSD13_ sheet is picked automatically: %v %+v", err, pv)
	}
	f.NewSheet("JSD13_Ver2")
	pv, err = s.Import(13, start, bookBytes(t, f), "", false, "kan")
	if err != nil || len(pv.NeedsSheet) != 2 || pv.Weeks != 0 {
		t.Fatalf("two candidates: ask which one: %v %+v", err, pv)
	}
	f2 := excelize.NewFile()
	f2.NewSheet("Other")
	pv, err = s.Import(13, start, bookBytes(t, f2), "", false, "kan")
	if err != nil || len(pv.NeedsSheet) == 0 {
		t.Fatalf("no JSD13 sheet: list the sheets: %v %+v", err, pv)
	}
}

func TestBadFilesAreRefused(t *testing.T) {
	s, st := newSvc()
	for name, data := range map[string][]byte{"garbage": []byte("this is not a spreadsheet"), "empty": nil, "zip-ish": []byte("PK\x03\x04junk")} {
		if _, err := s.Import(13, start, data, sheet, true, "kan"); !errors.Is(err, ErrBadFile) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if st.cal != nil {
		t.Error("nothing saved from a bad file")
	}
	if _, err := s.Import(13, start, bookBytes(t, fixture(t)), "Nope", false, "kan"); err == nil || !strings.Contains(err.Error(), "Nope") {
		t.Errorf("wrong sheet name: %v", err)
	}
	if _, err := s.Import(13, "", bookBytes(t, fixture(t)), sheet, false, "kan"); err == nil {
		t.Error("needs the cohort start date")
	}
}

func TestReimportKeepsPublishedAndShowsTheDiff(t *testing.T) {
	s, st := newSvc()
	_, _ = s.Import(13, start, bookBytes(t, fixture(t)), sheet, true, "kan")
	if err := s.Publish(13, 1, true); err != nil {
		t.Fatal(err)
	}
	// The team edits week 1 (drops the REACT-4 session) and the file is imported again.
	f := fixture(t)
	for _, c := range []string{"B48"} {
		_ = f.UnmergeCell(sheet, "B48", "B58")
		_ = f.SetCellValue(sheet, c, "")
	}
	pv, err := s.Import(13, start, bookBytes(t, f), sheet, true, "kan")
	if err != nil || !pv.HasExisting {
		t.Fatalf("reimport: %v %+v", err, pv)
	}
	var w1, w2 WeekDiff
	for _, d := range pv.Diff {
		if d.N == 1 {
			w1 = d
		}
		if d.N == 2 {
			w2 = d
		}
	}
	if w1.Status != "changed" || w1.Removed != 1 || !w1.Published || len(w1.Lines) == 0 || !strings.Contains(w1.Lines[0], "REACT-4") {
		t.Errorf("week 1 changed and is published: %+v", w1)
	}
	if w2.Status != "same" {
		t.Errorf("week 2 is untouched: %+v", w2)
	}
	if !st.cal.Weeks[0].Published || st.cal.Weeks[0].PublishedAt == nil || st.cal.Weeks[1].Published {
		t.Errorf("published state survives a re-import, per week: %+v", st.cal.Weeks)
	}
}

func TestDiffStatuses(t *testing.T) {
	mk := func(n int, titles ...string) Week {
		d := Day{Date: "2026-06-29", Sessions: []Session{}}
		for i, ti := range titles {
			d.Sessions = append(d.Sessions, Session{Start: hhmm(540 + i*60), End: hhmm(600 + i*60), Title: ti, Kind: "session"})
		}
		return Week{N: n, Days: []Day{d}}
	}
	old := []Week{mk(1, "A", "B"), mk(2, "C"), mk(3, "D")}
	cur := []Week{mk(1, "A", "B"), mk(2, "C", "E"), mk(4, "F")}
	got := map[int]WeekDiff{}
	for _, d := range Diff(old, cur) {
		got[d.N] = d
	}
	if got[1].Status != "same" || got[2].Status != "changed" || got[2].Added != 1 || got[3].Status != "removed" || got[4].Status != "new" || got[4].Added != 1 {
		t.Errorf("diff: %+v", got)
	}
	// A holiday appearing is a change too.
	h := mk(5, "X")
	h2 := mk(5, "X")
	h2.Days[0].Holiday = true
	if d := Diff([]Week{h}, []Week{h2})[0]; d.Status != "changed" || len(d.Lines) == 0 || !strings.Contains(strings.ToLower(d.Lines[0]), "holiday") {
		t.Errorf("holiday change: %+v", d)
	}
}

func TestPublishTogglesOneWeekOnly(t *testing.T) {
	s, st := newSvc()
	_, _ = s.Import(13, start, bookBytes(t, fixture(t)), sheet, true, "kan")
	if err := s.Publish(13, 2, true); err != nil {
		t.Fatal(err)
	}
	if st.cal.Weeks[0].Published || !st.cal.Weeks[1].Published || st.cal.Weeks[2].Published {
		t.Errorf("only week 2: %+v", st.cal.Weeks)
	}
	if err := s.Publish(13, 2, false); err != nil || st.cal.Weeks[1].Published || st.cal.Weeks[1].PublishedAt != nil {
		t.Errorf("unpublish: %v", err)
	}
	if err := s.Publish(13, 99, true); !errors.Is(err, ErrNoWeek) {
		t.Errorf("unknown week: %v", err)
	}
	empty, _ := newSvc()
	if err := empty.Publish(13, 1, true); !errors.Is(err, ErrNoWeek) && !errors.Is(err, ErrNoCalendar) {
		t.Errorf("no calendar yet: %v", err)
	}
}

func TestLearnersSeeOnlyPublishedWeeksAndNothingInternal(t *testing.T) {
	s, _ := newSvc()
	if lc, err := s.ForLearner(13); err != nil || len(lc.Weeks) != 0 {
		t.Fatalf("no calendar yet: %v %+v", err, lc)
	}
	_, _ = s.Import(13, start, bookBytes(t, fixture(t)), sheet, true, "kan")
	if lc, _ := s.ForLearner(13); len(lc.Weeks) != 0 {
		t.Fatal("nothing is published yet, so learners see nothing")
	}
	_ = s.Publish(13, 2, true)
	lc, _ := s.ForLearner(13)
	if len(lc.Weeks) != 1 || lc.Weeks[0].N != 2 || lc.ZoomURL == "" {
		t.Fatalf("learner calendar: %+v", lc)
	}
	raw, _ := json.Marshal(lc)
	for _, bad := range []string{"published", "warnings", "imported", "version", "sheet", "JSD13_Ver1", "internal note"} {
		if strings.Contains(strings.ToLower(string(raw)), strings.ToLower(bad)) {
			t.Errorf("learner JSON must not contain %q: %s", bad, raw)
		}
	}
	if !strings.Contains(string(raw), "facilitators") {
		t.Error("learners do see facilitator names")
	}
}

func TestStaleImportIsRefused(t *testing.T) {
	s, st := newSvc()
	data := bookBytes(t, fixture(t))
	_, _ = s.Import(13, start, data, sheet, true, "kan")
	// Someone publishes a week between this admin's preview and save: the version moves on.
	stale := st.cal.Version
	_ = s.Publish(13, 1, true)
	if _, err := s.ImportAt(13, start, data, sheet, true, "kan", &stale); !errors.Is(err, ErrConflict) {
		t.Errorf("an import based on an old version must not overwrite a newer publish: %v", err)
	}
	if !st.cal.Weeks[0].Published {
		t.Error("the publish switch was lost")
	}
}

func TestZoomLink(t *testing.T) {
	s, st := newSvc()
	_, _ = s.Import(13, start, bookBytes(t, fixture(t)), sheet, true, "kan")
	if err := s.SetZoom(13, "https://zoom.us/j/999"); err != nil || st.cal.ZoomURL != "https://zoom.us/j/999" {
		t.Fatalf("zoom: %v", err)
	}
	for _, bad := range []string{"http://zoom.us/j/1", "javascript:alert(1)", "zoom.us/j/1", "https://" + strings.Repeat("a", 600)} {
		if err := s.SetZoom(13, bad); !errors.Is(err, ErrBadURL) {
			t.Errorf("%q should be refused: %v", bad[:min(len(bad), 20)], err)
		}
	}
	if err := s.SetZoom(13, ""); err != nil || st.cal.ZoomURL != "" {
		t.Errorf("clearing the link: %v", err)
	}
}

func TestAdminSetZoomLinkSurvivesAReimport(t *testing.T) {
	s, st := newSvc()
	data := bookBytes(t, fixture(t))
	_, _ = s.Import(13, start, data, sheet, true, "kan")
	_ = s.SetZoom(13, "https://zoom.us/j/ADMIN")
	if _, err := s.Import(13, start, data, sheet, true, "kan"); err != nil {
		t.Fatal(err)
	}
	if st.cal.ZoomURL != "https://zoom.us/j/ADMIN" {
		t.Errorf("a link the admin set must not be replaced by the sheet's: %q", st.cal.ZoomURL)
	}
}
