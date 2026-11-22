package report

import (
	"errors"
	"strings"
	"testing"
	"time"

	"gofiber-baro/internal/service/wellbeing"
)

type memStore struct {
	m    map[string]*Saved
	lose bool // simulate another sender winning the race
}

func key(c int, w string) string { return w + "/" + string(rune('0'+c%10)) }

func (s *memStore) Get(c int, w string) (*Saved, error) {
	if v, ok := s.m[key(c, w)]; ok {
		cp := *v
		return &cp, nil
	}
	return nil, nil
}
func (s *memStore) SaveNotes(c int, w, notes string) (bool, error) {
	v := s.m[key(c, w)]
	if v != nil && v.Status == StatusSent {
		return false, nil
	}
	if v == nil {
		v = &Saved{Cohort: c, Week: w, Status: StatusDraft}
		s.m[key(c, w)] = v
	}
	v.Notes = notes
	return true, nil
}
func (s *memStore) SetPost(c int, w, errMsg string) error {
	if v := s.m[key(c, w)]; v != nil {
		v.PostError = errMsg
	}
	return nil
}
func (s *memStore) MarkSent(sv *Saved) (bool, error) {
	if s.lose {
		cp := *sv
		s.m[key(sv.Cohort, sv.Week)] = &cp // the other sender's report is what's stored
		return false, nil
	}
	if v := s.m[key(sv.Cohort, sv.Week)]; v != nil && v.Status == StatusSent {
		return false, nil
	}
	cp := *sv
	s.m[key(sv.Cohort, sv.Week)] = &cp
	return true, nil
}

type fakeSource struct{ in Input }

func (f *fakeSource) Load(cohort int, week string) (Input, error) {
	in := f.in
	in.Week = week
	return in, nil
}

type fakePoster struct {
	texts []string
	err   error
	on    bool
}

func (p *fakePoster) PostReport(text string) (bool, error) {
	if p.err != nil {
		return false, p.err
	}
	if !p.on {
		return false, nil
	}
	p.texts = append(p.texts, text)
	return true, nil
}

func newSvc(t *testing.T) (*Service, *memStore, *fakeSource, *fakePoster) {
	t.Helper()
	in, a, _, _ := fixture()
	in.Support = []wellbeing.SupportRow{{Learner: wellbeing.Ref(a), Reasons: []wellbeing.Reason{{Key: "leave", Text: "Leave 6/8"}}}}
	st, src, po := &memStore{m: map[string]*Saved{}}, &fakeSource{in: in}, &fakePoster{on: true}
	s := NewService(st, src, po).WithAppURL("https://baro.example")
	s.now = func() time.Time { return time.Date(2026, 11, 20, 12, 0, 0, 0, time.FixedZone("BKK", 7*3600)) }
	return s, st, src, po
}

func TestWeekValidation(t *testing.T) {
	s, _, _, _ := newSvc(t)
	if _, err := s.Get(13, "2026-11-17"); !errors.Is(err, ErrBadWeek) {
		t.Errorf("a Tuesday is not a week start: %v", err)
	}
	if _, err := s.Get(13, "nonsense"); !errors.Is(err, ErrBadWeek) {
		t.Errorf("garbage: %v", err)
	}
	if _, err := s.Get(13, "2026-11-23"); !errors.Is(err, ErrFutureWeek) {
		t.Errorf("next week isn't reportable yet: %v", err)
	}
	if _, err := s.Get(0, week); err == nil {
		t.Error("cohort 0 accepted")
	}
}

func TestDraftThenNotesThenSent(t *testing.T) {
	s, _, _, po := newSvc(t)
	r, err := s.Get(13, week)
	if err != nil || r.Status != StatusDraft || r.Notes != "" || len(r.Support) != 1 {
		t.Fatalf("draft: %v %+v", err, r)
	}
	if err := s.SaveNotes(13, week, "Great week, retro on Monday"); err != nil {
		t.Fatal(err)
	}
	if r, _ = s.Get(13, week); r.Notes != "Great week, retro on Monday" {
		t.Errorf("notes: %q", r.Notes)
	}
	if err := s.SaveNotes(13, week, strings.Repeat("x", 4001)); !errors.Is(err, ErrNotesTooLong) {
		t.Errorf("long notes: %v", err)
	}
	res, err := s.Send(13, week)
	if err != nil || !res.SentToSlack || res.Status != StatusSent || len(po.texts) != 1 {
		t.Fatalf("send: %v %+v posts=%d", err, res, len(po.texts))
	}
	if err := s.SaveNotes(13, week, "late edit"); !errors.Is(err, ErrSent) {
		t.Errorf("notes after send: %v", err)
	}
	if r, _ = s.Get(13, week); r.Status != StatusSent || r.SentAt == nil || r.Notes != "Great week, retro on Monday" {
		t.Errorf("sent report: %+v", r)
	}
}

func TestSentReportIsImmutable(t *testing.T) {
	s, _, src, po := newSvc(t)
	first, _ := s.Send(13, week)
	before, _ := s.Get(13, week)
	// The data changes afterwards (a record edited, a learner added): the sent report must not.
	src.in.Records = nil
	after, _ := s.Get(13, week)
	if before.KPIs.Attendance.Value != after.KPIs.Attendance.Value || after.KPIs.Attendance.Value == 0 {
		t.Fatalf("a sent report must keep its numbers: %v then %v", before.KPIs.Attendance.Value, after.KPIs.Attendance.Value)
	}
	again, err := s.Send(13, week)
	if err != nil || !again.SentToSlack || len(po.texts) != 2 {
		t.Fatalf("re-send posts the saved report again: %v %+v", err, again)
	}
	if po.texts[0] != po.texts[1] || !first.SentToSlack {
		t.Error("re-send must post the same text")
	}
}

