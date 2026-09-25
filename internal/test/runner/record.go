// Design: docs/architecture/testing/ci-format.md — test record types and state
// Detail: record_collection.go — Tests container and querying
// Detail: record_parse.go — CI file parsing and EncodingTests discovery

package runner

import (
	"net/netip"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/test/tmpfs"
	"github.com/ze-software/ze/internal/test/trace"
)

// NetnsLinkSpec is one interface a test asks the runner to provision inside its
// per-test network namespace before ze launches (option=netns-link). It exists
// because some Linux-only tests match or route through an interface the daemon
// never creates itself: a policy-routing next-hop needs a connected route to
// resolve its gateway, and enterTestNetns brings up only loopback. A link with
// the given address gives the netns that connectivity without touching the host
// (provisioning is gated on netns mode, so the option is inert elsewhere).
type NetnsLinkSpec struct {
	Name string
	// Peer names the far end of a veth PAIR. Empty means a dummy link, which has
	// no far end at all: a dummy drops everything written to it, so two processes
	// binding AF_PACKET sockets to one can never exchange a frame. A test that
	// puts a daemon on one side of a real Ethernet segment and a client on the
	// other (PPPoE discovery, RFC 2516) needs the pair, and both ends live in the
	// same per-test namespace so a broadcast leaving Peer arrives on Name.
	Peer string
	// VLAN is an 802.1Q tag. Zero means no sub-interface. Non-zero additionally
	// creates <Name>.<VLAN> (and <Peer>.<VLAN> when Peer is set), which is how a
	// test proves a feature works on a tagged sub-interface rather than only on
	// the parent.
	VLAN uint16
	// Address is the CIDR assigned to the link. The zero value means create the
	// link and bring it up without an address.
	Address netip.Prefix
}

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

	// FailTypePeerNeverBound marks a ze-peer that exited (or hung) before it
	// printed its "listening on" readiness token. ze then dials a dead port and
	// backs off, which looks like a BGP establishment stall but is a harness
	// fault. Its own failure type keeps it from being grouped with a real
	// connection_refused against a live peer.
	FailTypePeerNeverBound = "peer_never_bound"

	// FailTypeBackgroundNeverReady marks a cmd=background process that did not
	// print its ready= text before the test budget ended. The next step would
	// have dialed a socket nobody served, so the test stops here and names the
	// process rather than failing later as a protocol stall.
	FailTypeBackgroundNeverReady = "background_never_ready"

	// FailTypeLoopbackMissing marks a test whose fixture binds an address this
	// host does not carry. The runner adds an IPv4 loopback alias itself where
	// it can, and can never add an IPv6 one, so this is an environment fault
	// rather than a product fault: rec.Error names the command that fixes it
	// (internal/test/runner/loopback.go).
	FailTypeLoopbackMissing = "loopback_address_missing"
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
	Name string
	Nick string
	Port int
	// State and StartTime are written by the goroutine that runs the test and
	// read by Display.Status on the status ticker, so every write goes through
	// SetState or Begin and Status reads through progress, all under
	// progressMu. The test's own goroutine MAY read them without the lock,
	// because it is their only writer.
	State      State
	Active     bool
	StartTime  time.Time
	progressMu sync.Mutex
	Duration   time.Duration
	Error      error

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
	// ClientStdout and ClientStderr are the two accumulators a command's span
	// indexes into (RunCommand.stdoutFrom/To). ClientOutput stays their
	// concatenation: it is what the report prints, and a reader asking what the
	// test produced wants it whole.
	ClientStdout string
	ClientStderr string
	// FailedPeers names the check-mode peers that did not report a clean
	// exchange. PeerOutput joins every peer's output and so cannot say WHICH
	// peer failed; in a multi-peer test that distinction is the whole diagnosis
	// (peer_contract.go failedCheckPeers).
	FailedPeers []string

	// Logging test options
	EnvVars      []string // option:env:var=KEY:value=VALUE
	ExpectStderr []string // expect=stderr:pattern=PATTERN (regex)
	RejectStderr []string // reject=stderr:pattern=PATTERN (regex)
	ExpectSyslog []string // expect=syslog:pattern=PATTERN (regex)
	RejectSyslog []string // reject=syslog:pattern=PATTERN (regex)
	SyslogPort   int      // Dynamically assigned port for test-syslog

	// Exit code validation
	ExpectExitCode *int // expect:exit:code=N - expected exit code (nil = don't check)

	// AwaitStderr, when non-empty, makes the runner BLOCK until the daemon's
	// relayed stderr contains this substring before it tears the daemon down --
	// a deterministic fence that replaces a blind time.sleep for tests observing
	// an external plugin's refuse/warn message. It exists for the reject-fence
	// bucket: a plugin whose refusal aborts plugin startup (StartupCoordinator.
	// PluginFailed) leaves no in-daemon observer able to poll it, so the only
	// non-plugin signal is the relayed stderr line itself. Parsed from
	// await=stderr:contains=TEXT[:timeout=DUR][:then=stop]. Several await lines
	// make one fence that holds until EVERY needle has appeared. Empty =
	// disabled (no behavior change).
	AwaitStderr        []string
	AwaitStderrTimeout string // optional Go duration (e.g. "10s"); empty = default
	// AwaitThenStop makes the runner itself stop the daemon once the fence
	// holds, instead of waiting for the peers to end the test. It exists for a
	// test whose peer lingers (option=linger), which never ends by itself, and
	// whose daemon was otherwise stopped by a fixed-delay SIGTERM that raced the
	// event under test. Set by :then=stop on any await line.
	AwaitThenStop bool

	// Tmpfs embedded files
	// TmpfsFiles maps a path to the file the tmpfs= block declared, MODE
	// included. It held content alone until 2026-09-03, and the mode was
	// dropped at this boundary, so the orchestrated path re-derived one from
	// the file extension and a declared `mode=<octal>` reached disk only when
	// it happened to agree.
	TmpfsFiles   map[string]tmpfs.File
	TmpfsTempDir string // temp directory for tmpfs files, empty when the record declares none (set during execution)

	// WorkDir is the directory every child of this test runs in, set during
	// execution and removed when the test ends. It exists for every record,
	// including one that declares no tmpfs files, because a child with no
	// directory of its own inherits the runner's, which is the repository root.
	// TmpfsTempDir answers a different question and keeps its narrower meaning:
	// whether the record DECLARED files. When it did, the two are the same
	// directory.
	WorkDir string

	// Stdin blocks for process orchestration
	StdinBlocks map[string][]byte // name -> content from stdin= blocks

	// zeConfigFiles maps each ze-daemon stdin block name to the tmpfs config
	// file written for it. A second concurrent `ze -` daemon in the same test
	// (e.g. an IKE responder+initiator pair) uses a distinct block name, so it
	// gets a distinct file instead of clobbering the first daemon's ze-bgp.conf.
	// The first block keeps the canonical ze-bgp.conf name that rewrite/restart
	// tests (action=rewrite:dest=ze-bgp.conf) target; reusing the same block
	// (a restart) reuses its file.
	zeConfigFiles map[string]string

	// Run commands for process orchestration
	RunCommands []RunCommand // run= commands in order

	// HTTP checks for web endpoint assertions
	HTTPChecks []httpCheck // http= assertions in seq order
	HTTPWaits  []httpCheck // http=wait readiness polls (run before checks)

	// Engine steps: command=/stream= + expect=output|event|stream directives
	// executed by the spawned `le-test engine-steps` external plugin, fed via
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
	// ZE_QEMU_LINUX_ONLY filter (the `./le test qemu all-tests` tight loop) to
	// run ONLY these tests and skip everything else.
	NeedsLinux bool

	// NetnsLinks are interfaces the runner provisions inside this test's per-test
	// network namespace (option=netns-link) before spawning ze. Populated at parse
	// time; consumed only under netns mode (ZE_TEST_NETNS), so the option is inert
	// on the default host path and never touches the host.
	NetnsLinks []NetnsLinkSpec

	// ExclusiveGroup names a set of tests that must never execute concurrently
	// with EACH OTHER (option=exclusive:group=<name>). Tests outside the group are
	// unaffected and still run alongside them, so this costs far less wall-clock
	// than dropping a whole suite to -p 1.
	//
	// Use it when tests contend for a kernel-global observation surface that
	// unique names/addresses cannot partition. The ddos suite is the motivating
	// case: every test floods the SAME loopback interface, and each daemon's
	// detector picks the top destination by bytes over that interface's counters,
	// so a sibling's flood is indistinguishable from the test's own.
	ExclusiveGroup string

	// ParseFailed marks a .ci file that could not be parsed at discovery time.
	// Discover records the file as a permanent failure (State=StateFail, Error
	// set) and continues, so one unparseable file fails loudly without aborting
	// discovery of the rest of the suite. The runner short-circuits such records
	// without attempting execution.
	ParseFailed bool
}

