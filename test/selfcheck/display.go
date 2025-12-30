package selfcheck

import (
	"fmt"
	"io"
	"os"
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

	var passed, failed, running, pending int
	var failedNicks, pendingNicks []string
	var maxRunningElapsed time.Duration

	now := time.Now()
	for _, nick := range d.tests.ordered {
		r := d.tests.byNick[nick]
		switch r.State {
		case StateSuccess:
			passed++
		case StateFail, StateTimeout:
			failed++
			failedNicks = append(failedNicks, nick)
		case StateRunning, StateStarting:
			running++
			// Track longest running test
			if !r.StartTime.IsZero() {
				elapsed := now.Sub(r.StartTime)
				if elapsed > maxRunningElapsed {
					maxRunningElapsed = elapsed
				}
			}
		case StateSkip:
			// Skipped tests don't count toward pending
		case StateNone:
			if r.Active {
				pending++
				if len(pendingNicks) < 5 {
					pendingNicks = append(pendingNicks, nick)
				}
			}
		}
	}

	// Build status parts
	var parts []string

	// Timeout countdown (show longest running test vs timeout)
	if d.timeout > 0 && running > 0 {
		elapsedSec := int(maxRunningElapsed.Seconds())
		timeoutSec := int(d.timeout.Seconds())
		parts = append(parts, fmt.Sprintf("%s [%d/%s%d%s]",
			d.colors.Cyan("timeout"),
			elapsedSec,
			d.colors.Yellow(""),
			timeoutSec,
			d.colors.Reset()))
	}

	// Running count
	if running > 0 {
		parts = append(parts, fmt.Sprintf("%s %d", d.colors.Cyan("running"), running))
	}

	// Passed count
	parts = append(parts, fmt.Sprintf("%s %d", d.colors.Green("passed"), passed))

	// Failed count with IDs
	if failed > 0 {
		failedStr := fmt.Sprintf("%s %d", d.colors.Red("failed"), failed)
		if len(failedNicks) > 0 && len(failedNicks) <= 10 {
			failedStr += fmt.Sprintf(" [%s]", strings.Join(failedNicks, ", "))
		}
		parts = append(parts, failedStr)
	}

	// Pending count with IDs (if small)
	if pending > 0 {
		pendingStr := fmt.Sprintf("%s %d", d.colors.Yellow("pending"), pending)
		if len(pendingNicks) > 0 && pending <= 5 {
			pendingStr += fmt.Sprintf(" [%s]", strings.Join(pendingNicks, ", "))
		}
		parts = append(parts, pendingStr)
	}

	status := strings.Join(parts, " ")

	// Clear line and print status
	_, _ = fmt.Fprint(d.output, "\r\033[K"+status+d.colors.Reset()+"\r")
}

// Newline prints a newline to move past the status line.
func (d *Display) Newline() {
	if !d.quiet {
		_, _ = fmt.Fprintln(d.output)
	}
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
		_, _ = fmt.Fprint(d.output, d.colors.Green("ready")+" ")
	}
}
