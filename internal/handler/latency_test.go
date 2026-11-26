package handler

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

func TestPercentileNearestRank(t *testing.T) {
	var d []time.Duration
	for i := 1; i <= 100; i++ {
		d = append(d, time.Duration(i)*time.Millisecond)
	}
	for p, want := range map[float64]time.Duration{50: 50 * time.Millisecond, 95: 95 * time.Millisecond, 99: 99 * time.Millisecond, 100: 100 * time.Millisecond} {
		if got := percentile(d, p); got != want {
			t.Errorf("p%v = %v, want %v", p, got, want)
		}
	}
	if got := percentile([]time.Duration{7 * time.Millisecond}, 99); got != 7*time.Millisecond {
		t.Errorf("single sample p99 = %v", got)
	}
}

func TestLatencyKeepsOnlyTheRecentWindow(t *testing.T) {
	l := NewLatency()
	for i := 0; i < latencyWindow; i++ {
		l.Observe("GET /slow", 5*time.Second)
	}
	for i := 0; i < latencyWindow; i++ {
		l.Observe("GET /slow", 100*time.Millisecond)
	}
	r := l.snapshot()
	if len(r) != 1 || r[0].Count != latencyWindow || r[0].P99 != 100 {
		t.Fatalf("report = %+v, want only the last %d samples (all 100 ms)", r, latencyWindow)
	}
}

func TestLatencyReport(t *testing.T) {
	l := NewLatency()
	for i := 1; i <= 100; i++ {
		l.Observe("GET /admin/today", time.Duration(i)*time.Millisecond)
	}
	l.Observe("GET /health", time.Millisecond)
	app := fiber.New()
	app.Get("/admin/metrics/latency", l.Report)
	res, _ := app.Test(httptest.NewRequest("GET", "/admin/metrics/latency", nil))
	b, _ := io.ReadAll(res.Body)
	var body struct {
		Data []LatencyRow `json:"data"`
	}
	if err := json.Unmarshal(b, &body); err != nil || len(body.Data) != 2 {
		t.Fatalf("status %d body %s", res.StatusCode, b)
	}
	top := body.Data[0]
	if top.Route != "GET /admin/today" || top.Count != 100 || top.P50 != 50 || top.P95 != 95 || top.P99 != 99 {
		t.Errorf("slowest row = %+v", top)
	}
}
