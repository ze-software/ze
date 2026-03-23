package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTimingRecord(t *testing.T) {
	timings := make(Timings)

	// First sample sets avg directly
	timings.Record("encode", "ipv4", 1000*time.Millisecond)
	entry := timings["encode"]["ipv4"]
	if entry.AvgMs != 1000 {
		t.Errorf("first sample avg: got %f, want 1000", entry.AvgMs)
	}
	if entry.Samples != 1 {
		t.Errorf("samples: got %d, want 1", entry.Samples)
	}

	// Second sample uses EMA (alpha=0.3)
	timings.Record("encode", "ipv4", 2000*time.Millisecond)
	entry = timings["encode"]["ipv4"]
	// EMA: 1000*0.7 + 2000*0.3 = 700 + 600 = 1300
	if entry.AvgMs < 1299 || entry.AvgMs > 1301 {
		t.Errorf("second sample avg: got %f, want ~1300", entry.AvgMs)
	}
	if entry.MaxMs != 2000 {
		t.Errorf("max: got %f, want 2000", entry.MaxMs)
	}
	if entry.Samples != 2 {
		t.Errorf("samples: got %d, want 2", entry.Samples)
	}
}

func TestTimingIsSlow(t *testing.T) {
	timings := make(Timings)

	// Need 3 samples before IsSlow activates
	timings.Record("encode", "ipv4", 1000*time.Millisecond)
	timings.Record("encode", "ipv4", 1000*time.Millisecond)
	slow, _ := timings.IsSlow("encode", "ipv4", 3000*time.Millisecond)
	if slow {
		t.Error("should not be slow with only 2 samples")
	}

	// Third sample enables detection
	timings.Record("encode", "ipv4", 1000*time.Millisecond)
	slow, expected := timings.IsSlow("encode", "ipv4", 3000*time.Millisecond)
	if !slow {
		t.Error("3000ms should be slow when avg is ~1000ms")
	}
	if expected < 900*time.Millisecond || expected > 1100*time.Millisecond {
		t.Errorf("expected baseline: got %v, want ~1000ms", expected)
	}

	// Normal duration is not slow
	slow, _ = timings.IsSlow("encode", "ipv4", 1500*time.Millisecond)
	if slow {
		t.Error("1500ms should not be slow when avg is ~1000ms (threshold is 2x)")
	}
}

func TestTimingIsSlowUnknownSuite(t *testing.T) {
	timings := make(Timings)
	slow, _ := timings.IsSlow("unknown", "test", 5000*time.Millisecond)
	if slow {
		t.Error("unknown suite should not report slow")
	}
}

func TestTimingSaveLoad(t *testing.T) {
	dir := t.TempDir()

	// Record and save
	timings := make(Timings)
	timings.Record("encode", "ipv4", 1000*time.Millisecond)
	timings.Record("encode", "ipv6", 500*time.Millisecond)
	if err := timings.Save(dir); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Load and verify
	loaded := LoadTimings(dir)
	if len(loaded["encode"]) != 2 {
		t.Fatalf("loaded entries: got %d, want 2", len(loaded["encode"]))
	}
	if loaded["encode"]["ipv4"].AvgMs != 1000 {
		t.Errorf("loaded ipv4 avg: got %f, want 1000", loaded["encode"]["ipv4"].AvgMs)
	}
	if loaded["encode"]["ipv6"].AvgMs != 500 {
		t.Errorf("loaded ipv6 avg: got %f, want 500", loaded["encode"]["ipv6"].AvgMs)
	}
}

func TestTimingIsSlowBoundary(t *testing.T) {
	timings := make(Timings)
	// 3 identical samples: avg = exactly 1000ms
	timings.Record("encode", "ipv4", 1000*time.Millisecond)
	timings.Record("encode", "ipv4", 1000*time.Millisecond)
	timings.Record("encode", "ipv4", 1000*time.Millisecond)

	// Exactly 2x threshold: 2000ms vs 1000ms avg - NOT slow (must exceed, not equal)
	slow, _ := timings.IsSlow("encode", "ipv4", 2000*time.Millisecond)
	if slow {
		t.Error("exactly 2x should not be slow (threshold is > 2x, not >=)")
	}

	// Just over 2x: slow
	slow, _ = timings.IsSlow("encode", "ipv4", 2001*time.Millisecond)
	if !slow {
		t.Error("2001ms should be slow when avg is 1000ms")
	}
}

