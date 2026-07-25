// Design: docs/architecture/testing/ci-format.md — test runner framework
// Related: timing.go — timing baseline and slow test detection

package runner

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	// clearLine: carriage return + erase to end of line (repositions cursor and clears).
	clearLine = "\r\033[K"
	// cr: carriage return only (repositions cursor to column 0, no erase).
	cr = "\r"
)

// nonTTYStatusInterval is how often non-TTY progress lines are emitted.
const nonTTYStatusInterval = 5 * time.Second

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

	lastNonTTYStatus time.Time // rate-limit non-TTY progress lines
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
	var tb textbuf.Buffer
	l := tb.Str(label).Byte(' ').String()
	// Use rune count, not byte length — ═ is 3 UTF-8 bytes but 1 visual column.
	padRight := max(0, summaryWidth-utf8.RuneCountInString(prefix)-len(l))
	return tb.Reset().Str(colors.Cyan(prefix)).Str(l).Str(colors.Cyan(strings.Repeat("═", padRight))).String()
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

	var passed, failed, timedOut, skipped, running, pending int
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
			skipped++
		case StateNone:
			if r.Active {
				pending++
				pendingTests = append(pendingTests, nick)
			}
		}
	}

	// Build status parts
	var parts []string

	completed := passed + failed + timedOut + skipped
	total := d.total
	if total == 0 {
		total = completed + running + pending
	}
	var bProgress textbuf.Buffer
	parts = append(parts, bProgress.Reset().Str("progress ").Int(int64(completed)).Byte('/').Int(int64(total)).String())

	// Batch indicator (only if parallel < total, meaning we're batching)
	if d.parallel > 0 && d.parallel < total {
		totalBatches := (total + d.parallel - 1) / d.parallel // ceil division
		currentBatch := min((completed/d.parallel)+1, totalBatches)
		var b textbuf.Buffer
		parts = append(parts, b.Reset().Str("batch[").Int(int64(currentBatch)).Byte('/').Int(int64(totalBatches)).Str("]").String())
	}

	// Timer: longest running test elapsed vs max timeout of running tests
	if running > 0 && maxRunningTimeout > 0 {
		elapsed := int(maxRunningElapsed.Seconds())
		timeout := int(maxRunningTimeout.Seconds())
		var b2 textbuf.Buffer
		parts = append(parts, b2.Reset().Byte('[').Int(int64(elapsed)).Byte('/').Int(int64(timeout)).Str("s]").String())
	}

	// Passed count
	var bPassed textbuf.Buffer
	parts = append(parts, bPassed.Reset().Str(d.colors.Green("passed")).Byte(' ').Int(int64(passed)).String())

	// Running count with test names when <= 5
	if running > 0 {
		var bRunning textbuf.Buffer
		bRunning.Str(d.colors.Cyan("running")).Byte(' ').Int(int64(running))
		if running <= 5 && len(runningTests) > 0 {
			bRunning.Str(" [").Join(runningTests, ", ").Byte(']')
		}
		parts = append(parts, bRunning.String())
	}

	// Failed tests with nick:name
	if failed > 0 {
		var bFailed textbuf.Buffer
		bFailed.Str(d.colors.Red("failed")).Byte(' ').Int(int64(failed))
		if len(failedTests) > 0 {
			shown := failedTests
			if len(shown) > 3 {
				shown = shown[:3]
			}
			bFailed.Str(" [").Join(shown, ", ").Byte(']')
		}
		parts = append(parts, bFailed.String())
	}

	// Timed out tests with nick:name
	if timedOut > 0 {
		var bTimeout textbuf.Buffer
		bTimeout.Str(d.colors.Yellow("timed out")).Byte(' ').Int(int64(timedOut))
		if len(timedOutTests) > 0 {
			shown := timedOutTests
			if len(shown) > 3 {
				shown = shown[:3]
			}
			bTimeout.Str(" [").Join(shown, ", ").Byte(']')
		}
		parts = append(parts, bTimeout.String())
	}

	// Pending count (show names when <= 5 remaining)
	if pending > 0 {
		var bPending textbuf.Buffer
		bPending.Str(d.colors.Gray("pending")).Byte(' ').Int(int64(pending))
		if pending <= 5 {
			bPending.Str(" [").Join(pendingTests, ", ").Byte(']')
		}
		parts = append(parts, bPending.String())
	}

	status := textbuf.Join(parts, " ")

	// Clear line and print status
	if d.colors.Enabled() {
		d.print(clearLine + status + d.colors.Reset() + cr)
	} else if running > 0 && now.Sub(d.lastNonTTYStatus) >= nonTTYStatusInterval {
		d.lastNonTTYStatus = now
		maxParallel := d.parallel
		if maxParallel <= 0 {
			maxParallel = total
		}
		var b textbuf.Buffer
		b.Str(padRight(formatDuration(now.Sub(d.startTime)), 7))
		b.Str(padLeft(textbuf.StringInt(int64(completed)), 4)).Byte('/').Str(padRight(textbuf.StringInt(int64(total)), 4))
		b.Str("  ").Str(padLeft(textbuf.StringInt(int64(running)), 2)).Byte('/').Str(padRight(textbuf.StringInt(int64(maxParallel)), 2))
		b.Str(" running  ")
		for i, nick := range runningTests {
			if i > 0 {
				b.Str(", ")
			}
			r := d.tests.byNick[nick]
			elapsed := now.Sub(r.StartTime)
			b.Str(nick).Byte('(').Str(formatDuration(elapsed)).Byte(')')
		}
		d.println(b.String())
	}
}

