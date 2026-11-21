# Self check-in API (Plan 1)

All times are Asia/Bangkok. Default rules (per cohort, editable by admins):

| Session | Opens | Present until | Late until | Then |
|---|---|---|---|---|
| Morning | 08:30 | 09:15:00 | 10:30:00 | Absent (auto) |
| Afternoon | 12:30 | 13:15:00 | 14:30:00 | Absent (auto) |

## Learner

- `POST /attendance/check-in` — body `{"style":"slide"}` (optional). No code.
  - 201 → the new record (`status` present | late | late_excused).
  - 403 network not allowed · 409 already checked in / approved leave covers it · 422 not open yet / closed / no class.
- `GET /attendance/me/summary` → `today.morning|afternoon` (`status`, `checked_in_at`, `style`) or null, `streak`, `leave_used`, `leave_allowance` (8), `warning_level` (none | first ≥4 | second ≥6 | third ≥7), `attendance_rate` (0–1), `perfect_week`, `class_today` (false on weekends/holidays), `rules`, `preferences`, `course` (`start_date`, `course_days`, `day_number`, `week_number`, `total_weeks`; zeros when no start date).
- `PUT /users/me/preferences` — any of `checkin_style`, `checkin_random`, `avatar` `{style, seed, bg, options}`, `skills` (≤5, ≤24 chars). 403 when a style/avatar item is still locked (rocket 15 · arcade 20 · dj 30 · gold = perfect week · saturn 15 · galaxy 30).
- Removed for learners: `GET /attendance/code`. `POST /attendance/submit` was removed in Plan 2.

## Admin

- `GET /admin/cohorts/:cohort/settings` → rules + `allowed_networks` (defaults if never saved).
- `PUT /admin/cohorts/:cohort/settings` — `{rules, allowed_networks: ["10.0.0.0/8","203.0.113.7"], group_chat_urls, start_date: "2026-06-29", course_days: 75}`. Empty `allowed_networks` = any network.

## Background job

Every minute the server closes any session whose window has passed on a class day (Mon–Fri, not a holiday): learners without a record get `absent`, or their approved-leave status (`absent_excused` / `late_excused`). Existing records are never overwritten.

## Deploy note

Set `PROXY_HEADER=X-Forwarded-For` when running behind a load balancer so the network limit sees learners' real IPs.
