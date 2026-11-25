# Precomputed learner stats and slim reads for admin views

Date: 2026-11-26
Repos: `baro-gofiber` (most of the work), `react-genbaro` (one component)

## Problem

Every query between the backend (Dokploy) and MongoDB Atlas moves at about 97 KB/s. Admin pages pull hundreds of KB to MB per load, so each takes 3–13 s:

| Page | What it pulls today | Size, time |
|---|---|---|
| Today | learners with every reflection's mood fields + slim attendance history of the whole cohort | 247 + 512 KB, 5.5 s |
| Register | slim attendance history of the whole cohort | 512 KB, 5.5 s |
| Support / Learners | learners with the last 30 reflections, full text | 815 KB, 8.5 s |
| Reflections board | learners with the last 45 reflections, full text | 1.2 MB, 12.8 s |
| `GET /users` | badges, profile comments, reactions, care log for every classmate | 337 KB, 3.5 s |

The numbers each page shows (attendance rate, leave used, on-time streak, late run, last 10 days) are recomputed from a learner's whole attendance history on every load, and mood strips are built from reflection documents that carry full text.

## Goal

Each page loads only what it shows. Totals are read precomputed; day-level detail is fetched only for the dates on screen; a single learner's full data is fetched when that learner is opened. Pages must show exactly the numbers they show today.

Success: after rollout, Today, Register, Learners and Support produce no `slow request` log line (each under 0.5 s even at ~97 KB/s); the board stays around 1 s or less.

## Facts this design relies on

- Every stat is a pure function of one learner's live attendance records. `BuildDays`, `OnTimeStreak`, `LeaveDaysUsed`, `AttendanceRate` and the late-run loop in `wellbeing.SupportReasons` do not depend on the current time.
- Every attendance write goes through six `domain.AttendanceRepository` methods: `InsertRecord`, `InsertIfAbsent`, `UpsertRecord`, `UpdateRecord`, `UpdateRecords`, `DeleteRecord`. The only other writer is the duplicate cleanup in `config.go`, which runs once at startup.
- `UpdateRecords` has one caller, `SubmissionService.LockSession`, and it only sets `locked`.
- Reflection text is used only for the latest class night: board entries, the "worth a look" quote, and `WorthALook`'s watch-word check on the newest night. Every other night needs only its zone.
- Reflections are appended with `$push`, so the tail of the array is the newest.

## 1. `learner_stats`

### Data

New collection `learner_stats`, one document per learner, `_id` = user id, index on `cohort_number`:

```
{ _id, cohort_number,
  attendance_rate, leave_used, streak, late_run,
  last10: [{ date, status }], last_seen, updated_at }
```

- `last_seen` is the newest `submitted_at` of a self check-in (what the Learners table shows today).
- A learner with no document reads as all zeros, which matches what the current code computes from an empty history.

### Computation

`attendance.ComputeStats(records []domain.AttendanceRecord) LearnerStats` calls the existing functions (`BuildDays`, `OnTimeStreak`, `LeaveDaysUsed`, `AttendanceRate`). The late-run loop moves out of `wellbeing.SupportReasons` into a shared `attendance.LateRun(days)` that `SupportReasons` also calls, so the logic stays in one place.

### Keeping it correct

- A decorator wraps the attendance repository in `container.go`. After a successful `InsertRecord`, `InsertIfAbsent` (only when it inserted), `UpsertRecord`, `UpdateRecord` or `DeleteRecord`, it recomputes that learner's stats. `UpdateRecord` and `DeleteRecord` take a record id, so the decorator reads the record first to learn the user id.
- `UpdateRecords` recomputes the affected learners only when the update sets `status` or `deleted`. No current caller does.
- Recompute is synchronous: it reads that one learner's records (~10 KB), so the next read sees the new numbers (an admin marks a learner, Today refetches, the streak is already right). If recompute fails, the write still succeeds and the error is logged; the next rebuild fixes it.
- A rebuild recomputes every learner of every active cohort (same definition the session closer uses: a live record in the last 7 days). It runs in the background at startup, which also covers the startup duplicate cleanup, and nightly at 02:00 Asia/Bangkok.

## 2. What each page reads

Rule: totals from `learner_stats`, day-level records only for the dates on screen. JSON response shapes do not change, so the frontend needs no changes for these pages.

| Page | Reads |
|---|---|
| Today | stats for the cohort + full records for today only (check-in time, IP, marked-by, record id) |
| Register | stats (`leave_used` for the 6+/7 KPIs) + full records for the selected week only |
| Learners | stats (`attendance_rate`, `leave_used`, `last_seen`) + moods (section 3) |
| Support | stats (`leave_used`, `late_run`) + moods and last-night text (section 3) |
| Learner profile | unchanged: that learner's own records, with `ComputeStats` called there |

