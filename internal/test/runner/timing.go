// Design: docs/architecture/testing/ci-format.md — test timing baseline
// Related: display.go — timing display in summary output
// Related: runner.go — timing integration for encode/plugin tests

package runner

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const timingsFile = "tmp/test-timings.json"

// TimingEntry records the rolling average timing for a single test.
type TimingEntry struct {
	AvgMs   float64 `json:"avg-ms"`
	MaxMs   float64 `json:"max-ms"`
	Samples int     `json:"samples"`
}

// Timings maps suite -> test name -> timing entry.
type Timings map[string]map[string]TimingEntry

// maxTimingsFileSize is the maximum size of the timings JSON file (10 MB).
const maxTimingsFileSize = 10 << 20

// LoadTimings reads timings from the baseline file. Returns empty timings on error.
func LoadTimings(baseDir string) Timings {
	path := filepath.Join(baseDir, timingsFile)
	info, err := os.Stat(path)
	if err != nil {
		return make(Timings)
	}
	if info.Size() > maxTimingsFileSize {
		logger().Warn("timings file too large, starting fresh", "path", path, "size", info.Size())
		return make(Timings)
	}
	data, err := os.ReadFile(path) //nolint:gosec // project-local file
	if err != nil {
		return make(Timings)
	}
	var t Timings
	if err := json.Unmarshal(data, &t); err != nil {
		logger().Warn("timings file corrupt, starting fresh", "path", path, "error", err)
		return make(Timings)
	}
	// Discard entries with invalid values (negative avg, negative samples).
	for suite, tests := range t {
		for name, entry := range tests {
			if entry.AvgMs < 0 || entry.MaxMs < 0 || entry.Samples < 0 {
				delete(tests, name)
			}
		}
		if len(tests) == 0 {
			delete(t, suite)
		}
	}
	return t
}

// Save writes timings to the baseline file atomically (temp file + rename).
func (t Timings) Save(baseDir string) error {
	path := filepath.Join(baseDir, timingsFile)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "timings-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()        //nolint:errcheck,gosec // best-effort close on write error
		os.Remove(tmpName) //nolint:errcheck // best-effort cleanup
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName) //nolint:errcheck // best-effort cleanup
		return fmt.Errorf("close temp file: %w", err)
	}
	return os.Rename(tmpName, path)
}

// Record updates the rolling average for a test.
// Uses exponential moving average with alpha=0.3.
func (t Timings) Record(suite, name string, duration time.Duration) {
	if t[suite] == nil {
		t[suite] = make(map[string]TimingEntry)
	}
	ms := float64(duration.Milliseconds())
	entry := t[suite][name]

	if entry.Samples == 0 {
		entry.AvgMs = ms
		entry.MaxMs = ms
	} else {
		const alpha = 0.3
		entry.AvgMs = entry.AvgMs*(1-alpha) + ms*alpha
		entry.MaxMs = math.Max(entry.MaxMs, ms)
	}
	entry.Samples++
	t[suite][name] = entry
}

// IsSlow returns true if duration exceeds 2x the rolling average.
// Requires at least 3 samples to avoid false positives.
func (t Timings) IsSlow(suite, name string, duration time.Duration) (slow bool, expected time.Duration) {
	if t[suite] == nil {
		return false, 0
	}
	entry, ok := t[suite][name]
	if !ok || entry.Samples < 3 {
		return false, 0
	}
	ms := float64(duration.Milliseconds())
	threshold := entry.AvgMs * 2.0
	expected = time.Duration(entry.AvgMs) * time.Millisecond
	return ms > threshold, expected
}

