package handler

import (
	"math"
	"slices"
	"sort"
	"sync"
	"time"

	"gofiber-baro/pkg/utils"

	"github.com/gofiber/fiber/v2"
)

// latencyWindow is how many recent requests per route the percentiles cover.
const latencyWindow = 1000

// Latency keeps each route's most recent response times in memory (reset on restart).
type Latency struct {
	mu      sync.Mutex
	samples map[string][]time.Duration
	next    map[string]int
}

func NewLatency() *Latency {
	return &Latency{samples: map[string][]time.Duration{}, next: map[string]int{}}
}

// Observe records one response time for a route such as "GET /admin/today".
func (l *Latency) Observe(route string, d time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	s := l.samples[route]
	if len(s) < latencyWindow {
		l.samples[route] = append(s, d)
		return
	}
	s[l.next[route]] = d
	l.next[route] = (l.next[route] + 1) % latencyWindow
}

// LatencyRow is one route's percentiles in milliseconds.
type LatencyRow struct {
	Route string  `json:"route"`
	Count int     `json:"count"`
	P50   float64 `json:"p50_ms"`
	P95   float64 `json:"p95_ms"`
	P99   float64 `json:"p99_ms"`
}

func percentile(sorted []time.Duration, p float64) time.Duration {
	i := int(math.Ceil(p/100*float64(len(sorted)))) - 1
	return sorted[max(0, min(i, len(sorted)-1))]
}

func ms(d time.Duration) float64 { return math.Round(float64(d)/float64(time.Millisecond)*10) / 10 }

func (l *Latency) snapshot() []LatencyRow {
	l.mu.Lock()
	rows := make([]LatencyRow, 0, len(l.samples))
	for route, s := range l.samples {
		sorted := slices.Clone(s)
		slices.Sort(sorted)
		rows = append(rows, LatencyRow{Route: route, Count: len(sorted), P50: ms(percentile(sorted, 50)), P95: ms(percentile(sorted, 95)), P99: ms(percentile(sorted, 99))})
	}
	l.mu.Unlock()
	sort.Slice(rows, func(i, j int) bool { return rows[i].P95 > rows[j].P95 })
	return rows
}

// Report lists every route's p50/p95/p99 over its last requests, slowest p95 first.
// @Summary  Response-time percentiles  @Tags admin  @Security BearerAuth  @Router /admin/metrics/latency [get]
func (l *Latency) Report(c *fiber.Ctx) error {
	return utils.SendResponse(c, fiber.StatusOK, "Latency", l.snapshot())
}
