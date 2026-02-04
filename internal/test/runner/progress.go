package runner

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// Progress tracks test execution progress with real-time display.
// Thread-safe for use with parallel test execution.
type Progress struct {
	mu sync.Mutex

	total     int
	running   int
	passed    int
	failed    int
	completed int

	runningNames []string
	failedNames  []string

	startTime   time.Time
	avgDuration time.Duration // Rolling average of completed test durations
	durations   []time.Duration

	colors  *Colors
	output  io.Writer
	quiet   bool
	started bool
}

// NewProgress creates a progress tracker for the given total test count.
func NewProgress(total int) *Progress {
	return &Progress{
		total:     total,
		colors:    NewColors(),
		output:    os.Stdout,
		durations: make([]time.Duration, 0, total),
	}
}

// SetQuiet enables quiet mode (no progress output).
func (p *Progress) SetQuiet(quiet bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.quiet = quiet
}

// SetOutput sets the output writer.
func (p *Progress) SetOutput(w io.Writer) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.output = w
}

// Start marks the beginning of test execution.
func (p *Progress) Start() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.startTime = time.Now()
	p.started = true
}

// StartTest marks a test as starting.
func (p *Progress) StartTest(name string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.running++
	p.runningNames = append(p.runningNames, name)
}

// EndTest marks a test as completed.
func (p *Progress) EndTest(name string, passed bool, duration time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Remove from running
	p.running--
	for i, n := range p.runningNames {
		if n == name {
			p.runningNames = append(p.runningNames[:i], p.runningNames[i+1:]...)
			break
		}
	}

	// Update counts
	p.completed++
	if passed {
		p.passed++
	} else {
		p.failed++
		p.failedNames = append(p.failedNames, name)
	}

	// Update average duration
	p.durations = append(p.durations, duration)
	var total time.Duration
	for _, d := range p.durations {
		total += d
	}
	p.avgDuration = total / time.Duration(len(p.durations))
}

// Status prints the current progress status on a single line.
// Uses \r\033[K to update in place on TTY.
func (p *Progress) Status() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.quiet {
		return
	}

	var parts []string

	// Progress fraction [12/45]
	parts = append(parts, fmt.Sprintf("[%d/%d]", p.completed, p.total))

	// ETA if we have enough data
	if p.avgDuration > 0 && p.completed > 0 {
		remaining := p.total - p.completed
		eta := time.Duration(remaining) * p.avgDuration
		// Account for parallel execution (rough estimate)
		if p.running > 0 {
			eta /= time.Duration(p.running)
		}
		if eta > 0 {
			parts = append(parts, fmt.Sprintf("ETA %s", formatETA(eta)))
		}
	}

	// Running count with names (if <= 3)
	if p.running > 0 {
		runningStr := fmt.Sprintf("%s %d", p.colors.Cyan("running"), p.running)
		if p.running <= 3 && len(p.runningNames) > 0 {
			runningStr += fmt.Sprintf(" [%s]", strings.Join(p.runningNames, ", "))
		}
		parts = append(parts, runningStr)
	}

	// Passed count
	parts = append(parts, fmt.Sprintf("%s %d", p.colors.Green("passed"), p.passed))

	// Failed count with names
	if p.failed > 0 {
		failedStr := fmt.Sprintf("%s %d", p.colors.Red("failed"), p.failed)
		if len(p.failedNames) > 0 && len(p.failedNames) <= 3 {
			failedStr += fmt.Sprintf(" [%s]", strings.Join(p.failedNames, ", "))
		}
		parts = append(parts, failedStr)
	}

	// Pending count
	pending := p.total - p.completed - p.running
	if pending > 0 {
		parts = append(parts, fmt.Sprintf("%s %d", p.colors.Gray("pending"), pending))
	}

	status := strings.Join(parts, " ")

	// Update in place for TTY, skip for non-TTY (final summary will show)
	if p.colors.Enabled() {
		fmt.Fprint(p.output, "\r\033[K"+status+"\r") //nolint:errcheck // terminal output
	}
}

// Newline prints a newline to move past the status line.
func (p *Progress) Newline() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.quiet {
		return
	}
	if p.colors.Enabled() {
		fmt.Fprintln(p.output) //nolint:errcheck // terminal output
	}
}

// Summary prints the final summary.
func (p *Progress) Summary(testType string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.quiet {
		fmt.Fprintf(p.output, "\n%s: %d passed, %d failed\n", testType, p.passed, p.failed) //nolint:errcheck // terminal output
		return
	}

	// Clear the status line first
	if p.colors.Enabled() {
		fmt.Fprint(p.output, "\r\033[K") //nolint:errcheck // terminal output
	}

	// Final status line
	elapsed := time.Since(p.startTime)
	fmt.Fprintf(p.output, "\n%s: %d passed, %d failed (%s)\n", //nolint:errcheck // terminal output
		testType, p.passed, p.failed, elapsed.Truncate(time.Millisecond))

	// List failures if any
	if len(p.failedNames) > 0 {
		fmt.Fprintf(p.output, "%s: %s\n", //nolint:errcheck // terminal output
			p.colors.Red("failed"), strings.Join(p.failedNames, ", "))
	}
}

// Passed returns the number of passed tests.
func (p *Progress) Passed() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.passed
}

// Failed returns the number of failed tests.
func (p *Progress) Failed() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.failed
}

// formatETA formats a duration for ETA display.
func formatETA(d time.Duration) string {
	if d < time.Second {
		return "<1s"
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
}
