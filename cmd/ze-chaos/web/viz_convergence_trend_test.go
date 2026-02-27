package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestConvergenceTrendRecord verifies pushing latencies into the rolling buffer.
//
// VALIDATES: AC-1 — rolling buffer contains raw latency values.
// PREVENTS: Latency values not being recorded.
func TestConvergenceTrendRecord(t *testing.T) {
	t.Parallel()

	rb := NewRingBuffer[time.Duration](100)
	rb.Push(10 * time.Millisecond)
	rb.Push(20 * time.Millisecond)
	rb.Push(30 * time.Millisecond)

	if rb.Len() != 3 {
		t.Errorf("expected 3 items, got %d", rb.Len())
	}
	items := rb.All()
	if items[0] != 10*time.Millisecond {
		t.Errorf("expected 10ms, got %v", items[0])
	}
}

// TestConvergenceTrendPercentiles verifies p50/p90/p99 computation.
//
// VALIDATES: AC-2 — percentiles computed correctly from known dataset.
// PREVENTS: Wrong percentile calculation.
func TestConvergenceTrendPercentiles(t *testing.T) {
	t.Parallel()

	rb := NewRingBuffer[time.Duration](100)
	// Push 100 values: 1ms, 2ms, ..., 100ms.
	for i := 1; i <= 100; i++ {
		rb.Push(time.Duration(i) * time.Millisecond)
	}

	p := ComputeConvergencePercentiles(rb)
	if p.Count != 100 {
		t.Errorf("count: expected 100, got %d", p.Count)
	}
	// p50 = index 50 of sorted 100 items = 51ms.
	if p.P50 != 51*time.Millisecond {
		t.Errorf("p50: expected 51ms, got %v", p.P50)
	}
	// p90 = index 90 = 91ms.
	if p.P90 != 91*time.Millisecond {
		t.Errorf("p90: expected 91ms, got %v", p.P90)
	}
	// p99 = index 99 = 100ms.
	if p.P99 != 100*time.Millisecond {
		t.Errorf("p99: expected 100ms, got %v", p.P99)
	}
}

// TestConvergenceTrendPercentilesEmpty verifies empty buffer returns zero.
//
// VALIDATES: AC-3 — empty buffer handled gracefully.
// PREVENTS: Panic or wrong values with no data.
func TestConvergenceTrendPercentilesEmpty(t *testing.T) {
	t.Parallel()

	rb := NewRingBuffer[time.Duration](100)
	p := ComputeConvergencePercentiles(rb)
	if p.Count != 0 || p.P50 != 0 || p.P90 != 0 || p.P99 != 0 {
		t.Errorf("expected all zeros for empty buffer, got %+v", p)
	}
}

// TestConvergenceTrendPercentilesOne verifies single value gives p50=p90=p99=value.
//
// VALIDATES: Boundary — single item in buffer.
// PREVENTS: Off-by-one in percentile indexing.
func TestConvergenceTrendPercentilesOne(t *testing.T) {
	t.Parallel()

	rb := NewRingBuffer[time.Duration](100)
	rb.Push(42 * time.Millisecond)

	p := ComputeConvergencePercentiles(rb)
	if p.P50 != 42*time.Millisecond || p.P90 != 42*time.Millisecond || p.P99 != 42*time.Millisecond {
		t.Errorf("single value: expected all 42ms, got p50=%v p90=%v p99=%v", p.P50, p.P90, p.P99)
	}
}

// TestConvergenceTrendEviction verifies oldest values evicted when buffer full.
//
// VALIDATES: AC-4 — oldest values evicted; percentiles reflect only recent data.
// PREVENTS: Buffer growing unbounded or keeping stale data.
func TestConvergenceTrendEviction(t *testing.T) {
	t.Parallel()

	rb := NewRingBuffer[time.Duration](10)
	// Push 20 values (1ms..20ms) into a buffer of capacity 10.
	for i := 1; i <= 20; i++ {
		rb.Push(time.Duration(i) * time.Millisecond)
	}

	if rb.Len() != 10 {
		t.Errorf("expected 10 items after eviction, got %d", rb.Len())
	}
	// Only 11ms..20ms should remain.
	items := rb.All()
	if items[0] != 11*time.Millisecond {
		t.Errorf("oldest should be 11ms, got %v", items[0])
	}
}

