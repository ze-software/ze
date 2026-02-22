// Design: docs/architecture/testing/ci-format.md — test runner framework

package runner

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	// clearLine: carriage return + erase to end of line (repositions cursor and clears).
	clearLine = "\r\033[K"
	// cr: carriage return only (repositions cursor to column 0, no erase).
	cr = "\r"
)

// Display manages test status output.
type Display struct {
	tests     *Tests
	colors    *Colors
	output    io.Writer
	quiet     bool
	label     string // test suite label (e.g., "encode", "plugin")
	startTime time.Time
	timeout   time.Duration
	parallel  int // for batch display (0 = all at once)
	total     int // total test count
}

// NewDisplay creates a new display.
func NewDisplay(tests *Tests, colors *Colors) *Display {
	return &Display{
		tests:  tests,
		colors: colors,
		output: os.Stdout,
	}
}

// SetLabel sets the test suite label (e.g., "encode", "plugin").
func (d *Display) SetLabel(label string) {
	d.label = label
}

// SetQuiet enables quiet mode.
func (d *Display) SetQuiet(quiet bool) {
	d.quiet = quiet
}

// SetOutput sets the output writer.
func (d *Display) SetOutput(w io.Writer) {
	d.output = w
}

// SetTimeout sets the test timeout for display.
func (d *Display) SetTimeout(timeout time.Duration) {
	d.timeout = timeout
}

// SetParallel sets the parallel count for batch display.
func (d *Display) SetParallel(parallel, total int) {
	d.parallel = parallel
	d.total = total
}

// headerLine builds a left-aligned header line matching the summary format.
// Output: ═══ encode ═════════════════════════════════════════════════════════════
// Aligns label position with PASS/FAIL in summary lines.
func headerLine(colors *Colors, label string) string {
	prefix := "═══ "
	l := label + " "
	// Use rune count, not byte length — ═ is 3 UTF-8 bytes but 1 visual column.
	padRight := max(0, summaryWidth-utf8.RuneCountInString(prefix)-len(l))
	return colors.Cyan(prefix) + l + colors.Cyan(strings.Repeat("═", padRight))
}

// Header prints a section header for the test suite.
func (d *Display) Header() {
	if d.quiet || d.label == "" {
		return
	}
	d.println("")
	d.println(headerLine(d.colors, d.label))
}

// PrintHeader prints a section header without needing a Display.
func PrintHeader(label string) {
	colors := NewColors()
	fmt.Println()
	fmt.Println(headerLine(colors, label))
}

// Start marks the beginning of test execution.
func (d *Display) Start() {
	d.startTime = time.Now()
}

