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
- `PUT /users/me/preferences` — any of `checkin_style`, `checkin_random`, `avatar` `{style, seed, bg, options}`, `skills` (≤5, ≤24 chars). 403 when a style/avatar item is still locked (rocket 15 · arcade 20 · dj 30 · gold = perfect week · victory = this week's Cup champions · saturn 15 · galaxy 30). Summary also has `champion` (bool).
- Still available (from main): `GET /attendance/code` and `POST /attendance/submit` for the code-based flow. Self check-in works alongside it.

## Admin

- `GET /admin/cohorts/:cohort/settings` → rules + `allowed_networks` (defaults if never saved).
- `PUT /admin/cohorts/:cohort/settings` — `{rules, allowed_networks: ["10.0.0.0/8","203.0.113.7"], group_chat_urls, start_date: "2026-06-29", course_days: 75}`. Empty `allowed_networks` = any network.

## GenMate Cup

- `GET /cup/me` → `week` (Monday), `week_no`, `ends_at` (Fri 16:45), `standings[]` (`group`, `total`, `by_category` {coding, soft, showing_up, helping}, `by_day` [Mon..Fri]; total desc, ties → coding), `my_group`, `my_awards` (last 6 for my group), `champion_group` (last week's winner), `my_medals[]` (`week`, `week_no`), `wall_of_fame[]`.
- `POST /admin/cup/awards` — `{cohort, group, category: coding|soft|helping, points: 1-100, reason ≤140}` → 201. `showing_up` is server-only.
- `DELETE /admin/cup/awards/:id` — undo a TA award.
- `GET /admin/cup/standings?cohort=13&week=2026-09-07` → `{week, standings, awards}`.

## Background job

Every minute the server closes any session whose window has passed on a class day (Mon–Fri, not a holiday): learners without a record get `absent`, or their approved-leave status (`absent_excused` / `late_excused`). Existing records are never overwritten.

Right after that, the Cup runner (1) adds **+15 Showing up** to every GenMate group whose active learners were all present or excused in both sessions (after the afternoon closes), and (2) after Friday 16:45 closes the week: the winning group gets a "Rare" character gift box (`GenMate Cup · Week N champions`) and a medal, and is champion the following week (gold frame, Victory Lap). A missed Friday closes on the next tick; all-zero weeks have no winner. Every write is keyed, so restarts never double-award.

## Deploy note

Set `PROXY_HEADER=X-Forwarded-For` when running behind a load balancer so the network limit sees learners' real IPs.
