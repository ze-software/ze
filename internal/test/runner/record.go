// Design: docs/architecture/testing/ci-format.md — test record types and state
// Related: record_collection.go — Tests container and querying
// Related: record_parse.go — CI file parsing and EncodingTests discovery

package runner

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/slogutil"
)

var recordLogger = slogutil.LazyLogger("test.record")

// State represents a test's execution state.
type State int

// State string constants.
const (
	stateNone     = "none"
	stateSkip     = "skip"
	stateStarting = "starting"
	stateRunning  = "running"
	stateSuccess  = "success"
	stateFail     = "fail"
	stateTimeout  = "timeout"
	stateUnknown  = "unknown"
)

// FailureType constants.
const (
	FailTypeMismatch         = "mismatch"
	FailTypeJSONMismatch     = "json_mismatch"
	FailTypeLoggingMismatch  = "logging_mismatch"
	FailTypeConnectionRefuse = "connection_refused"
)

const (
	StateNone     State = iota // Not started
	StateSkip                  // Explicitly skipped
	StateStarting              // About to start
	StateRunning               // Currently executing
	StateSuccess               // Passed
	StateFail                  // Failed
	StateTimeout               // Timed out
)

// String returns the state name.
func (s State) String() string {
	switch s {
	case StateNone:
		return stateNone
	case StateSkip:
		return stateSkip
	case StateStarting:
		return stateStarting
	case StateRunning:
		return stateRunning
	case StateSuccess:
		return stateSuccess
	case StateFail:
		return stateFail
	case StateTimeout:
		return stateTimeout
	default:
		return stateUnknown
	}
}

// MessageExpect holds an expected message in multiple formats.
type MessageExpect struct {
	Index   int    // Message number (1, 2, 3...)
	Cmd     string // Human-readable API command (if present)
	Raw     []byte // Wire format bytes
	RawHex  string // Wire format as hex string
	JSON    string // JSON representation (if present)
	Decoded string // Human-readable decoded (generated from Raw)
}

// Record holds test configuration and state.
type Record struct {
	Name      string
	Nick      string
	Port      int
	State     State
	Active    bool
	StartTime time.Time
	Duration  time.Duration
	Error     error

	// Files
	CIFile     string
	ConfigFile string
	Files      []string

	// Configuration from .ci file
	Options []string
	Extra   map[string]string
	Conf    map[string]any

	// Expected messages (multiple formats)
	Messages []MessageExpect

	// Legacy expects (raw strings for backward compat)
	Expects []string

	// API test specific
	IsAPI   bool
	RunFile string

	// Failure details
	FailureType     string // "mismatch", "timeout", "connection_refused"
	ReceivedRaw     []string
	LastExpectedIdx int
	LastReceivedIdx int
	PeerOutput      string
	ClientOutput    string

	// Logging test options
	EnvVars      []string // option:env:var=KEY:value=VALUE
	ExpectStderr []string // expect=stderr:pattern=PATTERN (regex)
	RejectStderr []string // reject=stderr:pattern=PATTERN (regex)
	ExpectSyslog []string // expect=syslog:pattern=PATTERN (regex)
	RejectSyslog []string // reject=syslog:pattern=PATTERN (regex)
	SyslogPort   int      // Dynamically assigned port for test-syslog

	// Exit code validation
	ExpectExitCode    *int     // expect:exit:code=N - expected exit code (nil = don't check)
	ExpectStderrMatch string   // expect=stderr:contains=TEXT - substring match (not regex)
	ExpectStdoutMatch []string // expect=stdout:contains=TEXT - substring match (not regex), multiple allowed

	// Tmpfs embedded files
	TmpfsFiles   map[string][]byte // path -> content from tmpfs= blocks
	TmpfsTempDir string            // temp directory for tmpfs files (set during execution)

	// Stdin blocks for process orchestration
	StdinBlocks map[string][]byte // name -> content from stdin= blocks

	// Run commands for process orchestration
	RunCommands []RunCommand // run= commands in order

	// HTTP checks for web endpoint assertions
	HTTPChecks []HTTPCheck // http= assertions in seq order
}

// RunCommand represents a process to run during test execution.
type RunCommand struct {
	Mode    string // "background" or "foreground"
	Seq     int    // Execution order (lower first)
	Exec    string // Command to execute
	Stdin   string // Name of stdin block to pipe
	Timeout string // Timeout for foreground processes (e.g., "10s")
}

// HTTPCheck represents an HTTP request assertion in a .ci test.
// Format: http=get:seq=N:url=URL:status=CODE[:contains=TEXT]
// Executed after all cmd= processes start, with retry+backoff for startup.
type HTTPCheck struct {
	Seq      int    // Execution order (lower first, among HTTP checks)
	Method   string // HTTP method: "get" or "post"
	URL      string // Request URL (supports $PORT substitution)
	Status   int    // Expected HTTP status code
	Contains string // Expected body substring (optional, empty = skip body check)
}

