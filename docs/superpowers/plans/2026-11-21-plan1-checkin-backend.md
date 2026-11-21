# Plan 1 — Self Check-in Backend Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Learners check themselves in (no code) under per-cohort time rules and network limits; the server auto-marks absences, computes streak / leave usage / warnings, stores check-in-style + avatar preferences, and keeps private data private.

**Architecture:** All rules live in small **pure functions** in `internal/service/attendance/rules.go`, `progress.go` and `internal/service/user/unlocks.go`, unit-tested with table tests and no database. Services wire those functions to the existing Mongo repositories; handlers expose new endpoints next to the old ones. A once-a-minute goroutine closes each session.

**Tech Stack:** Go 1.24 module (`go 1.26` toolchain installed), Fiber v2, mongo-driver, standard `testing`.

**Spec:** `react-genbaro/docs/superpowers/specs/2026-11-21-learner-redesign-design.md` (§3, §5, §6, §7, §9). Mockups in `…/2026-11-21-learner-redesign-mockups/`.

**Where this sits (5 plans):**
1. **This plan — check-in backend** (repo `baro-gofiber`).
2. Frontend shell + design tokens (light/dark) + **Today** + **My attendance** (repo `react-genbaro`) — consumes this plan's endpoints.
3. **Check-in styles** (clean, fun, unlockable, random mode, Victory Lap hook).
4. **My profile, Classmates + avatar editor, Talk Board** restyle.
5. **GenMate Cup** (backend points + frontend cards).
Plans 2–5 are written after this one merges, against its real API.

## Global Constraints

