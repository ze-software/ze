// Design: docs/architecture/testing/ci-format.md — test record types and state
// Detail: record_collection.go — Tests container and querying
// Detail: record_parse.go — CI file parsing and EncodingTests discovery

package runner

import (
	"sort"
	"strconv"
	"sync"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	"codeberg.org/thomas-mangin/ze/internal/test/trace"
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
	FailTypeStdoutMismatch   = "stdout_mismatch"
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

// messageExpect holds an expected message in multiple formats.
type messageExpect struct {
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
	Messages []messageExpect

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
	ExpectExitCode       *int     // expect:exit:code=N - expected exit code (nil = don't check)
	ExpectStderrMatch    []string // expect=stderr:contains=TEXT - substring match (not regex), multiple allowed
	ExpectStdoutMatch    []string // expect=stdout:contains=TEXT - substring match (not regex), multiple allowed
	ExpectStdoutNotMatch []string // expect=stdout:!contains=TEXT - stdout must NOT contain TEXT, multiple allowed
	ExpectStdoutRegex    []string // expect=stdout:pattern=PATTERN (regex)
	RejectStdoutRegex    []string // reject=stdout:pattern=PATTERN (regex)

	// Tmpfs embedded files
	TmpfsFiles   map[string][]byte // path -> content from tmpfs= blocks
	TmpfsTempDir string            // temp directory for tmpfs files (set during execution)

	// Stdin blocks for process orchestration
	StdinBlocks map[string][]byte // name -> content from stdin= blocks

	// Run commands for process orchestration
	RunCommands []RunCommand // run= commands in order

	// HTTP checks for web endpoint assertions
	HTTPChecks []httpCheck // http= assertions in seq order
	HTTPWaits  []httpCheck // http=wait readiness polls (run before checks)

	// Engine steps: command=/stream= + expect=output|event|stream directives
	// executed by the spawned `ze-test engine-steps` external plugin, fed via
	// engine-steps.json in the tmpfs dir (engine_steps.go).
	EngineSteps []EngineStep

	// File checks for post-run filesystem assertions.
	FileChecks []fileCheck

	// StepTrace records per-assertion outcomes for trace output.
	StepTrace []trace.StepResult

	// Skip reason: when non-empty, the runner skips the test without executing
	// it and reports the reason (e.g. option=skip-os:value=darwin on non-Linux
	// platforms). Set at parse time and persists across Activate() calls.
	SkipReason string

	// NeedsLinux is set when the test carries option=needs-linux: it requires a
	// real Linux kernel and is validated in the QEMU Alpine VM. Used by the
	// ZE_QEMU_LINUX_ONLY filter (the `ze-qemu-needs-linux-test` tight loop) to
	// run ONLY these tests and skip everything else.
	NeedsLinux bool

	// ParseFailed marks a .ci file that could not be parsed at discovery time.
	// Discover records the file as a permanent failure (State=StateFail, Error
	// set) and continues, so one unparseable file fails loudly without aborting
	// discovery of the rest of the suite. The runner short-circuits such records
	// without attempting execution.
	ParseFailed bool
}

// RunCommand represents a process to run during test execution.
type RunCommand struct {
	Mode    string // "background" or "foreground"
	Seq     int    // Execution order (lower first)
	Exec    string // Command to execute
	Stdin   string // Name of stdin block to pipe
	Timeout string // Timeout for foreground processes (e.g., "10s")
}

// httpCheck represents an HTTP request assertion in a .ci test.
// Format: http=get:seq=N:url=URL:status=CODE[:contains=TEXT]
// Format: http=post:seq=N:url=URL:status=CODE[:contains=TEXT][:sendfile=FILE][:content-type=TYPE][:insecure-tls=true]
// Format: http=wait:seq=N:url=URL:status=CODE[:contains=TEXT][:timeout=DUR]
// "get"/"post" checks are assertions; "wait" polls until the condition is met
// (retrying on both connection errors and content mismatches).
// Executed after all cmd= processes start, with retry+backoff for startup.
type httpCheck struct {
	Seq         int    // Execution order (lower first, among HTTP checks)
	Method      string // HTTP method: "get", "post", or "wait"
	URL         string // Request URL (supports $PORT substitution)
	Status      int    // Expected HTTP status code
	Contains    string // Expected body substring (optional, empty = skip body check)
	BodyFile    string // Path to file with expected body content (exact match)
	SendFile    string // Path to file whose content is sent as the POST request body
	ContentType string // Content-Type header for sendfile bodies (default application/json)
	InsecureTLS bool   // Accept self-signed TLS certificates for HTTPS test endpoints
	Timeout     string // Poll timeout for wait checks (default "15s")
}

// fileCheck represents an expect=file assertion in a .ci test.
// Path checks target one file. Glob checks target all files matching a pattern.
type fileCheck struct {
	Path        string
	Glob        string
	Contains    string
	NotContains string
	Exists      bool
	Absent      bool
	Count       *int
}

// newRecord creates a new test record.
func newRecord(name string) *Record {
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

// GenerateNick generates a unique one-based numeric id for a test.
func GenerateNick(_ string) string {
	nickMu.Lock()
	defer nickMu.Unlock()

	nickIndex++
	return strconv.Itoa(nickIndex)
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

// Deactivate marks the test as not selected for execution.
func (r *Record) Deactivate() {
	r.Active = false
	r.State = StateNone
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

// getMessage returns the message at the given index (1-based).
func (r *Record) getMessage(idx int) *messageExpect {
	for i := range r.Messages {
		if r.Messages[i].Index == idx {
			return &r.Messages[i]
		}
	}
	return nil
}

// getOrCreateMessage returns or creates a message at the given index.
func (r *Record) getOrCreateMessage(idx int) *messageExpect {
	for i := range r.Messages {
		if r.Messages[i].Index == idx {
			return &r.Messages[i]
		}
	}
	msg := messageExpect{Index: idx}
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
