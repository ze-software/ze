package runner

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"
)

// Display manages test status output.
type Display struct {
	tests     *Tests
	colors    *Colors
	output    io.Writer
	quiet     bool
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
			failedTests = append(failedTests, fmt.Sprintf("%s:%s", nick, r.Name))
		case StateTimeout:
			timedOut++
			timedOutTests = append(timedOutTests, fmt.Sprintf("%s:%s", nick, r.Name))
		case StateRunning, StateStarting:
			running++
			runningTests = append(runningTests, fmt.Sprintf("%s:%s", nick, r.Name))
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
				pendingTests = append(pendingTests, fmt.Sprintf("%s:%s", nick, r.Name))
			}
		}
	}

	// Build status parts
	var parts []string

	// Batch indicator (only if parallel < total, meaning we're batching)
	completed := passed + failed + timedOut
	if d.parallel > 0 && d.parallel < d.total {
		totalBatches := (d.total + d.parallel - 1) / d.parallel // ceil division
		currentBatch := (completed / d.parallel) + 1
		if currentBatch > totalBatches {
			currentBatch = totalBatches
		}
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
		_, _ = fmt.Fprint(d.output, "\r\033[K"+status+d.colors.Reset()+"\r")
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
		_, _ = fmt.Fprintln(d.output)
	}
	// Non-TTY: no status line to move past (BuildStatus already ended with newline)
}

// FinalStatus prints the final test status (for non-TTY mode).
func (d *Display) FinalStatus() {
	if d.quiet || d.colors.Enabled() {
		return // TTY mode uses in-place updates, quiet mode shows nothing
	}

	d.tests.mu.RLock()
	defer d.tests.mu.RUnlock()

	var passed, failed, timedOut int
	var failedTests, timedOutTests []string

	for _, nick := range d.tests.ordered {
		r := d.tests.byNick[nick]
		switch r.State {
		case StateSuccess:
			passed++
		case StateFail:
			failed++
			failedTests = append(failedTests, fmt.Sprintf("%s:%s", nick, r.Name))
		case StateTimeout:
			timedOut++
			timedOutTests = append(timedOutTests, fmt.Sprintf("%s:%s", nick, r.Name))
		case StateNone, StateSkip, StateStarting, StateRunning:
			// Not terminal states - ignore
		}
	}

	// Build status parts
	var parts []string
	parts = append(parts, fmt.Sprintf("passed %d", passed))

	if failed > 0 {
		failedStr := fmt.Sprintf("failed %d", failed)
		if len(failedTests) > 0 {
			shown := failedTests
			if len(shown) > 5 {
				shown = shown[:5]
			}
			failedStr += fmt.Sprintf(" [%s]", strings.Join(shown, ", "))
		}
		parts = append(parts, failedStr)
	}

	if timedOut > 0 {
		timedOutStr := fmt.Sprintf("timed out %d", timedOut)
		if len(timedOutTests) > 0 {
			shown := timedOutTests
			if len(shown) > 5 {
				shown = shown[:5]
			}
			timedOutStr += fmt.Sprintf(" [%s]", strings.Join(shown, ", "))
		}
		parts = append(parts, timedOutStr)
	}

	_, _ = fmt.Fprintln(d.output, strings.Join(parts, " "))
}

// Printf prints formatted output.
func (d *Display) Printf(format string, args ...any) {
	_, _ = fmt.Fprintf(d.output, format, args...)
}

// Summary prints the test summary.
func (d *Display) Summary() {
	passed, failed, timedOut, skipped := d.tests.Summary()
	failedNicks := d.tests.FailedNicks()

	_, _ = fmt.Fprintln(d.output)
	_, _ = fmt.Fprintln(d.output, d.colors.DoubleSeparator())
	_, _ = fmt.Fprintln(d.output, "TEST SUMMARY")
	_, _ = fmt.Fprintln(d.output, d.colors.DoubleSeparator())

	if passed > 0 {
		_, _ = fmt.Fprintf(d.output, "%s    %d\n", d.colors.Green("passed"), passed)
	}
	if failed > 0 {
		_, _ = fmt.Fprintf(d.output, "%s    %d [%s]\n", d.colors.Red("failed"), failed, strings.Join(failedNicks, ", "))
	}
	if timedOut > 0 {
		_, _ = fmt.Fprintf(d.output, "%s %d\n", d.colors.Yellow("timed out"), timedOut)
	}
	if skipped > 0 {
		_, _ = fmt.Fprintf(d.output, "%s   %d\n", d.colors.Gray("skipped"), skipped)
	}

	_, _ = fmt.Fprintln(d.output, d.colors.DoubleSeparator())

	total := passed + failed + timedOut
	if total > 0 {
		rate := float64(passed) / float64(total) * 100
		_, _ = fmt.Fprintf(d.output, "Total: %d test(s) run, %.1f%% passed\n", total, rate)
	}
	_, _ = fmt.Fprintln(d.output)
}

