package wellbeing

import (
	"testing"

	"gofiber-baro/internal/domain"
	"gofiber-baro/internal/service/attendance"
)

func refl(day, label, text string) domain.Reflection {
	return domain.Reflection{Day: day, ReflectionData: domain.ReflectionContent{Barometer: label, TechSessions: domain.SessionDetails{Happy: text}}}
}

const (
	C = "Comfort Zone"
	E = "Stretch zone - Enjoying the challenges"
	O = "Stretch zone - Overwhelmed"
	P = "Panic Zone"
)

func keys(ps []Pattern) map[string]bool {
	m := map[string]bool{}
	for _, p := range ps {
		m[p.Key] = true
	}
	return m
}

func TestNightsSortsAndDedupes(t *testing.T) {
	n := Nights([]domain.Reflection{refl("2026-09-09", P, ""), refl("2026-09-08", C, ""), refl("2026-09-09", O, "later")})
	if len(n) != 2 || n[0].Day != "2026-09-08" || n[1].Zone != "stretch-overwhelmed" {
		t.Fatalf("nights = %+v", n)
	}
}

func TestWorthALook(t *testing.T) {
	recent := []string{"2026-09-10", "2026-09-11"}
	// Fri + Mon panic is a run (order, not calendar days)
	run := Nights([]domain.Reflection{refl("2026-09-04", C, ""), refl("2026-09-07", P, ""), refl("2026-09-11", P, "")})
	if !keys(WorthALook(run, []string{"2026-09-07", "2026-09-11"}, DefaultWords))["panic_run"] {
		t.Error("panic two nights in a row (across a gap) → panic_run")
	}
	var rough []domain.Reflection
	for i, z := range []string{O, P, O, C, O, E, P, O, C, C} {
		rough = append(rough, refl("2026-08-"+string(rune('1'+i/9))+string(rune('0'+i%10)), z, ""))
	}
	if !keys(WorthALook(Nights(rough), nil, DefaultWords))["rough_stretch"] {
		t.Error("6 of 10 overwhelmed/panic → rough_stretch")
	}
	drop := Nights([]domain.Reflection{refl("2026-09-01", C, ""), refl("2026-09-02", C, ""), refl("2026-09-03", E, ""), refl("2026-09-04", C, ""), refl("2026-09-07", C, ""), refl("2026-09-08", O, "")})
	if !keys(WorthALook(drop, []string{"2026-09-07", "2026-09-08"}, DefaultWords))["drop"] {
		t.Error("comfort then overwhelmed → drop")
	}
	if k := keys(WorthALook(nil, recent, DefaultWords)); !k["silent"] || len(k) != 1 {
		t.Errorf("no reflections → only silent, got %v", k)
	}
	thai := Nights([]domain.Reflection{refl("2026-09-10", E, ""), refl("2026-09-11", E, "วันนี้ไม่ไหวแล้วจริงๆ")})
	if !keys(WorthALook(thai, recent, DefaultWords))["words"] {
		t.Error("Thai watch word without spaces → words")
	}
	if got := Highlight("I feel LOST and want to quit", DefaultWords); len(got) != 2 {
		t.Errorf("highlight = %v", got)
	}
	unknown := Nights([]domain.Reflection{refl("2026-09-10", "???", ""), refl("2026-09-11", "", "")})
	if len(unknown) != 2 || unknown[0].Zone != "" {
		t.Errorf("unknown labels keep the night with empty zone: %+v", unknown)
	}
}

func TestSupportReasons(t *testing.T) {
	d := func(date string, s domain.AttendanceStatus) attendance.DayResult {
		return attendance.DayResult{Date: date, Status: s}
	}
	r := SupportReasons(6, nil, nil, 0)
	if len(r) != 1 || r[0].Key != "leave" {
		t.Errorf("leave 6 → leave, got %+v", r)
	}
	late := []attendance.DayResult{d("2026-09-07", domain.StatusPresent), d("2026-09-08", domain.StatusLate), d("2026-09-09", domain.StatusLate), d("2026-09-10", domain.StatusLate)}
	if r := SupportReasons(0, late, nil, 0); len(r) != 1 || r[0].Key != "late_run" {
		t.Errorf("3 lates in a row → late_run, got %+v", r)
	}
	r = SupportReasons(0, nil, []Pattern{{Key: "panic_run"}}, 2)
	if len(r) != 2 || r[0].Key != "pattern:panic_run" || r[1].Key != "plan" {
		t.Errorf("pattern + plan, got %+v", r)
	}
}

func TestWatchWordsNoFalsePositives(t *testing.T) {
	recent := []string{"2026-09-10", "2026-09-11"}
	for _, txt := range []string{"เหนื่อยแต่สนุกมาก", "ปวดท้องนิดหน่อย ท้องฟ้าสวย", "I can't wait for demo day", "behind the scenes of React"} {
		n := Nights([]domain.Reflection{refl("2026-09-10", E, ""), refl("2026-09-11", E, txt)})
		if keys(WorthALook(n, recent, DefaultWords))["words"] {
			t.Errorf("ordinary text flagged: %q", txt)
		}
	}
	esc := Nights([]domain.Reflection{refl("2026-09-10", E, ""), refl("2026-09-11", E, "I can&#39;t keep up, I want to give up")})
	if !keys(WorthALook(esc, recent, DefaultWords))["words"] || esc[1].Text != "I can't keep up, I want to give up" {
		t.Errorf("stored HTML-escaped text must be unescaped and matched: %q", esc[1].Text)
	}
}

func TestPatternsAreTiedToRecentClassNights(t *testing.T) {
	// panic Mon, nothing Tue-Thu, panic Fri: not "two nights in a row"
	gap := Nights([]domain.Reflection{refl("2026-09-07", P, ""), refl("2026-09-11", P, "")})
	if keys(WorthALook(gap, []string{"2026-09-10", "2026-09-11"}, DefaultWords))["panic_run"] {
		t.Error("panic on non-consecutive class nights must not be a run")
	}
	// worrying words from weeks ago don't count as last night
	old := Nights([]domain.Reflection{refl("2026-08-20", E, "I want to quit")})
	if keys(WorthALook(old, []string{"2026-09-10", "2026-09-11"}, DefaultWords))["words"] {
		t.Error("old words must not show as last night's")
	}
}

func TestHighlightWithNoMatchesIsEmptyNotNull(t *testing.T) {
	got := Highlight("Hooks clicked today", []string{"lost"})
	if got == nil || len(got) != 0 {
		t.Fatalf("no watch words must encode as [] not null, got %#v", got)
	}
}