// spanOpen marks a command's output span as still being written. A background
// process is still producing when the run ends, and a command the run never
// reached produced nothing at all: closeOpenSpans tells the two apart, so an
// unreached command answers empty rather than inheriting every later command's
// output.
const spanOpen = -1

// RunCommand represents a process to run during test execution.
type RunCommand struct {
	Mode    string // "background", "foreground", or "stop"
	Seq     int    // Execution order (lower first)
	Exec    string // Command to execute (empty for the "stop" directive)
	Stdin   string // Name of stdin block to pipe
	Timeout string // Timeout for foreground processes (e.g., "10s")

	// Name is the optional handle a cmd=background line assigns to its process
	// (cmd=background:...:name=NAME). It is REQUIRED on a cmd=stop directive,
	// which names the previously-started background process to terminate. Empty
	// for an unnamed background process or any foreground command.
	Name string

	// Ready is the text a cmd=background process prints, on stdout or stderr,
	// once it can serve (cmd=background:...:ready=TEXT). The runner starts no
	// later step until the text appears, and fails the test when the budget
	// ends first. Empty means the process has no readiness barrier.
	Ready string

	// Signal selects how a cmd=stop directive terminates its target: "kill"
	// (SIGKILL, the default -- a peer that stops answering, needed by the DPD
	// proof) or "term" (SIGTERM, a graceful stop that lets the process flush and
	// send protocol teardown). Empty on non-stop commands.
	Signal string

	// The stream assertions authored UNDER this command: every expect=stdout,
	// reject=stdout and expect=stderr:contains / reject=stderr:contains line
	// between this cmd= line and the next one in the file. Each is checked
	// against this command's own output span and nothing else
	// (checkOutputAssertions), which is what makes the line mean what its
	// position says it means.
	//
	// They used to be six flat slices on Record, checked against the whole
	// test's accumulated stdout AND stderr. A file could therefore satisfy an
	// assertion with a DIFFERENT command's output, and two did:
	// test/plugin/kernel-capability-unknown-starts.ci asserted the doctor
	// printed doctor-ipsec-xfrm-unknown and then ran `ze explain
	// doctor-ipsec-xfrm-unknown`, whose output satisfied the assertion on a
	// host where the doctor check does not run at all.
	//
	// expect=stderr:pattern= and reject=stderr:pattern= are deliberately NOT
	// here. Those are the daemon-logging mechanism validateLogging owns; they
	// read the daemon's relayed stderr rather than a command's span, and they
	// stay on Record.
	ExpectStdout    []string // expect=stdout:contains=
	RejectStdout    []string // reject=stdout:contains=
	ExpectStdoutRe  []string // expect=stdout:pattern=
	RejectStdoutRe  []string // reject=stdout:pattern=
	ExpectStderrHas []string // expect=stderr:contains=
	RejectStderrHas []string // reject=stderr:contains=

	// The half-open byte span this command wrote into the run's two
	// accumulators. A span is MARKED rather than captured into a buffer of its
	// own: the accumulators already exist, already carry the truncation cap,
	// and are already what the report prints, so a second copy of every byte
	// would be a second declaration of the same output.
	//
	// stdoutTo and stderrTo are the accumulator lengths when the command
	// FINISHED; for a background process, which is still writing when the test
	// ends, they are the lengths at the end of the run. A background span can
	// therefore contain bytes a later foreground command wrote, because the two
	// share one accumulator and interleave in arrival order. That over-inclusion
	// is the honest answer for a process whose output has no end: 18 stream
	// assertions in the corpus sit under a background command, against 3339
	// under a foreground one.
	stdoutFrom, stdoutTo int
	stderrFrom, stderrTo int

	// ExitCode is the exit code asserted for THIS command (cmd=...:exit=N), as
	// opposed to the file-level expect=exit:code=, which only ever reaches the
	// last quick-exit ze command (Record.ExpectExitCode is a single value and
	// runOrchestrated compares it against lastQuickZeErr). A file that runs
	// several `ze config validate` commands therefore leaves every earlier one
	// unasserted; use exit= per command to assert each. nil = not asserted.
	ExitCode *int
}