func TestSlackTextIsAggregateOnly(t *testing.T) {
	s, _, _, po := newSvc(t)
	_ = s.SaveNotes(13, week, "Mint struggled this week")
	if _, err := s.Send(13, week); err != nil {
		t.Fatal(err)
	}
	text := po.texts[0]
	for _, bad := range []string{"Mint", "01_A", "Leave 6/8", "struggled"} {
		if strings.Contains(text, bad) {
			t.Errorf("slack text must not contain %q: %s", bad, text)
		}
	}
	for _, want := range []string{"JSD13", "83%", "https://baro.example/admin/report?week=" + week, "1 learner needs support"} {
		if !strings.Contains(text, want) {
			t.Errorf("slack text missing %q: %s", want, text)
		}
	}
}

func TestSendSurvivesSlackProblems(t *testing.T) {
	s, _, _, po := newSvc(t)
	po.err = errors.New("slack: not_in_channel")
	res, err := s.Send(13, week)
	if err != nil || res.SentToSlack || res.SlackError == "" || res.Status != StatusSent {
		t.Fatalf("a Slack failure must not lose the report: %v %+v", err, res)
	}
	s2, _, _, po2 := newSvc(t)
	po2.on = false // Slack not connected / no channel
	res, err = s2.Send(13, week)
	if err != nil || res.SentToSlack || res.SlackError != "" || res.Status != StatusSent {
		t.Fatalf("not connected: %v %+v", err, res)
	}
}

func TestSupportBetterComesFromLastSentReport(t *testing.T) {
	s, st, src, _ := newSvc(t)
	a := src.in.Users[0] // the learner the source reports as needing support
	// Last week was sent with learner A in the support list.
	prevReport := Build(Input{Week: prev, Today: today, Users: src.in.Users, Support: []wellbeing.SupportRow{{Learner: wellbeing.Ref(a)}}})
	st.m[key(13, prev)] = &Saved{Cohort: 13, Week: prev, Status: StatusSent, Snapshot: &prevReport}
	r, _ := s.Get(13, week)
	if len(r.Support) != 1 || r.Support[0].Status != "still" {
		t.Fatalf("A was in last week's list and still is: %+v", r.Support)
	}
}

func TestRaceLoserDoesNotPostToSlack(t *testing.T) {
	s, st, _, po := newSvc(t)
	st.lose = true
	res, err := s.Send(13, week)
	if err != nil || !res.AlreadySent || res.SentToSlack || len(po.texts) != 0 {
		t.Fatalf("the loser of a send race must not post: %v %+v posts=%d", err, res, len(po.texts))
	}
}

func TestAutoSendPostsOnceAndRetriesOnlyFailures(t *testing.T) {
	s, st, _, po := newSvc(t)
	po.err = errors.New("slack: not_in_channel")
	res, err := s.AutoSend(13, week)
	if err != nil || res.SlackError == "" || res.SentToSlack {
		t.Fatalf("first run records the failure: %v %+v", err, res)
	}
	if st.m[key(13, week)].PostError == "" || st.m[key(13, week)].Status != StatusSent {
		t.Fatalf("report is saved as sent with the post error: %+v", st.m[key(13, week)])
	}
	po.err = nil
	if res, _ = s.AutoSend(13, week); !res.SentToSlack || len(po.texts) != 1 || st.m[key(13, week)].PostError != "" {
		t.Fatalf("the failed post is retried and the error cleared: %+v posts=%d", res, len(po.texts))
	}
	if res, _ = s.AutoSend(13, week); !res.AlreadySent || len(po.texts) != 1 {
		t.Fatalf("after it is posted, the job never posts again: %+v posts=%d", res, len(po.texts))
	}
}

func TestAutoSendWhenSlackIsNotSetUpDoesNotRetryForever(t *testing.T) {
	s, _, _, po := newSvc(t)
	po.on = false
	_, _ = s.AutoSend(13, week)
	if res, _ := s.AutoSend(13, week); !res.AlreadySent {
		t.Errorf("no Slack configured is not a failure to retry: %+v", res)
	}
}

func TestOldUnsentWeekCannotBeSent(t *testing.T) {
	s, st, _, _ := newSvc(t) // "now" is Friday 20 Nov: this week 16 Nov, last week 9 Nov
	if _, err := s.Send(13, "2026-10-26"); !errors.Is(err, ErrTooOld) {
		t.Errorf("an old never-sent week would freeze today's data as that week: %v", err)
	}
	if _, err := s.Send(13, prev); err != nil {
		t.Errorf("last week may still be sent: %v", err)
	}
	r := Build(Input{Week: "2026-10-26", Today: today})
	st.m[key(13, "2026-10-26")] = &Saved{Cohort: 13, Week: "2026-10-26", Status: StatusSent, Snapshot: &r}
	if _, err := s.Send(13, "2026-10-26"); err != nil {
		t.Errorf("an already-sent old week can be re-posted: %v", err)
	}
}
