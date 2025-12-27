package functional

import (
	"sync"
	"time"
)

// Nick character sequence: 0-9, A-Z, a-z (62 total).
// Matches ExaBGP's qa/bin/functional for consistency.
const nickChars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

var (
	nickIndex int
	nickMu    sync.Mutex
)

// ResetNickCounter resets the nick counter for testing purposes.
func ResetNickCounter() {
	nickMu.Lock()
	nickIndex = 0
	nickMu.Unlock()
}

// nextNick returns the next nick in sequence.
func nextNick() string {
	nickMu.Lock()
	defer nickMu.Unlock()
	if nickIndex >= len(nickChars) {
		// Wrap around, though 62 tests should be plenty
		nickIndex = 0
	}
	nick := string(nickChars[nickIndex])
	nickIndex++
	return nick
}

// Record represents a single test case with metadata and state.
type Record struct {
	Nick      string            // Single-char identifier (0-9, A-Z, a-z)
	Name      string            // Full test name (from filename)
	State     State             // Current lifecycle state
	Conf      map[string]any    // Test configuration
	Files     []string          // Related source files (for --edit)
	StartTime time.Time         // When test entered Running state
	Timeout   time.Duration     // Per-test timeout (default: very long)
	Port      int               // Port number for this test
	Options   []string          // Options parsed from .ci file
	Expects   []string          // Expected messages from .ci file
	IsAPI     bool              // True for API tests (have .run script)
	Extra     map[string]string // Additional parsed options
}

// NewRecord creates a new test record with automatic nick assignment.
func NewRecord(name string) *Record {
	return &Record{
		Nick:    nextNick(),
		Name:    name,
		State:   StateSkip, // Initially skipped until selected
		Conf:    make(map[string]any),
		Files:   nil,
		Timeout: 999999 * time.Second, // Effectively disabled, use global timeout
		Extra:   make(map[string]string),
	}
}

// Activate marks the test as selected (state = None).
func (r *Record) Activate() *Record {
	r.State = StateNone
	return r
}

// Deactivate marks the test as skipped.
func (r *Record) Deactivate() *Record {
	r.State = StateSkip
	return r
}

// Setup advances the test state: None -> Starting -> Running.
// Called twice to fully transition from None to Running.
func (r *Record) Setup() {
	switch r.State { //nolint:exhaustive // Only handle relevant states
	case StateNone:
		r.State = StateStarting
	case StateStarting:
		r.State = StateRunning
		r.StartTime = time.Now()
	}
}

// Running sets state to Running and records start time.
func (r *Record) Running() {
	r.State = StateRunning
	if r.StartTime.IsZero() {
		r.StartTime = time.Now()
	}
}

// Result sets the terminal state based on success.
func (r *Record) Result(success bool) bool {
	if success {
		r.State = StateSuccess
	} else {
		r.State = StateFail
	}
	return success
}

// HasTimedOut returns true if the test has exceeded its timeout.
func (r *Record) HasTimedOut() bool {
	if r.StartTime.IsZero() {
		return false
	}
	return time.Since(r.StartTime) > r.Timeout
}

// MarkTimeout sets state to Timeout.
func (r *Record) MarkTimeout() *Record {
	r.State = StateTimeout
	return r
}

// IsActive returns true if the test is in an active (non-terminal) state.
func (r *Record) IsActive() bool {
	return r.State.IsActive()
}

// Colored returns the nick wrapped in ANSI color codes based on state.
func (r *Record) Colored() string {
	return r.State.Color() + r.Nick + colorReset
}

// Duration returns elapsed time since test started, or zero if not started.
func (r *Record) Duration() time.Duration {
	if r.StartTime.IsZero() {
		return 0
	}
	return time.Since(r.StartTime)
}