// TestFinished emits a per-test result line so logs and TTY runs show every
// completion with its suite ordinal, elapsed time, outcome, nick, and name.
func (d *Display) TestFinished(nick string, state State, elapsed time.Duration) {
	if d.quiet {
		return
	}
	var tag string
	switch state {
	case StateSuccess:
		tag = "PASS"
	case StateFail:
		tag = "FAIL"
	case StateTimeout:
		tag = "TIME"
	case StateSkip:
		tag = "SKIP"
	default:
		return
	}
	ordinal, total, name := d.testProgress(nick)
	var b textbuf.Buffer
	b.Str(padRight(formatDuration(elapsed), 7))
	if ordinal > 0 && total > 0 {
		b.Str("  ").Int(int64(ordinal)).Byte('/').Int(int64(total))
	}
	b.Str("  ").Str(tag).Str("  ").Str(nick)
	if name != "" && name != nick {
		b.Str("  ").Str(name)
	}
	if d.colors.Enabled() {
		d.print(clearLine)
	}
	d.println(b.String())
}

func (d *Display) testProgress(nick string) (int, int, string) {
	d.tests.mu.RLock()
	defer d.tests.mu.RUnlock()

	total := 0
	ordinal := 0
	name := ""
	for _, candidate := range d.tests.ordered {
		rec := d.tests.byNick[candidate]
		if !rec.Active {
			continue
		}
		total++
		if rec.Nick == nick || rec.Name == nick {
			ordinal = total
			name = rec.Name
		}
	}
	return ordinal, total, name
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
//	pass  42/42  100.0%  3.2s
//	fail  40/42  95.2%  3.2s  failed 2 [A, B]  timeout 1 [C]
func (d *Display) Summary() {
	passed, failed, timedOut, skipped := d.tests.Summary()
	total := passed + failed + timedOut
	if total == 0 && skipped == 0 {
		return
	}

	elapsed := time.Since(d.startTime)
	allPassed := failed == 0 && timedOut == 0
	rate := 100.0
	if total > 0 {
		rate = float64(passed) / float64(total) * 100
	}

	var b textbuf.Buffer

	if allPassed {
		b.Str(d.colors.Green("pass"))
	} else {
		b.Str(d.colors.Red("fail"))
	}

	b.Str("  ").Int(int64(passed)).Byte('/').Int(int64(total)).Str("  ").Float(rate, 1).Str("%  ").Str(formatDuration(elapsed))

	if failed > 0 {
		nicks := d.tests.failedNicks()
		b.Str("  ").Str(d.colors.Red("failed")).Byte(' ').Int(int64(failed)).Str(" [").Join(nicks, ", ").Byte(']')
	}

	if timedOut > 0 {
		nicks := d.tests.timedOutNicks()
		b.Str("  ").Str(d.colors.Yellow("timeout")).Byte(' ').Int(int64(timedOut)).Str(" [").Join(nicks, ", ").Byte(']')
	}

	if skipped > 0 {
		nicks := d.tests.skippedNicks()
		b.Str("  ").Str(d.colors.Gray("skip")).Byte(' ').Int(int64(skipped)).Str(" [").Join(nicks, ", ").Byte(']')
	}

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

// buildStatus prints build status.
func (d *Display) buildStatus(building bool, err error) {
	if d.quiet {
		return
	}

	switch {
	case building:
		var tb textbuf.Buffer
		d.print(tb.Str(d.colors.Cyan("building...")).Byte(' ').String())
	case err != nil:
		d.Printf("%s %v\n", d.colors.Red("build failed:"), err)
	default:
		if d.colors.Enabled() {
			// TTY: status updates will overwrite this line
			var tb2 textbuf.Buffer
			d.print(tb2.Str(d.colors.Green("ready")).Byte(' ').String())
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
		"ID", "Pass", "Fail", "T/O", "Min", "Avg", "Max", "Rate")
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

// timingDetail prints per-test timing and flags slow tests.
// Called after Summary to show timing baseline comparison.
func (d *Display) timingDetail(suite string, timings Timings) {
	if d.quiet {
		return
	}

	d.tests.mu.RLock()
	var records []*Record
	for _, nick := range d.tests.ordered {
		r := d.tests.byNick[nick]
		if r.Duration > 0 {
			records = append(records, r)
		}
	}
	d.tests.mu.RUnlock()

	if len(records) == 0 {
		return
	}

	if line := FormatTimingLine(suite, records, timings, d.colors); line != "" {
		d.println(line)
	}
	if slow := FormatSlowTests(suite, records, timings, d.colors); slow != "" {
		d.print(slow)
	}
}

// debugHints prints commands to rerun failed tests individually.
// Called after Summary when there are failures.
func (d *Display) debugHints() {
	if d.quiet || d.label == "" {
		return
	}

	failed := d.tests.failedRecords()
	if len(failed) == 0 {
		return
	}

	d.println("")
	d.println(d.colors.Yellow("To run failed tests individually:"))
	for _, rec := range failed {
		d.Printf("  %s\n", formatRecordRerunCommand(d.label, rec))
	}
	d.Printf("\n  %s\n", d.colors.Gray("Add -v for verbose output"))
	d.println("")
}

func padLeft(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return strings.Repeat(" ", width-len(s)) + s
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

// formatDuration formats a duration for display.
func formatDuration(d time.Duration) string {
	if d == 0 {
		return "-"
	}
	if d < time.Second {
		return textbuf.IntStr(d.Milliseconds(), "ms")
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}
