# Baro Backend

Go Fiber REST API for the Baro learner reflection system.

## Prerequisites

- Go 1.24+
- MongoDB (local or Atlas)
- Running MongoDB instance

## Installation

```bash
cd baro-gofiber
go mod download
```

## Environment Variables

Create a `.env` file (copy from `.env.example`):

```bash
cp .env.example .env
```

Edit `.env`:

| Variable | Description | Required |
|----------|-------------|----------|
| `PORT` | Server port | Yes (default: 3000) |
| `MONGO_URI` | MongoDB connection string (`MONGODB_URI` is ignored) | Yes |
| `DATABASE_NAME` | MongoDB database name | Yes |
| `JWT_SECRET_KEY` | Secret for JWT signing | Yes |
| `CORS_ALLOWED_ORIGINS` | Allowed CORS origins (comma-separated) | Yes |
| `ENVIRONMENT` | Environment (development/production) | No |

Example `.env`:
```env
PORT=3000
MONGO_URI=mongodb://localhost:27017/baro
DATABASE_NAME=baro
JWT_SECRET_KEY=your-secret-key
CORS_ALLOWED_ORIGINS=http://localhost:5173
ENVIRONMENT=development
```

## Running the Server

```bash
# Development (the package spans several files in cmd/, so run the directory)
go run ./cmd

# Production
go build -o baro-backend ./cmd
./baro-backend
```

The API runs on `http://localhost:3000` (or PORT env value).

> On every boot the server cleans duplicate attendance rows, syncs indexes and rebuilds `learner_stats` against whatever `MONGO_URI` points at. Don't start it against production data casually.

## Available Scripts

| Command | Description |
|---------|-------------|
| `go run ./cmd` | Start development server |
| `go test ./internal/...` | Run the tests |
| `go vet ./internal/... ./cmd/...` | Vet |
| `go build` | Build binary |
| `go mod tidy` | Clean up dependencies |
| `go mod download` | Download dependencies |

## Tech Stack

- **Framework**: Go Fiber v2
- **Database**: MongoDB (go.mongodb.org/mongo-driver)
- **Auth**: JWT (golang-jwt/jwt/v4)
- **Docs**: Swagger (swaggo)
- **Validation**: go-playground/validator

## Project Structure

```
baro-gofiber/
├── cmd/
│   ├── main.go           # Entry point, app setup
│   ├── routes.go        # Route definitions
│   └── container.go     # Dependency injection container
├── internal/
│   ├── handler/         # HTTP request handlers
│   │   ├── user.go
│   │   ├── admin.go
│   │   ├── attendance.go
│   │   ├── leave.go
│   │   ├── holiday.go
│   │   ├── talk_board.go
│   │   └── notification.go
│   └── service/         # Business logic
│       ├── user/
│       │   ├── user_service.go
│       │   └── badge_service.go
│       └── reflection/
│           └── reflection_service.go
├── pkg/
│   ├── middleware/      # Custom middleware
│   │   └── auth.go      # JWT auth & admin role check
│   └── utils/           # Utilities
│       ├── response.go  # JSON response helpers
│       ├── jwt.go       # JWT helpers
│       ├── bcrypt.go    # Password hashing
│       └── time.go      # Time utilities
├── config/
│   └── config.go        # Configuration loading
├── docs/                # Generated Swagger docs
├── go.mod
├── go.sum
└── .env.example
```

## API Endpoints

### Authentication
| Method | Endpoint | Description | Auth |
|--------|----------|-------------|------|
| POST | `/login` | User login | No |
| GET | `/api/verify-token` | Verify JWT token | Yes |

### Users (Protected)
| Method | Endpoint | Description | Auth |
|--------|----------|-------------|------|
| GET | `/users/:id` | Get user profile | Yes |
| POST | `/users/:id/reflections` | Create reflection | Yes |
| GET | `/users/:id/reflections` | Get user reflections | Yes |