// assertsAStream reports whether this command carries any scoped stream
// assertion.
func (c *RunCommand) assertsAStream() bool {
	return len(c.ExpectStdout) != 0 || len(c.RejectStdout) != 0 ||
		len(c.ExpectStdoutRe) != 0 || len(c.RejectStdoutRe) != 0 ||
		len(c.ExpectStderrHas) != 0 || len(c.RejectStderrHas) != 0
}

// hasStreamAssertion reports whether any command in this test asserts on a
// stream.
//
// DERIVED from the commands rather than kept beside them: a second field
// counting the same lines is a future disagreement with nothing to arbitrate
// it, and the callers that ask this question decide whether a test observes
// anything at all (accept_only.go, peer_contract.go), so the answer must be the
// same set checkOutputAssertions will check.
func (r *Record) hasStreamAssertion() bool {
	for i := range r.RunCommands {
		if r.RunCommands[i].assertsAStream() {
			return true
		}
	}
	return false
}

// httpHeader is one request header set by a :header=Name: Value key.
// Repeating the key on a single http= line yields several entries, kept in the
// order the keys appear on the line.
type httpHeader struct {
	Name  string // Field name, whitespace-trimmed (e.g. "MCP-Protocol-Version")
	Value string // Field value, whitespace-trimmed; may itself contain colons
}