func TestVisibleLen(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"hello", 5},
		{"", 0},
		{"\033[93mhello\033[0m", 5},              // yellow "hello"
		{"\033[91m2.1s!\033[0m", 5},              // red "2.1s!"
		{"0:1.2s", 6},                            // no color
		{"0:\033[93m1.2s!\033[0m", 7},            // "0:" + yellow "1.2s!"
		{"\033[93ma\033[0m \033[91mb\033[0m", 3}, // "a b" colored
	}
	for _, tt := range tests {
		got := visibleLen(tt.input)
		if got != tt.want {
			t.Errorf("visibleLen(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestSuggestedTimeout(t *testing.T) {
	timings := make(Timings)
	fallback := 15 * time.Second

	// No baseline: returns fallback
	got := timings.SuggestedTimeout("encode", "ipv4", fallback)
	if got != fallback {
		t.Errorf("no baseline: got %v, want %v", got, fallback)
	}

	// < 3 samples: returns fallback
	timings.Record("encode", "ipv4", 1000*time.Millisecond)
	timings.Record("encode", "ipv4", 1000*time.Millisecond)
	got = timings.SuggestedTimeout("encode", "ipv4", fallback)
	if got != fallback {
		t.Errorf("2 samples: got %v, want %v", got, fallback)
	}

	// 3 samples, avg=1000ms: derived = max(5s, 5*1s) = 5s < 15s fallback
	timings.Record("encode", "ipv4", 1000*time.Millisecond)
	got = timings.SuggestedTimeout("encode", "ipv4", fallback)
	if got != 5*time.Second {
		t.Errorf("1s avg: got %v, want 5s (floor)", got)
	}

	// Fast test (200ms avg): derived = max(5s, 5*200ms=1s) = 5s (floor wins)
	fast := make(Timings)
	fast.Record("encode", "fast", 200*time.Millisecond)
	fast.Record("encode", "fast", 200*time.Millisecond)
	fast.Record("encode", "fast", 200*time.Millisecond)
	got = fast.SuggestedTimeout("encode", "fast", fallback)
	if got != 5*time.Second {
		t.Errorf("200ms avg: got %v, want 5s (floor)", got)
	}

	// Slow test (4s avg): derived = max(5s, 5*4s=20s) = 20s > 15s fallback, clamped
	slow := make(Timings)
	slow.Record("encode", "slow", 4000*time.Millisecond)
	slow.Record("encode", "slow", 4000*time.Millisecond)
	slow.Record("encode", "slow", 4000*time.Millisecond)
	got = slow.SuggestedTimeout("encode", "slow", fallback)
	if got != fallback {
		t.Errorf("4s avg: got %v, want %v (clamped to fallback)", got, fallback)
	}
}

func TestSuggestedTimeoutBoundary(t *testing.T) {
	// derived == fallback: avg=3000ms -> derived = max(5s, 5*3s=15s) = 15s = fallback
	timings := make(Timings)
	timings.Record("encode", "test", 3000*time.Millisecond)
	timings.Record("encode", "test", 3000*time.Millisecond)
	timings.Record("encode", "test", 3000*time.Millisecond)
	got := timings.SuggestedTimeout("encode", "test", 15*time.Second)
	if got != 15*time.Second {
		t.Errorf("derived==fallback: got %v, want 15s", got)
	}
}

func TestVisibleLenMalformed(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"\033[93mhello", 5},    // unclosed CSI: "hello" visible
		{"\033hello", 6},        // lone ESC not followed by '[': ESC counted as visible + "hello"
		{"\033", 1},             // lone ESC at end: counted as visible
		{"\033[", 0},            // CSI start, no final byte, no visible chars
		{"\033[93m\033[91m", 0}, // two CSI sequences, no content
	}
	for _, tt := range tests {
		got := visibleLen(tt.input)
		if got != tt.want {
			t.Errorf("visibleLen(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestLoadTimingsCorruptJSON(t *testing.T) {
	dir := t.TempDir()
	// Write corrupt JSON
	path := filepath.Join(dir, timingsFile)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"bad json`), 0o644); err != nil {
		t.Fatal(err)
	}
	timings := LoadTimings(dir)
	if len(timings) != 0 {
		t.Errorf("corrupt JSON: expected empty timings, got %d suites", len(timings))
	}
}

func TestLoadTimingsNegativeValues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, timingsFile)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	// Write valid JSON with negative avg-ms
	data := `{"encode":{"bad":{"avg-ms":-100,"max-ms":50,"samples":5},"good":{"avg-ms":500,"max-ms":800,"samples":3}}}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	timings := LoadTimings(dir)
	if _, ok := timings["encode"]["bad"]; ok {
		t.Error("negative avg-ms entry should have been discarded")
	}
	if _, ok := timings["encode"]["good"]; !ok {
		t.Error("valid entry should have been kept")
	}
}

func TestFormatTimingLine(t *testing.T) {
	colors := NewColorsWithOverride(false) // no ANSI for assertions
	timings := make(Timings)

	// Empty records
	if got := FormatTimingLine("encode", nil, timings, colors); got != "" {
		t.Errorf("empty: got %q, want empty", got)
	}

	// Records with only non-terminal states: should return ""
	records := []*Record{
		{Nick: "0", Name: "test1", State: StateNone, Duration: 100 * time.Millisecond},
		{Nick: "1", Name: "test2", State: StateSkip, Duration: 200 * time.Millisecond},
	}
	if got := FormatTimingLine("encode", records, timings, colors); got != "" {
		t.Errorf("non-terminal: got %q, want empty", got)
	}

	// Records with terminal states
	records = []*Record{
		{Nick: "0", Name: "test1", State: StateSuccess, Duration: 1200 * time.Millisecond},
		{Nick: "1", Name: "test2", State: StateFail, Duration: 800 * time.Millisecond},
		{Nick: "2", Name: "test3", State: StateTimeout, Duration: 5 * time.Second},
	}
	got := FormatTimingLine("encode", records, timings, colors)
	if got == "" {
		t.Error("expected non-empty timing line")
	}
	if !strings.Contains(got, "0:1.2s") {
		t.Errorf("missing test1 timing in %q", got)
	}
	if !strings.Contains(got, "1:800ms") {
		t.Errorf("missing test2 timing in %q", got)
	}
}

func TestFormatSlowTests(t *testing.T) {
	colors := NewColorsWithOverride(false)
	timings := make(Timings)

	// No baseline: no slow tests
	records := []*Record{
		{Nick: "0", Name: "test1", State: StateSuccess, Duration: 5 * time.Second},
	}
	if got := FormatSlowTests("encode", records, timings, colors); got != "" {
		t.Errorf("no baseline: got %q, want empty", got)
	}

	// With baseline: 3 samples of 500ms, then 1500ms should be slow (>2x)
	timings.Record("encode", "test1", 500*time.Millisecond)
	timings.Record("encode", "test1", 500*time.Millisecond)
	timings.Record("encode", "test1", 500*time.Millisecond)

	records = []*Record{
		{Nick: "0", Name: "test1", State: StateSuccess, Duration: 1500 * time.Millisecond},
		{Nick: "1", Name: "test2", State: StateSuccess, Duration: 200 * time.Millisecond},
	}
	got := FormatSlowTests("encode", records, timings, colors)
	if !strings.Contains(got, "slow:") {
		t.Errorf("expected slow header, got %q", got)
	}
	if !strings.Contains(got, "test1") {
		t.Errorf("expected test1 in slow list, got %q", got)
	}
	if strings.Contains(got, "test2") {
		t.Errorf("test2 should not be in slow list (no baseline)")
	}
}

func TestJoinWrap(t *testing.T) {
	// Empty
	if got := joinWrap(nil, 80); got != "" {
		t.Errorf("empty: got %q", got)
	}

	// Single part, fits
	if got := joinWrap([]string{"hello"}, 80); got != "hello" {
		t.Errorf("single: got %q", got)
	}

	// Multiple parts, all fit on one line
	got := joinWrap([]string{"a", "b", "c"}, 80)
	if got != "a b c" {
		t.Errorf("one line: got %q, want %q", got, "a b c")
	}

	// Parts that exceed maxWidth: should wrap
	got = joinWrap([]string{"aaaa", "bbbb", "cccc"}, 10)
	if !strings.Contains(got, "\n") {
		t.Errorf("expected line wrap in %q (maxWidth=10)", got)
	}

	// ANSI-colored parts: visible width should be used for wrapping
	yellow := "\033[93mtest!\033[0m" // 5 visible chars, 14 bytes
	parts := []string{yellow, yellow, yellow}
	got = joinWrap(parts, 20)
	// 3 parts of 5 visible chars + 2 spaces = 17, fits in 20
	if strings.Contains(got, "\n") {
		t.Errorf("ANSI: should not wrap (17 visible <= 20), got %q", got)
	}
}

func TestTimingLoadMissing(t *testing.T) {
	timings := LoadTimings("/nonexistent/path")
	if timings == nil {
		t.Error("LoadTimings should return empty map, not nil")
	}
	if len(timings) != 0 {
		t.Errorf("expected empty timings, got %d entries", len(timings))
	}
}