// PortInfo prints port allocation info.
func (d *Display) PortInfo(pr PortRange, shifted bool) {
	if d.quiet {
		return
	}

	if shifted {
		_, _ = fmt.Fprintf(d.output, "%s %s (base in use, shifted)\n",
			d.colors.Yellow("ports:"), pr.String())
	} else {
		_, _ = fmt.Fprintf(d.output, "%s %s (%d tests)\n",
			d.colors.Cyan("ports:"), pr.String(), pr.Count)
	}
}

// UlimitInfo prints ulimit check info.
func (d *Display) UlimitInfo(check *LimitCheck) {
	if d.quiet {
		return
	}

	if check.Raised {
		_, _ = fmt.Fprintf(d.output, "%s raised to %d\n",
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
		_, _ = fmt.Fprint(d.output, d.colors.Cyan("building...")+" ")
	case err != nil:
		_, _ = fmt.Fprintf(d.output, "%s %v\n", d.colors.Red("build failed:"), err)
	default:
		if d.colors.Enabled() {
			// TTY: status updates will overwrite this line
			_, _ = fmt.Fprint(d.output, d.colors.Green("ready")+" ")
		} else {
			// Non-TTY: end line since no status updates follow
			_, _ = fmt.Fprintln(d.output, d.colors.Green("ready"))
		}
	}
}

// StressSummary prints stress test statistics.
func (d *Display) StressSummary(result *StressResult, count int) {
	stats := result.Stats

	_, _ = fmt.Fprintln(d.output)
	_, _ = fmt.Fprintln(d.output, d.colors.DoubleSeparator())
	_, _ = fmt.Fprintln(d.output, "STRESS TEST SUMMARY")
	_, _ = fmt.Fprintln(d.output, d.colors.DoubleSeparator())
	_, _ = fmt.Fprintf(d.output, "Iterations: %d\n\n", count)

	// Per-test stats header
	_, _ = fmt.Fprintf(d.output, "%-6s %6s %6s %6s %10s %10s %10s %7s\n",
		"Nick", "Pass", "Fail", "T/O", "Min", "Avg", "Max", "Rate")
	_, _ = fmt.Fprintln(d.output, strings.Repeat("-", 75))

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

		_, _ = fmt.Fprintf(d.output, "%-6s %6d %6d %6d %10s %10s %10s %s\n",
			nick,
			s.Passed, s.Failed, s.TimedOut,
			formatDuration(s.Min()),
			formatDuration(s.Avg()),
			formatDuration(s.Max()),
			rateStr)
	}

	_, _ = fmt.Fprintln(d.output, d.colors.DoubleSeparator())

	// Iteration timing summary
	if len(result.IterationDurations) > 0 {
		_, _ = fmt.Fprintf(d.output, "Iteration timing: min=%s avg=%s max=%s total=%s\n",
			formatDuration(result.IterationMin()),
			formatDuration(result.IterationAvg()),
			formatDuration(result.IterationMax()),
			formatDuration(result.TotalDuration))
	}

	// Total summary
	total := totalPassed + totalFailed + totalTimedOut
	if total > 0 {
		rate := float64(totalPassed) / float64(total) * 100
		_, _ = fmt.Fprintf(d.output, "Total: %d iterations, %d passed, %d failed, %d timed out (%.1f%% pass rate)\n",
			total, totalPassed, totalFailed, totalTimedOut, rate)
	}
	_, _ = fmt.Fprintln(d.output)
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