// TestWriteConvergenceTrend verifies the render function produces bars and labels.
//
// VALIDATES: AC-2, AC-6, AC-7 — bars with proportional widths and labels.
// PREVENTS: Missing bars or labels in rendered HTML.
func TestWriteConvergenceTrend(t *testing.T) {
	t.Parallel()

	rb := NewRingBuffer[time.Duration](100)
	for i := 1; i <= 50; i++ {
		rb.Push(time.Duration(i) * time.Millisecond)
	}

	var buf strings.Builder
	writeConvergenceTrend(&buf, rb)
	html := buf.String()

	for _, needle := range []string{"p50", "p90", "p99", "trend-p50", "trend-p90", "trend-p99", "samples"} {
		if !strings.Contains(html, needle) {
			t.Errorf("output missing %q", needle)
		}
	}
}

// TestWriteConvergenceTrendEmpty verifies empty state message.
//
// VALIDATES: AC-3 — empty buffer shows awaiting message.
// PREVENTS: Empty chart with no explanation.
func TestWriteConvergenceTrendEmpty(t *testing.T) {
	t.Parallel()

	rb := NewRingBuffer[time.Duration](100)

	var buf strings.Builder
	writeConvergenceTrend(&buf, rb)
	html := buf.String()

	if !strings.Contains(html, "Awaiting") {
		t.Error("empty trend should show awaiting message")
	}
	if strings.Contains(html, "trend-p50") {
		t.Error("empty trend should not have bars")
	}
}

// TestWriteConvergenceTrendSSEAttributes verifies SSE swap attributes.
//
// VALIDATES: AC-5 — outer div has sse-swap="convergence-trend".
// PREVENTS: SSE updates not reaching the trend panel.
func TestWriteConvergenceTrendSSEAttributes(t *testing.T) {
	t.Parallel()

	rb := NewRingBuffer[time.Duration](100)
	rb.Push(10 * time.Millisecond)

	var buf strings.Builder
	writeConvergenceTrend(&buf, rb)
	html := buf.String()

	if !strings.Contains(html, `sse-swap="convergence-trend"`) {
		t.Error("missing sse-swap attribute")
	}
	if !strings.Contains(html, `id="viz-convergence-trend"`) {
		t.Error("missing id attribute")
	}
	if !strings.Contains(html, `hx-swap="outerHTML"`) {
		t.Error("missing hx-swap attribute")
	}
}

// TestWriteConvergenceTrendColors verifies CSS classes for percentile tiers.
//
// VALIDATES: AC-8 — p50 green, p90 yellow, p99 red.
// PREVENTS: Wrong color mapping.
func TestWriteConvergenceTrendColors(t *testing.T) {
	t.Parallel()

	rb := NewRingBuffer[time.Duration](100)
	for i := 1; i <= 10; i++ {
		rb.Push(time.Duration(i) * time.Millisecond)
	}

	var buf strings.Builder
	writeConvergenceTrend(&buf, rb)
	html := buf.String()

	if !strings.Contains(html, `class="trend-p50"`) {
		t.Error("missing trend-p50 class")
	}
	if !strings.Contains(html, `class="trend-p90"`) {
		t.Error("missing trend-p90 class")
	}
	if !strings.Contains(html, `class="trend-p99"`) {
		t.Error("missing trend-p99 class")
	}
}

// TestHandleVizConvergenceTrend verifies the HTTP handler.
//
// VALIDATES: AC-2, AC-5 — handler returns trend chart HTML.
// PREVENTS: Handler not wired or returning wrong content.
func TestHandleVizConvergenceTrend(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(5)
	defer d.broker.Close()

	// Push some data.
	d.state.ConvergenceTrend.Push(10 * time.Millisecond)
	d.state.ConvergenceTrend.Push(50 * time.Millisecond)

	req := httptest.NewRequest(http.MethodGet, "/viz/convergence-trend", http.NoBody)
	rec := httptest.NewRecorder()
	d.handleVizConvergenceTrend(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "convergence-trend") {
		t.Error("response missing convergence-trend identifier")
	}
	if !strings.Contains(body, "trend-p50") {
		t.Error("response missing trend bars")
	}
}

// TestFormatTrendDuration verifies duration formatting for chart labels.
//
// VALIDATES: AC-7 — labels show formatted duration values.
// PREVENTS: Wrong duration format in chart.
func TestFormatTrendDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		d    time.Duration
		want string
	}{
		{500 * time.Microsecond, "500µs"},
		{5 * time.Millisecond, "5ms"},
		{1500 * time.Millisecond, "1.5s"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()
			if got := formatTrendDuration(tt.d); got != tt.want {
				t.Errorf("formatTrendDuration(%v) = %q, want %q", tt.d, got, tt.want)
			}
		})
	}
}