// Status shows current test status on a single line.
func (d *Display) Status() {
	if d.quiet {
		return
	}

	d.tests.mu.RLock()
	defer d.tests.mu.RUnlock()

	var passed, failed, timedOut, running, pending int
	var failedTests, timedOutTests, runningTests, pendingTests []string
	var maxRunningElapsed time.Duration
	maxRunningTimeout := d.timeout

	now := time.Now()
	for _, nick := range d.tests.ordered {
		r := d.tests.byNick[nick]
		switch r.State {
		case StateSuccess:
			passed++
		case StateFail:
			failed++
			failedTests = append(failedTests, nick)
		case StateTimeout:
			timedOut++
			timedOutTests = append(timedOutTests, nick)
		case StateRunning, StateStarting:
			running++
			runningTests = append(runningTests, nick)
			if !r.StartTime.IsZero() {
				elapsed := now.Sub(r.StartTime)
				if elapsed > maxRunningElapsed {
					maxRunningElapsed = elapsed
				}
			}
			// Track max timeout of running tests
			if timeoutStr, ok := r.Extra["timeout"]; ok {
				if t, err := time.ParseDuration(timeoutStr); err == nil && t > maxRunningTimeout {
					maxRunningTimeout = t
				}
			}
		case StateSkip:
			// Skipped tests don't count toward pending
		case StateNone:
			if r.Active {
				pending++
				pendingTests = append(pendingTests, nick)
			}
		}
	}

	// Build status parts
	var parts []string

	// Batch indicator (only if parallel < total, meaning we're batching)
	completed := passed + failed + timedOut
	if d.parallel > 0 && d.parallel < d.total {
		totalBatches := (d.total + d.parallel - 1) / d.parallel // ceil division
		currentBatch := min((completed/d.parallel)+1, totalBatches)
		parts = append(parts, fmt.Sprintf("batch[%d/%d]", currentBatch, totalBatches))
	}

	// Timer: longest running test elapsed vs max timeout of running tests
	if running > 0 && maxRunningTimeout > 0 {
		elapsed := int(maxRunningElapsed.Seconds())
		timeout := int(maxRunningTimeout.Seconds())
		parts = append(parts, fmt.Sprintf("[%d/%ds]", elapsed, timeout))
	}

	// Passed count
	parts = append(parts, fmt.Sprintf("%s %d", d.colors.Green("passed"), passed))

	// Running count with test names when <= 5
	if running > 0 {
		runningStr := fmt.Sprintf("%s %d", d.colors.Cyan("running"), running)
		if running <= 5 && len(runningTests) > 0 {
			runningStr += fmt.Sprintf(" [%s]", strings.Join(runningTests, ", "))
		}
		parts = append(parts, runningStr)
	}

	// Failed tests with nick:name
	if failed > 0 {
		failedStr := fmt.Sprintf("%s %d", d.colors.Red("failed"), failed)
		if len(failedTests) > 0 {
			shown := failedTests
			if len(shown) > 3 {
				shown = shown[:3]
			}
			failedStr += fmt.Sprintf(" [%s]", strings.Join(shown, ", "))
		}
		parts = append(parts, failedStr)
	}

	// Timed out tests with nick:name
	if timedOut > 0 {
		timedOutStr := fmt.Sprintf("%s %d", d.colors.Yellow("timed out"), timedOut)
		if len(timedOutTests) > 0 {
			shown := timedOutTests
			if len(shown) > 3 {
				shown = shown[:3]
			}
			timedOutStr += fmt.Sprintf(" [%s]", strings.Join(shown, ", "))
		}
		parts = append(parts, timedOutStr)
	}

	// Pending count (show names when <= 5 remaining)
	if pending > 0 {
		pendingStr := fmt.Sprintf("%s %d", d.colors.Gray("pending"), pending)
		if pending <= 5 {
			pendingStr += fmt.Sprintf(" [%s]", strings.Join(pendingTests, ", "))
		}
		parts = append(parts, pendingStr)
	}

	status := strings.Join(parts, " ")

	// Clear line and print status
	if d.colors.Enabled() {
		// TTY mode: update in place
		d.print(clearLine + status + d.colors.Reset() + cr)
	}
	// Non-TTY: skip intermediate updates (final summary will show results)
}

// Newline prints a newline to move past the status line.
func (d *Display) Newline() {
	if d.quiet {
		return
	}
	if d.colors.Enabled() {
		// TTY: move past the in-place status line
		d.println("")
	}
	// Non-TTY: no status line to move past (BuildStatus already ended with newline)
}

// Printf prints formatted output.
func (d *Display) Printf(format string, args ...any) {
	fmt.Fprintf(d.output, format, args...) //nolint:errcheck // display output
}

// println writes a line to the display output.
func (d *Display) println(s string) {
	fmt.Fprintln(d.output, s) //nolint:errcheck // display output
}

// print writes to the display output without a trailing newline.
func (d *Display) print(s string) {
	fmt.Fprint(d.output, s) //nolint:errcheck // display output
}

// summaryWidth is the total character width of the summary line.
const summaryWidth = 79

// Summary prints a single-line test summary that is easy to scan for humans and parse for tools.
//
// Format:
//
//	═══ PASS  42/42  100.0%  3.2s
//	═══ FAIL  40/42  95.2%  3.2s  failed 2 [A, B]  timeout 1 [C]
func (d *Display) Summary() {
	passed, failed, timedOut, _ := d.tests.Summary()
	total := passed + failed + timedOut
	if total == 0 {
		return
	}

	elapsed := time.Since(d.startTime)
	allPassed := failed == 0 && timedOut == 0
	rate := float64(passed) / float64(total) * 100

	var b strings.Builder

	b.WriteString(d.colors.Cyan("═══ "))

	if allPassed {
		b.WriteString(d.colors.Green("PASS"))
	} else {
		b.WriteString(d.colors.Red("FAIL"))
	}

	fmt.Fprintf(&b, "  %d/%d  %.1f%%  %s", passed, total, rate, formatDuration(elapsed))

	if failed > 0 {
		nicks := d.tests.FailedNicks()
		fmt.Fprintf(&b, "  %s %d [%s]", d.colors.Red("failed"), failed, strings.Join(nicks, ", "))
	}

	if timedOut > 0 {
		nicks := d.tests.TimedOutNicks()
		fmt.Fprintf(&b, "  %s %d [%s]", d.colors.Yellow("timeout"), timedOut, strings.Join(nicks, ", "))
	}

	d.println("")
	d.println(b.String())
}