// SuggestedTimeout returns a per-test timeout derived from baseline data.
// Uses min(fallback, max(floor, multiplier * baseline_avg)).
// Returns fallback if no baseline data exists (fewer than 3 samples).
func (t Timings) SuggestedTimeout(suite, name string, fallback time.Duration) time.Duration {
	const (
		multiplier = 5.0
		floor      = 5 * time.Second
	)
	if t[suite] == nil {
		return fallback
	}
	entry, ok := t[suite][name]
	if !ok || entry.Samples < 3 {
		return fallback
	}
	// The max(_, floor) also protects against int64 overflow from corrupted AvgMs:
	// huge AvgMs overflows to negative Duration, floor clamp catches it.
	derived := max(time.Duration(entry.AvgMs*multiplier)*time.Millisecond, floor)
	return min(derived, fallback)
}

// FormatTimingLine returns a compact single-line timing summary.
// Format: "timing: 0:1.2s 1:0.8s 2:2.1s ..."
// Slow tests are marked with color.
func FormatTimingLine(suite string, records []*Record, timings Timings, colors *Colors) string {
	if len(records) == 0 {
		return ""
	}

	var parts []string
	for _, rec := range records {
		if rec.State != StateSuccess && rec.State != StateFail && rec.State != StateTimeout {
			continue
		}
		dur := formatDuration(rec.Duration)
		slow, _ := timings.IsSlow(suite, rec.Name, rec.Duration)
		if slow {
			parts = append(parts, fmt.Sprintf("%s:%s", rec.Nick, colors.Yellow(dur+"!")))
		} else {
			parts = append(parts, fmt.Sprintf("%s:%s", rec.Nick, dur))
		}
	}

	if len(parts) == 0 {
		return ""
	}
	const prefix = "timing: " // 8 chars — joinWrap continuation indent matches this
	return fmt.Sprintf("%s %s", colors.Gray("timing:"), joinWrap(parts, summaryWidth-len(prefix)))
}

// FormatSlowTests returns detail lines for slow tests.
// Format: "slow: K addpath 2.1s (avg 1.0s)".
func FormatSlowTests(suite string, records []*Record, timings Timings, colors *Colors) string {
	var lines []string
	for _, rec := range records {
		if rec.State != StateSuccess && rec.State != StateFail && rec.State != StateTimeout {
			continue
		}
		slow, expected := timings.IsSlow(suite, rec.Name, rec.Duration)
		if slow {
			lines = append(lines, fmt.Sprintf("  %s %s %s (avg %s)",
				colors.Yellow(rec.Nick), rec.Name,
				colors.Yellow(formatDuration(rec.Duration)),
				formatDuration(expected)))
		}
	}
	if len(lines) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(colors.Yellow("slow:"))
	b.WriteByte('\n')
	for _, l := range lines {
		b.WriteString(l)
		b.WriteByte('\n')
	}
	return b.String()
}

// joinWrap joins strings with spaces, wrapping at maxWidth.
// Uses visibleLen to handle ANSI escape sequences correctly.
func joinWrap(parts []string, maxWidth int) string {
	if len(parts) == 0 {
		return ""
	}
	var b strings.Builder
	lineLen := 0
	for i, p := range parts {
		vlen := visibleLen(p)
		addition := vlen
		if i > 0 {
			addition++ // space
		}
		if lineLen+addition > maxWidth && lineLen > 0 {
			b.WriteString("\n        ") // indent continuation
			lineLen = 8
		} else if i > 0 {
			b.WriteByte(' ')
			lineLen++
		}
		b.WriteString(p)
		lineLen += vlen
	}
	return b.String()
}

// visibleLen returns the display width of a string, excluding ANSI escape sequences.
// ANSI sequences have the form ESC [ <params> <final byte> where final byte is 0x40-0x7E.
func visibleLen(s string) int {
	n := 0
	inEscape := false
	for i := 0; i < len(s); i++ {
		if inEscape {
			// End of CSI sequence: final byte in 0x40-0x7E range
			if s[i] >= 0x40 && s[i] <= 0x7E {
				inEscape = false
			}
			continue
		}
		if s[i] == '\033' && i+1 < len(s) && s[i+1] == '[' {
			inEscape = true
			i++ // skip '['
			continue
		}
		n++
	}
	return n
}