// httpCheck represents an HTTP request assertion in a .ci test.
// Format: http=get:seq=N:url=URL:status=CODE[:contains=TEXT][:header=NAME: VALUE]...
// Format: http=post:seq=N:url=URL:status=CODE[:contains=TEXT][:sendfile=FILE][:content-type=TYPE][:header=NAME: VALUE]...[:insecure-tls=true]
// Format: http=wait:seq=N:url=URL:status=CODE[:contains=TEXT][:header=NAME: VALUE]...[:timeout=DUR]
// "get"/"post" checks are assertions; "wait" polls until the condition is met
// (retrying on both connection errors and content mismatches).
// Executed after all cmd= processes start, with retry+backoff for startup.
type httpCheck struct {
	Seq         int          // Execution order (lower first, among HTTP checks)
	Method      string       // HTTP method: "get", "post", or "wait"
	URL         string       // Request URL (supports $PORT substitution)
	Status      int          // Expected HTTP status code
	Contains    string       // Expected body substring (optional, empty = skip body check)
	BodyFile    string       // Path to file with expected body content (exact match)
	SendFile    string       // Path to file whose content is sent as the POST request body
	ContentType string       // Content-Type header for sendfile bodies (default application/json)
	Headers     []httpHeader // Request headers from repeatable header= keys, applied after ContentType
	InsecureTLS bool         // Accept self-signed TLS certificates for HTTPS test endpoints
	Timeout     string       // Poll timeout for wait checks (default "15s")
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
	Key         bool // Decode a database/ key rather than read an ordinary file.
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

// SetState records the test's state. Safe for concurrent use with progress.
func (r *Record) SetState(state State) {
	r.progressMu.Lock()
	defer r.progressMu.Unlock()
	r.State = state
}

// Begin records that the test entered state now, setting State and StartTime
// together so Display.Status never sees a running test without its start
// time. Safe for concurrent use with progress.
func (r *Record) Begin(state State) {
	r.progressMu.Lock()
	defer r.progressMu.Unlock()
	r.State = state
	r.StartTime = time.Now()
}

// progress returns State and StartTime as one consistent pair, for a reader on
// another goroutine than the test's. Safe for concurrent use.
func (r *Record) progress() (State, time.Time) {
	r.progressMu.Lock()
	defer r.progressMu.Unlock()
	return r.State, r.StartTime
}

// Activate marks the test for execution.
func (r *Record) Activate() {
	r.Active = true
}

// Deactivate marks the test as not selected for execution.
func (r *Record) Deactivate() {
	r.Active = false
	r.SetState(StateNone)
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