// NewRecord creates a new test record.
func NewRecord(name string) *Record {
	return &Record{
		Name:   name,
		Nick:   GenerateNick(name),
		Extra:  make(map[string]string),
		Conf:   make(map[string]any),
		Active: false,
		State:  StateNone,
	}
}

// nickIndex tracks used nicks to ensure uniqueness.
var (
	nickIndex int
	nickMu    sync.Mutex
)

// GenerateNick generates a unique short nick for a test.
func GenerateNick(_ string) string {
	nickMu.Lock()
	defer nickMu.Unlock()

	// Use single character for short nicks: 0-9, A-Z, a-z
	chars := "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	idx := nickIndex
	nickIndex++

	if idx < len(chars) {
		return string(chars[idx])
	}
	// Use numeric for large test suites
	return fmt.Sprintf("%d", idx)
}

// ResetNickCounter resets the nick counter (for testing).
func ResetNickCounter() {
	nickMu.Lock()
	defer nickMu.Unlock()
	nickIndex = 0
}

// Activate marks the test for execution.
func (r *Record) Activate() {
	r.Active = true
}

// Deactivate marks the test to be skipped.
func (r *Record) Deactivate() {
	r.Active = false
	r.State = StateSkip
}

// IsActive returns true if the test should run.
func (r *Record) IsActive() bool {
	return r.Active
}

// Colored returns the nick with ANSI color based on state.
func (r *Record) Colored() string {
	const (
		reset  = "\033[0m"
		red    = "\033[91m"
		green  = "\033[92m"
		yellow = "\033[93m"
		cyan   = "\033[96m"
		gray   = "\033[90m"
	)

	switch r.State { //nolint:exhaustive // default handles StateNone, StateStarting
	case StateSuccess:
		return green + r.Nick + reset
	case StateFail:
		return red + r.Nick + reset
	case StateTimeout:
		return yellow + r.Nick + reset
	case StateRunning:
		return cyan + r.Nick + reset
	case StateSkip:
		return gray + r.Nick + reset
	default:
		return r.Nick
	}
}

// GetMessage returns the message at the given index (1-based).
func (r *Record) GetMessage(idx int) *MessageExpect {
	for i := range r.Messages {
		if r.Messages[i].Index == idx {
			return &r.Messages[i]
		}
	}
	return nil
}

// getOrCreateMessage returns or creates a message at the given index.
func (r *Record) getOrCreateMessage(idx int) *MessageExpect {
	for i := range r.Messages {
		if r.Messages[i].Index == idx {
			return &r.Messages[i]
		}
	}
	msg := MessageExpect{Index: idx}
	r.Messages = append(r.Messages, msg)
	// Sort by index
	sort.Slice(r.Messages, func(i, j int) bool {
		return r.Messages[i].Index < r.Messages[j].Index
	})
	// Return the newly added message
	for i := range r.Messages {
		if r.Messages[i].Index == idx {
			return &r.Messages[i]
		}
	}
	return nil
}

// ConnectionOffset returns the message index offset for API tests based on Nick.
// Nick "A" or "1" -> 0, "B" or "2" -> 100, "C" or "3" -> 200, etc.
// Only applies to API tests with single-letter connection identifiers (A, B, C, D).
func (r *Record) ConnectionOffset() int {
	// Only apply for API tests with single-letter Nick (multi-connection tests)
	if !r.IsAPI || len(r.Nick) != 1 {
		return 0
	}
	first := r.Nick[0]
	// Only A-D are valid connection letters
	if first >= 'A' && first <= 'D' {
		return int(first-'A') * 100
	}
	if first >= 'a' && first <= 'd' {
		return int(first-'a') * 100
	}
	return 0
}

// ReceivedMessageOffset returns the offset into ReceivedRaw for the current connection.
// For connection C, this counts messages from A and B to find where C's messages start.
// Only applies to API tests with single-letter connection identifiers.
func (r *Record) ReceivedMessageOffset() int {
	// Only apply for API tests with single-letter Nick (multi-connection tests)
	if !r.IsAPI || len(r.Nick) != 1 {
		return 0
	}
	first := r.Nick[0]
	var connIdx int
	// Only A-D are valid connection letters
	if first >= 'A' && first <= 'D' {
		connIdx = int(first - 'A')
	} else if first >= 'a' && first <= 'd' {
		connIdx = int(first - 'a')
	}
	if connIdx == 0 {
		return 0
	}

	// Count messages from preceding connections
	offset := 0
	for _, exp := range r.Expects {
		parts := strings.SplitN(exp, ":", 2)
		if len(parts) < 2 {
			continue
		}
		prefix := parts[0]
		if prefix == "" {
			continue
		}
		expFirst := prefix[0]
		var expConn int
		if expFirst >= 'A' && expFirst <= 'D' {
			expConn = int(expFirst - 'A')
		} else if expFirst >= 'a' && expFirst <= 'd' {
			expConn = int(expFirst - 'a')
		}
		// Count raw messages from connections before the current one
		if expConn < connIdx && strings.Contains(exp, ":raw:") {
			offset++
		}
	}
	return offset
}