Code changes:

- `BuildTodayRoster` takes a stats map instead of the whole history; streak, warning level, rate and last10 come from stats, the morning/afternoon cells from today's records.
- `BuildRegister` takes a `leave_used` map instead of deriving it from history.
- `ReasonsFor` and `BuildSupportList` take stats instead of records.
- The weekly report job keeps reading full history (it needs eight weeks of trends and runs once a week). It builds its `leave_used` map with `ComputeStats` from the history it already loads.
- `loadCohortRecords` (slim whole-cohort history) remains only for the weekly report.
- The learner-side Today (`MySummary`) is unchanged; it already reads one learner's records.

## 3. Reflections through an aggregation pipeline

A user-repository method `FindCohortLearnersWithReflections(cohort, baseFields, n, textDay)` runs:

```
$match   { cohort_number, role: "learner" }
$sort    { jsd_number: 1 }
$project { <base fields>,
  reflections: $map over $slice(reflections, -n):
    day of r == textDay ? r (whole)
                        : { _id, day, date, reflection: { barometer } } }
```

- `textDay` is the latest class night. An empty `textDay` keeps no text.
- A reflection without `day` is matched on `$dateToString` of `date` in UTC, the same as `dayOf` in `wellbeing/signals.go`.
- The result is still `[]domain.User`, so `Nights`, `WorthALook` and `BuildBoard` are unchanged.

| Page | n | Text kept for | Size before → after (approx.) |
|---|---|---|---|
| Today (yesterday's mood) | 5 | none | 247 KB → 15 KB |
| Learners, Support | 20 | latest class night | 815 KB → 60 KB |
| Reflections board | 30 | the board's night | 1.2 MB → 110 KB |
| Weekly summary, weekly report | all | unchanged | unchanged |

Known limit: a panic run is counted back at most `n` nights (20 for Learners/Support). Today's code already caps it at 30.

## 4. `GET /users` and fetching a learner on open

- `GET /users` stops loading `profile_comments`, `profile_reactions`, `plant_reactions`, `care_energy_log`, `gift_boxes`, `owned_cosmetic_ids` and `new_cosmetic_ids` (`reflections` and `password` were dropped in #98). It keeps `badges` and `social_links`: they are small, and the list shows badge counts.
- `ClassmatePanel` fetches `GET /users/:id` when opened (React Query key `["person", id]`). Name, avatar, bio and badges render from the list right away; the Recommendations section shows a skeleton until the fetch completes.
- `GET /users/:id` skips loading `reflections` when the viewer is neither the owner nor an admin; `ToSafe()` already drops them before responding, so this only saves the transfer.
- Unchanged: the full profile page (`UserProfilePage`) already fetches one user; the genmate garden has its own endpoint.

Expected: the list drops from 337 KB to about 40–60 KB; opening a classmate costs about 5–10 KB.

## 5. Testing and rollout

### Tests

- `ComputeStats` against record fixtures (late, excused, holiday, absent, half-day records): rate, leave, streak, late run and last10 must equal the current functions' results.
- Repository decorator: each write method recomputes the right learner; `InsertIfAbsent` that did not insert and `UpdateRecords` setting only `locked` do not; a failing recompute does not fail the write.
- Rebuild: every learner of every active cohort gets a document.
- Handlers: Today, Register, Learners and Support JSON unchanged (existing fakes).
- Frontend: `ClassmatePanel` shows the skeleton, then the fetched recommendations.
- Every PR passes `go test ./internal/...`, typecheck, vitest and `npm run build`.

### Rollout: three PRs, in this order

Each can ship on its own without breaking anything.

1. **Backend, write only.** `learner_stats`, decorator, startup and nightly rebuild. Reads are unchanged. While this runs, the Today handler compares its computed stats with `learner_stats` and logs `stats drift: cohort N, M learners differ` on any mismatch. Leave it a day to prove parity on real data.
2. **Frontend.** `ClassmatePanel` fetches the learner on open. This ships before step 3 drops `profile_comments` from the list, so recommendations never disappear.
3. **Backend, switch reads.** Pages read stats, reflections go through the pipeline, `GET /users` and `GET /users/:id` load fewer fields, and the drift check is removed.

Rollback: reverting PR 3 returns every page to compute-on-read; a leftover `learner_stats` collection is harmless.

## Out of scope

- Moving MongoDB off Atlas or changing the Atlas tier (separate decision; the 97 KB/s cap may lift once the 7-day transfer window resets after #98).
- Server-side response caching.
- Changing the weekly report or weekly summary queries.