- Timezone: Asia/Bangkok via `utils.GetThailandTime()`; dates as `"2006-01-02"`.
- Morning: opens **8:30**, Present while `now ≤ 9:15:00`, Late while `now ≤ 10:30:00`, then Absent.
- Afternoon: opens **12:30** (Claude's call: 30 min before start, mirroring morning), Present while `now ≤ 13:15:00`, Late while `now ≤ 14:30:00`, then Absent.
- Rule times are per cohort, defaulting to the above.
- No attendance code for learners. Learners must no longer be able to read the active code.
- Network limit: per-cohort list of CIDRs/IPs; **empty list = any network allowed**.
- Leave allowance **8** days; warning levels by days used: `≥4 first`, `≥6 second`, `≥7 third`, else `none`.
- Privacy: classmates never receive attendance %, records, leave, warnings, streak count, reflections. `ToSafe()` may gain `avatar` and `skills` only.
- Check-in style ids (exact): `slide hold ring run coffee red cat bubble scratch alarm slot gacha rocket gold arcade dj`. Unlocks: `rocket` 15-day streak, `arcade` 20, `dj` 30, `gold` = has a perfect week. All others free.
- Avatar styles (exact): `notionists lorelei adventurer fun-emoji pixel-art thumbs`. Locked avatar options: notionists `bodyIcon` `saturn` (15-day), `galaxy` (30-day).
- Work on branch `feat/self-checkin`. Run `go build ./... && go test ./...` before each commit.

## Review Focus

1. **Exact boundary seconds** (9:15:00 vs 9:15:01, 10:30:00 vs 10:30:01): users argue about these; must match the constraint above. → Task 1 tests.
2. **Learner with approved *late* leave who then arrives on time** must be Present, not Late-Excused; arriving late must be Late-Excused, not Late. → Task 3 tests.
3. **Holiday / no-class day**: check-in refused and the closing job writes nothing. → Task 3 and Task 4 tests.
4. **Closing job runs twice** (restart, two pods): must not create duplicate absent records. → Task 4 test + upsert-by-(user,date,session).
5. **Learner reading a classmate** via `GET /users/:id` must not see the new private summary or preferences beyond avatar/skills. → Task 6 test.

---

### Task 1: Session rules engine

**Files:**
- Create: `internal/service/attendance/rules.go`
- Test: `internal/service/attendance/rules_test.go`

**Interfaces:**
- Produces:
  - `type Clock struct{ H, M int }` with `func (c Clock) On(day time.Time) time.Time` (same date/location, seconds 0).
  - `type SessionWindow struct{ OpenAt, LateAfter, AbsentAfter Clock }` (bson/json tags `open_at`, `late_after`, `absent_after`).
  - `type CheckinRules struct{ Morning, Afternoon SessionWindow }` (tags `morning`, `afternoon`).
  - `func DefaultRules() CheckinRules`
  - `func (r CheckinRules) Validate() error` — each window `OpenAt < LateAfter < AbsentAfter`, and `Morning.AbsentAfter < Afternoon.OpenAt`.
  - `func (r CheckinRules) SessionAt(now time.Time) domain.AttendanceSession` — afternoon iff `now ≥ Afternoon.OpenAt`.
  - `type Phase int` consts `PhaseNotOpen, PhaseOnTime, PhaseLate, PhaseClosed`.
  - `func (w SessionWindow) PhaseAt(now time.Time) Phase`
  - `func CombineDayStatus(morning, afternoon domain.AttendanceStatus) domain.AttendanceStatus`

- [ ] **Step 1: Write failing tests** in `rules_test.go`:

```go
var bkk, _ = time.LoadLocation("Asia/Bangkok")
func at(h, m, s int) time.Time { return time.Date(2026, 9, 8, h, m, s, 0, bkk) }

func TestPhaseAt_MorningBoundaries(t *testing.T) {
	w := DefaultRules().Morning
	cases := []struct{ t time.Time; want Phase }{
		{at(8, 29, 59), PhaseNotOpen}, {at(8, 30, 0), PhaseOnTime},
		{at(9, 15, 0), PhaseOnTime}, {at(9, 15, 1), PhaseLate},
		{at(10, 30, 0), PhaseLate}, {at(10, 30, 1), PhaseClosed},
	}
	for _, c := range cases { if got := w.PhaseAt(c.t); got != c.want { t.Errorf("%v: got %v want %v", c.t, got, c.want) } }
}
func TestPhaseAt_AfternoonBoundaries(t *testing.T) { /* same shape: 12:29:59 NotOpen, 12:30 OnTime, 13:15:00 OnTime, 13:15:01 Late, 14:30:00 Late, 14:30:01 Closed */ }
func TestSessionAt(t *testing.T) { /* 11:59 → morning, 12:30 → afternoon */ }
func TestValidate(t *testing.T) { /* DefaultRules() ok; LateAfter before OpenAt → error; Morning.AbsentAfter 13:00 with Afternoon.OpenAt 12:30 → error */ }
func TestCombineDayStatus_SheetTable(t *testing.T) {
	// Every row of the "TH-Status Criteria" sheet (morning, afternoon → day). Short names:
	P, A, AE, H, L, LE, N := domain.StatusPresent, domain.StatusAbsent, domain.StatusAbsentExcused, domain.StatusHoliday, domain.StatusLate, domain.StatusLateExcused, domain.StatusNoClass
	rows := [][3]domain.AttendanceStatus{
		{P, P, P}, {P, A, A}, {P, AE, AE}, {P, H, H}, {P, L, L}, {P, LE, LE}, {P, N, P},
		{A, P, A}, {A, A, A}, {A, AE, A}, {A, H, H}, {A, L, A}, {A, LE, A}, {A, N, A},
		{AE, P, AE}, {AE, A, AE}, {AE, AE, AE}, {AE, H, H}, {AE, L, AE}, {AE, LE, AE}, {AE, N, AE},
		{L, P, L}, {L, A, A}, {L, AE, AE}, {L, H, H}, {L, L, L}, {L, LE, L}, {L, N, L},
		{LE, P, LE}, {LE, A, A}, {LE, AE, AE}, {LE, H, H}, {LE, L, L}, {LE, LE, LE}, {LE, N, LE},
		{N, P, P}, {N, A, A}, {N, AE, AE}, {N, H, H}, {N, L, L}, {N, LE, LE}, {N, N, N},
		{H, P, H}, {H, A, H}, {H, AE, H}, {H, H, H}, {H, L, H}, {H, LE, H}, {H, N, H},
	}
	for _, r := range rows { if got := CombineDayStatus(r[0], r[1]); got != r[2] { t.Errorf("%s+%s: got %s want %s", r[0], r[1], got, r[2]) } }
	if got := CombineDayStatus(domain.StatusDropout, domain.StatusPresent); got != domain.StatusDropout { t.Errorf("dropout wins") }
	if got := CombineDayStatus(domain.StatusDismissed, domain.StatusPresent); got != domain.StatusDismissed { t.Errorf("dismissed wins") }
	if got := CombineDayStatus("", domain.StatusPresent); got != domain.StatusPresent { t.Errorf("missing morning treated as no_class") }
}
```

- [ ] **Step 2: Run** `go test ./internal/service/attendance/ -run 'Phase|Session|Validate|Combine' -v` → FAIL (undefined: DefaultRules…).

- [ ] **Step 3: Implement `rules.go`.** `CombineDayStatus`: dropout/dismissed on either side win; empty string is treated as `no_class`; otherwise a lookup map copied from the test table (keep it a literal map, not clever precedence).

- [ ] **Step 4: Run** the same command → PASS. Also `go build ./...`.

- [ ] **Step 5: Commit** `git add internal/service/attendance/rules*.go && git commit -m "feat(attendance): session rules engine and day-status table"`

---

### Task 2: Cohort settings (rules + allowed networks)

**Files:**
- Create: `internal/domain/cohort_settings.go`, `internal/repository/cohort_settings_repo.go`, `internal/service/cohort/service.go`, `internal/service/cohort/network.go`, `internal/handler/cohort_handler.go`
- Test: `internal/service/cohort/network_test.go`, `internal/service/cohort/service_test.go`
- Modify: `cmd/container.go` (repo + service + handler), `cmd/routes.go`

**Interfaces:**
- Consumes: `attendance.CheckinRules`, `DefaultRules`, `Validate` (Task 1).
- Produces:
  - `domain.CohortSettings{ CohortNumber int; Rules attendance.CheckinRules; AllowedNetworks []string; GroupChatURLs map[string]string }` — collection `cohort_settings`, unique on `cohort_number`. *(If importing `attendance` from `domain` makes a cycle, move `Clock/SessionWindow/CheckinRules` into `domain` and alias them in `attendance`; the names stay the same.)*
  - `func IPAllowed(allowed []string, ip string) bool` — empty → true; entries are CIDRs or single IPs; unparsable entries ignored.
  - `(*cohort.Service) Get(cohort int) (domain.CohortSettings, error)` — missing doc → settings with `DefaultRules()` and empty networks (never an error).
  - `(*cohort.Service) Update(cohort int, s domain.CohortSettings) error` — rejects invalid rules and unparsable network entries.
  - Routes: `GET /admin/cohorts/:cohort/settings`, `PUT /admin/cohorts/:cohort/settings`.

- [ ] **Step 1: Failing tests.** `TestIPAllowed`: `(nil,"1.2.3.4")→true`; `(["10.0.0.0/8"],"10.4.5.6")→true`; `(["10.0.0.0/8"],"11.0.0.1")→false`; `(["203.0.113.7"],"203.0.113.7")→true`; `(["garbage","10.0.0.0/8"],"10.0.0.1")→true`; IPv6 `(["2001:db8::/32"],"2001:db8::1")→true`. `TestUpdateRejects` with a fake repo (map in test file): bad rules → error, `"not-an-ip"` network → error, valid → stored. `TestGetDefaults`: empty repo → `Rules == DefaultRules()`.
- [ ] **Step 2: Run** `go test ./internal/service/cohort/ -v` → FAIL.
- [ ] **Step 3: Implement** using `net.ParseCIDR` / `net.ParseIP`; repo interface `FindByCohort(ctx, int) (*domain.CohortSettings, error)` (nil,nil when absent) and `Upsert(ctx, *domain.CohortSettings) error`.
- [ ] **Step 4: Run** tests → PASS; `go build ./...`.
- [ ] **Step 5: Commit** `feat(cohort): per-cohort check-in rules and allowed networks`.

---

### Task 3: Code-free check-in endpoint

**Files:**
- Create: `internal/service/attendance/checkin.go`
- Test: `internal/service/attendance/checkin_test.go`
- Modify: `internal/domain/attendance.go` (add `CheckinStyle string \`bson:"checkin_style,omitempty" json:"checkin_style,omitempty"\`` to `AttendanceRecord`), `internal/handler/attendance_handler.go` (new `CheckIn`), `cmd/routes.go`, `cmd/container.go`

**Interfaces:**
- Consumes: Task 1 rules; `cohort.Service.Get`, `cohort.IPAllowed` (Task 2); existing holiday + leave repos; `recordRepo.CountRecords` / `InsertRecord`.
- Produces:
  - Errors: `ErrNoClassToday`, `ErrNotOpenYet`, `ErrCheckInClosed`, `ErrCoveredByLeave`, `ErrNetworkNotAllowed` (+ existing `ErrAlreadySubmitted`).
  - `type CheckInInput struct{ Now time.Time; Rules CheckinRules; IsClassDay, AlreadyCheckedIn, IPAllowed bool; Leave domain.AttendanceStatus /* "" | absent_excused | late_excused */ }`
  - `type CheckInDecision struct{ Session domain.AttendanceSession; Status domain.AttendanceStatus }`
  - `func DecideCheckIn(in CheckInInput) (CheckInDecision, error)` — order: no class → not open → closed → network → already → leave.
  - `(*CheckinService) CheckIn(userID primitive.ObjectID, ip, style string) (*domain.AttendanceRecord, error)` — loads user, settings, holidays, approved leave for (date, session), builds input with `utils.GetThailandTime()`, inserts record with `MarkedBy: self`, `CheckinStyle: style`.
  - `POST /attendance/check-in` body `{ "style": "slide" }` → 201 record; errors → 409 (already / covered), 403 (network), 422 (not open / closed / no class) with plain English messages.
  - Remove `student.Get("/code", …)` from the learner group (admins keep `/admin/attendance/active-code`). Keep `POST /attendance/submit` working until Plan 2 ships, then delete it in Plan 2.

- [ ] **Step 1: Failing table test** `TestDecideCheckIn` (rules = `DefaultRules()`, base input class day, IP allowed):

| now | extra | want |
|---|---|---|
| 8:00 | — | `ErrNotOpenYet` |
| 9:10 | — | morning, present |
| 9:40 | — | morning, late |
| 10:31 | — | `ErrCheckInClosed` |
| 13:05 | — | afternoon, present |
| 14:00 | — | afternoon, late |
| 9:10 | `IsClassDay=false` | `ErrNoClassToday` |
| 9:10 | `IPAllowed=false` | `ErrNetworkNotAllowed` |
| 9:10 | `AlreadyCheckedIn=true` | `ErrAlreadySubmitted` |
| 9:10 | `Leave=absent_excused` | `ErrCoveredByLeave` |
| 9:10 | `Leave=late_excused` | morning, **present** |
| 9:40 | `Leave=late_excused` | morning, **late_excused** |

- [ ] **Step 2: Run** `go test ./internal/service/attendance/ -run DecideCheckIn -v` → FAIL.
- [ ] **Step 3: Implement** `DecideCheckIn` and `CheckinService` (new struct in `checkin.go`; holiday check = today within any holiday `StartDate..EndDate`; "class day" = weekday Mon–Fri and not holiday). Handler maps errors to the statuses above.
- [ ] **Step 4: Run** tests → PASS; `go build ./...`; manual: run server, `curl -X POST localhost:$PORT/attendance/check-in -H "Authorization: Bearer $LEARNER" -d '{"style":"slide"}'` → 201 during a window / 422 outside.
- [ ] **Step 5: Commit** `feat(attendance): code-free self check-in`.

---

### Task 4: Session closing job (auto-absent + leave fill)

**Files:**
- Create: `internal/service/attendance/closer.go`
- Test: `internal/service/attendance/closer_test.go`
- Modify: `cmd/main.go` (start ticker goroutine), `internal/repository/attendance_repo.go` (add `UpsertRecord(ctx, *domain.AttendanceRecord) error` keyed by `user_id+date+session+deleted:false`), `internal/domain/attendance.go` (interface method)

**Interfaces:**
- Consumes: Task 1 (`PhaseAt`, `SessionAt`), Task 2 settings, learner list via `UserServiceInterface.GetAllUsers(cohort, "learner", …, "dropout,dismissed")`.
- Produces:
  - `func PlanSessionClose(learners []primitive.ObjectID, checkedIn map[primitive.ObjectID]bool, leave map[primitive.ObjectID]domain.AttendanceStatus) map[primitive.ObjectID]domain.AttendanceStatus` — for each learner not checked in: leave status if present in `leave`, else `absent`. Checked-in learners are omitted.
  - `(*SessionCloser) CloseIfDue(now time.Time) error` — for every cohort with learners: if the session's `PhaseAt(now) == PhaseClosed`, it is a class day, and not yet closed today (in-memory `map[cohort+date+session]bool`), upsert planned records with `MarkedBy: admin`, `MarkedByUser: "system"`.
  - `main.go`: `go func(){ for range time.Tick(time.Minute) { closer.CloseIfDue(utils.GetThailandTime()) } }()`

- [ ] **Step 1: Failing tests.** `TestPlanSessionClose`: learners {a,b,c,d}; checkedIn {a}; leave {c: absent_excused, d: late_excused} → `{b: absent, c: absent_excused, d: late_excused}`. `TestPlanSessionClose_AllIn` → empty map. `TestCloseIfDue_Idempotent` using an in-test fake repo counting upserts: calling `CloseIfDue(10:31)` twice upserts once per learner; `CloseIfDue(10:29)` upserts nothing; on a holiday upserts nothing.
- [ ] **Step 2: Run** `go test ./internal/service/attendance/ -run 'PlanSessionClose|CloseIfDue' -v` → FAIL.
- [ ] **Step 3: Implement** (fakes live in `closer_test.go`; give `SessionCloser` small interfaces for the repos it uses so the fake stays tiny).
- [ ] **Step 4: Run** → PASS; `go build ./...`.
- [ ] **Step 5: Commit** `feat(attendance): close sessions and auto-mark absences`.

---

### Task 5: Learner progress (streak, leave used, warnings, rate) + summary endpoint

**Files:**
- Create: `internal/service/attendance/progress.go`
- Test: `internal/service/attendance/progress_test.go`
- Modify: `internal/handler/attendance_handler.go` (`GetMySummary`), `cmd/routes.go`

**Interfaces:**
- Consumes: `CombineDayStatus` (Task 1), existing `GetMyAttendanceHistory` data source.
- Produces:
  - `type DayResult struct{ Date string; Status domain.AttendanceStatus }` (oldest → newest).
  - `func OnTimeStreak(days []DayResult) int` — walk from newest: `present` +1; `holiday`, `no_class`, `absent_excused`, `late_excused` skip; `late`, `absent` stop.
  - `func LeaveDaysUsed(days []DayResult) int` — count of `absent`, `absent_excused`, `late`, `late_excused`.
  - `func WarningLevel(used int) string` — `"none" | "first" | "second" | "third"` per Global Constraints.
  - `func AttendanceRate(days []DayResult) float64` — `(present+late+late_excused) / (days excluding holiday, no_class, dropout, dismissed)`; 0 when denominator 0.
  - `func HasPerfectWeek(days []DayResult) bool` — true if some Mon–Fri week has a record for all 5 weekdays, at least one of them is a class day, and every class day (not `holiday`/`no_class`) is `present`.
  - `GET /attendance/me/summary` → `{ "today": {"morning": {"status","checked_in_at","style"} | null, "afternoon": …}, "streak": int, "leave_used": int, "leave_allowance": 8, "warning_level": string, "attendance_rate": float, "perfect_week": bool, "rules": CheckinRules }` for the caller only.

- [ ] **Step 1: Failing tests** (`d(date, status)` helper):
  - `OnTimeStreak` (oldest→newest input): `[P,P,L,P,AE,H,P]` → **2** (newest P, skip H, skip AE, P, then L stops); `[]` → 0; `[P,P,P]` → 3; `[P,A]` → 0.
  - `LeaveDaysUsed([P,A,AE,L,LE,H,N])` → 4.
  - `WarningLevel`: 3→none, 4→first, 5→first, 6→second, 7→third, 8→third.
  - `AttendanceRate([P,P,L,A,H,N])` → 0.75; `[]` → 0.
  - `HasPerfectWeek`: Mon 2026-09-07…Fri 09-11 all P → true; same week with Wed L → false; same week with Fri holiday and Mon–Thu P → true; only Mon–Wed records → false.
- [ ] **Step 2: Run** `go test ./internal/service/attendance/ -run 'Streak|LeaveDays|Warning|Rate|PerfectWeek' -v` → FAIL.
- [ ] **Step 3: Implement** `progress.go` and the handler (build `[]DayResult` from the caller's records grouped by date with `CombineDayStatus`; today included only if its session(s) so far are final).
- [ ] **Step 4: Run** → PASS; `go build ./...`.
- [ ] **Step 5: Commit** `feat(attendance): learner summary with streak, leave and warnings`.

---

### Task 6: Preferences, unlocks and privacy

**Files:**
- Create: `internal/service/user/unlocks.go`
- Test: `internal/service/user/unlocks_test.go`, `internal/domain/user_safe_test.go`
- Modify: `internal/domain/user.go` (fields + `ToSafe`), `internal/handler/user_handler.go` (`UpdateMyPreferences`), `cmd/routes.go`

**Interfaces:**
- Consumes: `OnTimeStreak`, `HasPerfectWeek` (Task 5) via the summary data.
- Produces:
  - `domain.Avatar{ Style string \`json:"style"\`; Seed string \`json:"seed"\`; Bg string \`json:"bg"\`; Options map[string]map[string]*string \`json:"options"\` }` — `Options[style][part] = value | nil (None)`.
  - User fields (bson/json snake_case): `CheckinStyle string`, `CheckinRandom bool`, `Avatar *Avatar`, `Skills []string`.
  - `UserSafe` gains **only** `Avatar *Avatar \`json:"avatar,omitempty"\`` and `Skills []string \`json:"skills,omitempty"\``.
  - `func StyleUnlocked(style string, streak int, perfectWeek bool) (known, unlocked bool)`
  - `func AvatarAllowed(a domain.Avatar, streak int) error` — style in the 6 ids; `Bg` matches `^[0-9a-fA-F]{6}$`; notionists `bodyIcon` `saturn` needs ≥15, `galaxy` ≥30.
  - `PUT /users/me/preferences` body `{ "checkin_style"?, "checkin_random"?, "avatar"?, "skills"? }` → 200 updated prefs; 403 `"Locked: keep your streak going"` when locked; 400 unknown style / bad avatar; `skills` max 5 items, each ≤ 24 chars.

- [ ] **Step 1: Failing tests.**
  - `TestStyleUnlocked`: `("slide",0,false)→(true,true)`; `("rocket",14,false)→(true,false)`; `("rocket",15,false)→(true,true)`; `("arcade",19,…)→locked`; `("dj",30,…)→unlocked`; `("gold",99,false)→locked`; `("gold",0,true)→unlocked`; `("victory-lap",…)→(false,false)` (comes in Plan 5).
  - `TestAvatarAllowed`: valid notionists → nil; style `"bitmoji"` → error; bg `"#fff"` → error; saturn at 14 → error, at 15 → nil; galaxy at 29 → error.
  - `TestToSafeHidesPrivateData`: build a `User` with `AttendanceStatus:"dropout"`, reflections, `Avatar`, `Skills`; `json.Marshal(u.ToSafe())` contains `"avatar"` and `"skills"` and does **not** contain `attendance_status`, `reflections`, `checkin_style`, `checkin_random`, `email`, `password`.
- [ ] **Step 2: Run** `go test ./internal/... -run 'StyleUnlocked|AvatarAllowed|ToSafe' -v` → FAIL.
- [ ] **Step 3: Implement**; handler loads the caller's summary (Task 5) to get streak / perfect week before validating.
- [ ] **Step 4: Run** → PASS; `go build ./... && go test ./...`.
- [ ] **Step 5: Commit** `feat(user): check-in style and avatar preferences with unlocks`.

---

### Task 7: Swagger + branch wrap-up

**Files:** Modify `docs/swagger.yaml` (and regenerate `docs.go`/`swagger.json` with the repo's existing swag command if present; otherwise edit yaml only).

- [ ] **Step 1:** Document `POST /attendance/check-in`, `GET /attendance/me/summary`, `PUT /users/me/preferences`, `GET|PUT /admin/cohorts/:cohort/settings`, and mark `GET /attendance/code` removed.
- [ ] **Step 2: Run** `go build ./... && go test ./... && go vet ./...` → all pass.
- [ ] **Step 3: Commit** `docs(api): self check-in endpoints`.