// PortInfo prints port allocation info.
func (d *Display) PortInfo(pr PortRange, shifted bool) {
	if d.quiet {
		return
	}

	if shifted {
		d.Printf("%s %s (base in use, shifted)\n",
			d.colors.Yellow("ports:"), pr.String())
	} else {
		d.Printf("%s %s (%d tests)\n",
			d.colors.Cyan("ports:"), pr.String(), pr.Count)
	}
}

// UlimitInfo prints ulimit check info.
func (d *Display) UlimitInfo(check *LimitCheck) {
	if d.quiet {
		return
	}

	if check.Raised {
		d.Printf("%s raised to %d\n",
			d.colors.Yellow("ulimit:"), check.RaisedTo)
	}
}

// BuildStatus prints build status.
func (d *Display) BuildStatus(building bool, err error) {
	if d.quiet {
		return
	}

	switch {
	case building:
		d.print(d.colors.Cyan("building...") + " ")
	case err != nil:
		d.Printf("%s %v\n", d.colors.Red("build failed:"), err)
	default:
		if d.colors.Enabled() {
			// TTY: status updates will overwrite this line
			d.print(d.colors.Green("ready") + " ")
		} else {
			// Non-TTY: end line since no status updates follow
			d.println(d.colors.Green("ready"))
		}
	}
}

// StressSummary prints stress test statistics.
func (d *Display) StressSummary(result *StressResult, count int) {
	stats := result.Stats

	d.println("")
	d.println(d.colors.DoubleSeparator())
	d.println("STRESS TEST SUMMARY")
	d.println(d.colors.DoubleSeparator())
	d.Printf("Iterations: %d\n\n", count)

	// Per-test stats header
	d.Printf("%-6s %6s %6s %6s %10s %10s %10s %7s\n",
		"Nick", "Pass", "Fail", "T/O", "Min", "Avg", "Max", "Rate")
	d.println(strings.Repeat("-", 75))

	// Collect nicks in order
	nicks := make([]string, 0, len(stats))
	for nick := range stats {
		nicks = append(nicks, nick)
	}
	sort.Strings(nicks)

	var totalPassed, totalFailed, totalTimedOut int

	for _, nick := range nicks {
		s := stats[nick]
		if s.Total() == 0 {
			continue // Skip tests that weren't run
		}

		totalPassed += s.Passed
		totalFailed += s.Failed
		totalTimedOut += s.TimedOut

		// Color the pass rate based on value
		rate := s.PassRate()
		var rateStr string
		switch {
		case rate == 100:
			rateStr = d.colors.Green(fmt.Sprintf("%6.1f%%", rate))
		case rate >= 50:
			rateStr = d.colors.Yellow(fmt.Sprintf("%6.1f%%", rate))
		default:
			rateStr = d.colors.Red(fmt.Sprintf("%6.1f%%", rate))
		}

		d.Printf("%-6s %6d %6d %6d %10s %10s %10s %s\n",
			nick,
			s.Passed, s.Failed, s.TimedOut,
			formatDuration(s.Min()),
			formatDuration(s.Avg()),
			formatDuration(s.Max()),
			rateStr)
	}

	d.println(d.colors.DoubleSeparator())

	// Iteration timing summary
	if len(result.IterationDurations) > 0 {
		d.Printf("Iteration timing: min=%s avg=%s max=%s total=%s\n",
			formatDuration(result.IterationMin()),
			formatDuration(result.IterationAvg()),
			formatDuration(result.IterationMax()),
			formatDuration(result.TotalDuration))
	}

	// Total summary
	total := totalPassed + totalFailed + totalTimedOut
	if total > 0 {
		rate := float64(totalPassed) / float64(total) * 100
		d.Printf("Total: %d iterations, %d passed, %d failed, %d timed out (%.1f%% pass rate)\n",
			total, totalPassed, totalFailed, totalTimedOut, rate)
	}
	d.println("")
}

// DebugHints prints commands to rerun failed tests individually.
// Called after Summary when there are failures.
func (d *Display) DebugHints() {
	if d.quiet || d.label == "" {
		return
	}

	failed := d.tests.FailedRecords()
	if len(failed) == 0 {
		return
	}

	d.println("")
	d.println(d.colors.Yellow("To run failed tests individually:"))
	for _, rec := range failed {
		if d.label == "editor" {
			d.Printf("  ze-test editor -p %s\n", rec.Name)
		} else {
			d.Printf("  ze-test bgp %s %s\n", d.label, rec.Nick)
		}
	}
	d.Printf("\n  %s\n", d.colors.Gray("Add -v for verbose output"))
	d.println("")
}

// formatDuration formats a duration for display.
func formatDuration(d time.Duration) string {
	if d == 0 {
		return "-"
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}