### Admin views
| Method | Endpoint | Description | Auth |
|--------|----------|-------------|------|
| GET | `/admin/today?cohort=` | Live check-in board (stats + today's records) | Admin |
| GET | `/admin/attendance/register?cohort=&week=` | Week register (selected + previous week for KPI deltas) | Admin |
| GET | `/admin/learners?cohort=` | All learners table | Admin |
| GET | `/admin/support?cohort=` | Needs support list | Admin |
| GET | `/admin/reflections/board?cohort=&range=` | Reflections board (`night`, `week`, `4w`) | Admin |
| GET | `/admin/learners/:id/overview` | One learner's full profile | Admin |
| GET | `/admin/metrics/latency` | p50/p95/p99 per route (see Observability) | Owner |

### Admin
| Method | Endpoint | Description | Auth |
|--------|----------|-------------|------|
| GET | `/admin/users` | Get all users | Admin |
| GET | `/admin/userreflections/:id` | Get user with reflections | Admin |
| POST | `/admin/users/:id/badges` | Award badge to user | Admin |
| PUT | `/admin/users/:userId/reflections/:reflectionId/feedback` | Give feedback | Admin |
| GET | `/admin/barometer` | Get barometer data | Admin |
| GET | `/admin/reflections` | Get all reflections | Admin |
| GET | `/admin/reflections/chartday` | Daily barometer chart data | Admin |
| GET | `/admin/reflections/weekly` | Weekly summary | Admin |
| GET | `/admin/emoji-zone-table` | Emoji zone table | Admin |

### Attendance (Admin)
| Method | Endpoint | Description | Auth |
|--------|----------|-------------|------|
| POST | `/admin/attendance/generate-code` | Generate attendance code | Admin |
| GET | `/admin/attendance/active-code` | Get active code | Admin |
| GET | `/admin/attendance/today` | Today's overview | Admin |
| POST | `/admin/attendance/manual` | Manual mark attendance | Admin |
| GET | `/admin/attendance/logs` | Get attendance logs | Admin |
| GET | `/admin/attendance/stats` | Attendance statistics | Admin |
| GET | `/admin/attendance/stats-by-days` | Stats by days | Admin |
| GET | `/admin/attendance/daily-stats` | Daily stats | Admin |
| GET | `/admin/attendance/student/:id` | Student attendance history | Admin |
| POST | `/admin/attendance/lock` | Lock attendance session | Admin |
| POST | `/admin/attendance/bulk` | Bulk mark attendance | Admin |
| DELETE | `/admin/attendance/:id` | Delete attendance record | Admin |

### Attendance (Student)
| Method | Endpoint | Description | Auth |
|--------|----------|-------------|------|
| POST | `/attendance/submit` | Submit attendance | Yes |
| GET | `/attendance/my-status` | My today's status | Yes |
| GET | `/attendance/my-history` | My attendance history | Yes |
| GET | `/attendance/my-daily-stats` | My daily stats | Yes |
| GET | `/attendance/code` | Get active code | Yes |

### Holidays (Admin)
| Method | Endpoint | Description | Auth |
|--------|----------|-------------|------|
| POST | `/admin/holidays` | Create holiday | Admin |
| GET | `/admin/holidays` | Get all holidays | Admin |
| DELETE | `/admin/holidays/:id` | Delete holiday | Admin |

### Leave Requests
| Method | Endpoint | Description | Auth |
|--------|----------|-------------|------|
| POST | `/leave-requests` | Create leave request | Yes |
| GET | `/leave-requests/my` | My leave requests | Yes |

### Leave Requests (Admin)
| Method | Endpoint | Description | Auth |
|--------|----------|-------------|------|
| POST | `/admin/leave-requests` | Create for user | Admin |
| GET | `/admin/leave-requests` | Get all requests | Admin |
| PATCH | `/admin/leave-requests/:id` | Update status | Admin |

### Notifications
| Method | Endpoint | Description | Auth |
|--------|----------|-------------|------|
| GET | `/api/notifications` | Get active notifications | Yes |
| POST | `/api/notifications/:id/read` | Mark as read | Yes |

### Notifications (Admin)
| Method | Endpoint | Description | Auth |
|--------|----------|-------------|------|
| POST | `/admin/notifications` | Create notification | Admin |
| GET | `/admin/notifications` | Get all notifications | Admin |
| PUT | `/admin/notifications/:id` | Update notification | Admin |
| DELETE | `/admin/notifications/:id` | Delete notification | Admin |

### Talk Board
| Method | Endpoint | Description | Auth |
|--------|----------|-------------|------|
| GET | `/board/posts` | Get all posts | Yes |
| GET | `/board/posts/:postId` | Get single post | Yes |
| POST | `/board/posts` | Create post | Yes |
| POST | `/board/posts/:postId/comments` | Add comment | Yes |
| POST | `/board/posts/:postId/reactions` | Add reaction | Yes |
| DELETE | `/board/posts/:postId/reactions` | Remove reaction | Yes |
| POST | `/board/posts/:postId/comments/:commentId/reactions` | React to comment | Yes |

## Authentication

All protected routes require a JWT token in the header:
```
Authorization: Bearer <token>
```

Tokens are generated on login and contain user ID and role.

## Admin Role

Admin routes check for `role: "admin"` in the JWT payload. The middleware `CheckAdminRole` enforces this.

## Swagger Documentation

Swagger docs are generated at `/swagger/index.html` when running in development.

Access: `http://localhost:3000/swagger/index.html`

## Rate Limiting

Admin routes are rate-limited to 300 requests/minute using Fiber's limiter middleware.

## Precomputed Learner Stats

The admin views don't recompute a cohort's whole attendance history on every page load. Each learner's totals live in the `learner_stats` collection (one document per learner):

| Field | Meaning |
|-------|---------|
| `attendance_rate` | Attended sessions ÷ class sessions |
| `leave_used` | Days whose combined morning+afternoon status is absent, excused, late or late-excused (one day counts once; a holiday on either half makes the day a holiday) |
| `streak` | Consecutive on-time days from the newest (holidays and excused days skipped) |
| `late_run` | Consecutive late days from the newest |
| `last10` | The last 10 combined days (only the Today board reads it) |
| `last_seen` | Newest self check-in |

How it stays correct:
- **On every write.** `attendance.TrackStats` wraps the attendance repository. After any insert, upsert, update or delete of an attendance record, it recomputes that one learner's stats from their records. A failed recompute is logged and never fails the write.
- **Rebuilds.** At startup every cohort with records is rebuilt. Nightly at 02:00 Asia/Bangkok, cohorts active in the last 7 days are rebuilt. A rebuild skips learners recomputed after it started.
- **One source of truth.** `attendance.ComputeStats` calls the same functions (`BuildDays`, `OnTimeStreak`, `LeaveDaysUsed`, `AttendanceRate`) that the learner's own summary and the weekly report use.

Each view also loads only what it shows:
- Today loads today's records; Register loads the selected week (plus the previous week, slim, for KPI deltas).
- Learners, Support and the board load each learner's last 20 or 30 reflections through an aggregation that keeps text only for the latest class night.
- `GET /users` drops comments, reactions and logs; a classmate's `GET /users/:id` skips reflections.

## Observability

- `slow request: METHOD path status duration` is logged for requests over 500 ms.
- `slow mongo: <command> <collection> took X, reply N KB` is logged for Mongo commands over 300 ms, and failures are logged too.
- `MongoDB round trip: X` is logged at startup.
- `GET /admin/metrics/latency` lists p50/p95/p99 (ms) per route over each route's last 1000 requests, slowest p95 first. It is kept in memory and resets on deploy.

## Database Collections

| Collection | Description |
|------------|-------------|
| `users` | User accounts |
| `reflections` | Daily reflections |
| `holidays` | Admin-set holidays |
| `leave_requests` | Leave requests |
| `posts` | Talk board posts |
| `comments` | Post comments |
| `reactions` | Post/comment reactions |
| `notifications` | System notifications |
| `badges` | Available badges |
| `user_badges` | User-earned badges |
| `attendance_records` | Check-in records (morning/afternoon per learner per day) |
| `learner_stats` | Precomputed attendance totals per learner |

## Middleware

| Middleware | Purpose |
|------------|---------|
| `AuthMiddleware` | Verify JWT token |
| `CheckAdminRole` | Ensure user is admin |
| `limiter` | Rate limiting (admin routes) |

## License

ISC
