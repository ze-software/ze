package runner

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/slogutil"
	"codeberg.org/thomas-mangin/ze/internal/test/ci"
	"codeberg.org/thomas-mangin/ze/internal/tmpfs"
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
}

// RunCommand represents a process to run during test execution.
type RunCommand struct {
	Mode    string // "background" or "foreground"
	Seq     int    // Execution order (lower first)
	Exec    string // Command to execute
	Stdin   string // Name of stdin block to pipe
	Timeout string // Timeout for foreground processes (e.g., "10s")
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
	// Fallback to numeric for large test suites
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
// Nick "A" or "1" → 0, "B" or "2" → 100, "C" or "3" → 200, etc.
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
		if len(prefix) == 0 {
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

// Tests is a container for test records.
type Tests struct {
	byNick  map[string]*Record
	ordered []string
	mu      sync.RWMutex
}

// NewTests creates a new test container.
func NewTests() *Tests {
	return &Tests{
		byNick:  make(map[string]*Record),
		ordered: nil,
	}
}

// Add creates and registers a new test record.
func (ts *Tests) Add(name string) *Record {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	r := NewRecord(name)
	ts.byNick[r.Nick] = r
	ts.ordered = append(ts.ordered, r.Nick)
	return r
}

// GetByNick returns the test with the given nick.
func (ts *Tests) GetByNick(nick string) *Record {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	return ts.byNick[nick]
}

// EnableByNick activates a test by nick.
func (ts *Tests) EnableByNick(nick string) bool {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if r, ok := ts.byNick[nick]; ok {
		r.Activate()
		return true
	}
	return false
}

// EnableAll activates all tests.
func (ts *Tests) EnableAll() {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	for _, r := range ts.byNick {
		r.Activate()
	}
}

// DisableAll deactivates all tests.
func (ts *Tests) DisableAll() {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	for _, r := range ts.byNick {
		r.Deactivate()
	}
}

// Registered returns all tests in order.
func (ts *Tests) Registered() []*Record {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	result := make([]*Record, 0, len(ts.ordered))
	for _, nick := range ts.ordered {
		result = append(result, ts.byNick[nick])
	}
	return result
}

// Selected returns active tests.
func (ts *Tests) Selected() []*Record {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	var result []*Record
	for _, nick := range ts.ordered {
		r := ts.byNick[nick]
		if r.IsActive() {
			result = append(result, r)
		}
	}
	return result
}

// Count returns the number of tests.
func (ts *Tests) Count() int {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	return len(ts.ordered)
}

// Summary returns counts by state.
func (ts *Tests) Summary() (passed, failed, timedOut, skipped int) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	for _, r := range ts.byNick {
		switch r.State { //nolint:exhaustive // only count terminal states
		case StateSuccess:
			passed++
		case StateFail:
			failed++
		case StateTimeout:
			timedOut++
		case StateSkip:
			skipped++
		}
	}
	return
}

// FailedRecords returns failed test records.
func (ts *Tests) FailedRecords() []*Record {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	var result []*Record
	for _, nick := range ts.ordered {
		r := ts.byNick[nick]
		if r.State == StateFail || r.State == StateTimeout {
			result = append(result, r)
		}
	}
	return result
}

// FailedNicks returns nicks of failed tests.
func (ts *Tests) FailedNicks() []string {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	var result []string
	for _, nick := range ts.ordered {
		r := ts.byNick[nick]
		if r.State == StateFail || r.State == StateTimeout {
			result = append(result, nick)
		}
	}
	return result
}

// Sort orders tests by name.
func (ts *Tests) Sort() {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	sort.Slice(ts.ordered, func(i, j int) bool {
		return ts.byNick[ts.ordered[i]].Name < ts.byNick[ts.ordered[j]].Name
	})
}

// EncodingTests manages encoding test discovery.
type EncodingTests struct {
	*Tests
	baseDir string
	port    int
}

// NewEncodingTests creates an encoding test manager.
func NewEncodingTests(baseDir string) *EncodingTests {
	return &EncodingTests{
		Tests:   NewTests(),
		baseDir: baseDir,
		port:    1790,
	}
}

// SetBasePort sets the starting port.
func (et *EncodingTests) SetBasePort(port int) {
	et.port = port
}

// Discover finds all .ci files in the directory.
func (et *EncodingTests) Discover(dir string) error {
	pattern := filepath.Join(dir, "*.ci")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}

	sort.Strings(files)

	for _, ciFile := range files {
		if err := et.parseAndAdd(ciFile); err != nil {
			return fmt.Errorf("%s: %w", filepath.Base(ciFile), err)
		}
	}

	return nil
}

// parseAndAdd parses a .ci file and adds it as a test.
// Uses new key=value format: action=type:key=value:key=value:...
// Supports Tmpfs blocks for embedded files.
func (et *EncodingTests) parseAndAdd(ciFile string) error {
	// First, try Tmpfs parsing to extract embedded files
	v, err := tmpfs.ReadFrom(ciFile)
	if err != nil {
		return fmt.Errorf("parse %s: %w", ciFile, err)
	}

	name := strings.TrimSuffix(filepath.Base(ciFile), ".ci")
	r := et.Add(name)
	r.Port = et.port
	et.port++
	r.CIFile = ciFile
	r.Files = append(r.Files, ciFile)

	// Store Tmpfs files if any
	if len(v.Files) > 0 {
		r.TmpfsFiles = make(map[string][]byte)
		for _, f := range v.Files {
			r.TmpfsFiles[f.Path] = f.Content
		}
	}

	// Store stdin blocks if any
	if len(v.StdinBlocks) > 0 {
		r.StdinBlocks = v.StdinBlocks
		for name, content := range v.StdinBlocks {
			recordLogger().Debug("stdin block loaded", "name", name, "size", len(content), "preview", string(content[:min(100, len(content))]))
		}

		// Also parse "peer" stdin block for expectations (for reporting purposes).
		// The peer block content is passed to ze-peer which parses it, but the
		// test runner also needs to know about expectations for progress/failure reporting.
		if peerBlock, ok := v.StdinBlocks["peer"]; ok {
			lines := strings.Split(string(peerBlock), "\n")
			for _, line := range lines {
				trimmed := strings.TrimSpace(line)
				if trimmed == "" || strings.HasPrefix(trimmed, "#") {
					continue
				}
				// Parse expect= and action= lines for reporting purposes
				if strings.HasPrefix(trimmed, "expect=") || strings.HasPrefix(trimmed, "action=") {
					if err := et.parseLine(r, ciFile, trimmed); err != nil {
						// Log but don't fail - these are primarily for ze-peer
						recordLogger().Debug("parsing peer block line", "line", trimmed, "error", err)
					}
				}
			}
		}
	}

	// Parse the non-Tmpfs lines (option:, expect:, cmd:, run=, etc.)
	for lineNum, line := range v.OtherLines {
		if err := et.parseLine(r, ciFile, line); err != nil {
			return fmt.Errorf("line %d: %w", lineNum+1, err)
		}
	}

	// Verify config exists (for non-Tmpfs configs)
	if configPath, ok := r.Conf["config"].(string); ok {
		// Check if it's a Tmpfs file first
		if r.TmpfsFiles != nil {
			if _, isTmpfs := r.TmpfsFiles[filepath.Base(configPath)]; isTmpfs {
				// Config is in Tmpfs, will be written to temp dir at runtime
				goto generateDecoded
			}
		}
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			return fmt.Errorf("config not found: %s", configPath)
		}
	}

generateDecoded:
	// Generate decoded strings for messages with Raw
	for i := range r.Messages {
		if len(r.Messages[i].Raw) > 0 {
			if decoded, err := DecodeMessageBytes(r.Messages[i].Raw); err == nil {
				r.Messages[i].Decoded = decoded.String()
			}
		}
	}

	return nil
}

// parseLine parses a single .ci line in the action=type:key=value format.
func (et *EncodingTests) parseLine(r *Record, ciFile, line string) error {
	// Parse action=type:key=value:key=value:...
	// First segment is action=type, remaining segments are key=value pairs
	parts := strings.Split(line, ":")
	if len(parts) < 1 {
		return fmt.Errorf("invalid format %q, expected action=type:key=value", line)
	}

	// First segment is action=type
	actionType := strings.SplitN(parts[0], "=", 2)
	if len(actionType) != 2 {
		return fmt.Errorf("invalid format %q, expected action=type:key=value", line)
	}
	action := actionType[0]
	lineType := actionType[1]
	kvPairs := ci.ParseKVPairs(parts[1:])

	switch action {
	case "option":
		return et.parseOption(r, ciFile, lineType, kvPairs)
	case "expect":
		return et.parseExpect(r, lineType, kvPairs)
	case "reject":
		return et.parseReject(r, lineType, kvPairs)
	case "action":
		return et.parseAction(r, lineType, kvPairs)
	case "cmd":
		return et.parseCmd(r, lineType, kvPairs)
	default:
		return fmt.Errorf("unknown action %q in %q", action, line)
	}
}

// parseOption handles option=type:key=value lines.
func (et *EncodingTests) parseOption(r *Record, ciFile, optType string, kv map[string]string) error {
	switch optType {
	case "file":
		configName := kv["path"]
		if configName == "" {
			return fmt.Errorf("option:file missing path=")
		}
		configPath := filepath.Join(filepath.Dir(ciFile), configName)
		absConfig, err := filepath.Abs(configPath)
		if err != nil {
			return fmt.Errorf("invalid config path: %w", err)
		}
		absTestDir, err := filepath.Abs(filepath.Dir(ciFile))
		if err != nil {
			return fmt.Errorf("invalid test dir: %w", err)
		}
		if !strings.HasPrefix(absConfig, absTestDir+string(filepath.Separator)) && absConfig != absTestDir {
			return fmt.Errorf("config file outside test directory: %s", configName)
		}
		r.Conf["config"] = configPath
		r.ConfigFile = configPath
		r.Files = append(r.Files, configPath)

	case "asn":
		value := kv["value"]
		if value == "" {
			return fmt.Errorf("option:asn missing value=")
		}
		r.Extra["asn"] = value
		r.Options = append(r.Options, fmt.Sprintf("option=asn:value=%s", value))

	case "bind":
		value := kv["value"]
		if value == "" {
			return fmt.Errorf("option:bind missing value=")
		}
		r.Extra["bind"] = value
		r.Options = append(r.Options, fmt.Sprintf("option=bind:value=%s", value))

	case "timeout":
		value := kv["value"]
		if value == "" {
			return fmt.Errorf("option:timeout missing value=")
		}
		r.Extra["timeout"] = value

	case "tcp_connections":
		value := kv["value"]
		if value == "" {
			return fmt.Errorf("option:tcp_connections missing value=")
		}
		r.Options = append(r.Options, fmt.Sprintf("option=tcp_connections:value=%s", value))

	case "open":
		value := kv["value"]
		if value == "" {
			return fmt.Errorf("option:open missing value=")
		}
		r.Options = append(r.Options, fmt.Sprintf("option=open:value=%s", value))

	case "update":
		value := kv["value"]
		if value == "" {
			return fmt.Errorf("option:update missing value=")
		}
		r.Options = append(r.Options, fmt.Sprintf("option=update:value=%s", value))

	case "env":
		varName := kv["var"]
		value := kv["value"]
		if varName == "" {
			return fmt.Errorf("option:env missing var=")
		}
		// Store as KEY=VALUE for environment setting
		r.EnvVars = append(r.EnvVars, fmt.Sprintf("%s=%s", varName, value))

	default:
		return fmt.Errorf("unknown option type %q", optType)
	}
	return nil
}

// parseExpect handles expect:type:... lines.
func (et *EncodingTests) parseExpect(r *Record, expType string, kv map[string]string) error {
	switch expType {
	case "bgp":
		conn, seq, err := parseConnSeq(kv)
		if err != nil {
			return fmt.Errorf("expect:bgp: %w", err)
		}
		hexData := kv["hex"]
		if hexData == "" {
			return fmt.Errorf("expect:bgp missing hex=")
		}
		idx := connSeqToIndex(conn, seq)
		msg := r.getOrCreateMessage(idx)
		msg.RawHex = strings.ReplaceAll(hexData, ":", "")
		if rawBytes, err := hex.DecodeString(msg.RawHex); err == nil {
			msg.Raw = rawBytes
		}
		// Add to Expects for testpeer (new format).
		r.Expects = append(r.Expects, fmt.Sprintf("expect=bgp:conn=%d:seq=%d:hex=%s", conn, seq, hexData))

	case "json":
		conn, seq, err := parseConnSeq(kv)
		if err != nil {
			return fmt.Errorf("expect:json: %w", err)
		}
		jsonData := kv["json"]
		if jsonData == "" {
			return fmt.Errorf("expect:json missing json=")
		}
		idx := connSeqToIndex(conn, seq)
		msg := r.getOrCreateMessage(idx)
		msg.JSON = jsonData

	case "exit":
		codeStr := kv["code"]
		if codeStr == "" {
			return fmt.Errorf("expect:exit missing code=")
		}
		code, err := strconv.Atoi(codeStr)
		if err != nil {
			return fmt.Errorf("expect:exit invalid code=%q: %w", codeStr, err)
		}
		r.ExpectExitCode = &code

	case "stderr":
		// Support both pattern= (regex) and contains= (substring)
		if pattern, ok := kv["pattern"]; ok {
			r.ExpectStderr = append(r.ExpectStderr, pattern)
		}
		if contains := kv["contains"]; contains != "" {
			r.ExpectStderrMatch = contains
		}

	case "stdout":
		// Support contains= (substring match)
		if contains := kv["contains"]; contains != "" {
			r.ExpectStdoutMatch = append(r.ExpectStdoutMatch, contains)
		}

	case "syslog":
		pattern := kv["pattern"]
		r.ExpectSyslog = append(r.ExpectSyslog, pattern)

	default:
		return fmt.Errorf("unknown expect type %q", expType)
	}
	return nil
}

// parseReject handles reject:type:... lines.
func (et *EncodingTests) parseReject(r *Record, rejType string, kv map[string]string) error {
	switch rejType {
	case "stderr":
		pattern := kv["pattern"]
		r.RejectStderr = append(r.RejectStderr, pattern)

	case "syslog":
		pattern := kv["pattern"]
		r.RejectSyslog = append(r.RejectSyslog, pattern)

	default:
		return fmt.Errorf("unknown reject type %q", rejType)
	}
	return nil
}

// parseAction handles action:type:... lines.
func (et *EncodingTests) parseAction(r *Record, actType string, kv map[string]string) error {
	switch actType {
	case "notification":
		conn, seq, err := parseConnSeq(kv)
		if err != nil {
			return fmt.Errorf("action:notification: %w", err)
		}
		text := kv["text"]
		// Add to Expects for testpeer (new format).
		r.Expects = append(r.Expects, fmt.Sprintf("action=notification:conn=%d:seq=%d:text=%s", conn, seq, text))

	case "send":
		conn, seq, err := parseConnSeq(kv)
		if err != nil {
			return fmt.Errorf("action:send: %w", err)
		}
		hexData := kv["hex"]
		if hexData == "" {
			return fmt.Errorf("action:send missing hex=")
		}
		// Add to Expects for testpeer (new format).
		r.Expects = append(r.Expects, fmt.Sprintf("action=send:conn=%d:seq=%d:hex=%s", conn, seq, hexData))

	default:
		return fmt.Errorf("unknown action type %q", actType)
	}
	return nil
}

// parseCmd handles cmd:type:... lines.
func (et *EncodingTests) parseCmd(r *Record, cmdType string, kv map[string]string) error {
	switch cmdType {
	case "api":
		conn, seq, err := parseConnSeq(kv)
		if err != nil {
			return fmt.Errorf("cmd:api: %w", err)
		}
		text := kv["text"]
		idx := connSeqToIndex(conn, seq)
		msg := r.getOrCreateMessage(idx)
		msg.Cmd = text

	case "background", "foreground":
		seqStr := kv["seq"]
		if seqStr == "" {
			return fmt.Errorf("cmd:%s missing seq=", cmdType)
		}
		seq, err := strconv.Atoi(seqStr)
		if err != nil || seq < 1 {
			return fmt.Errorf("cmd:%s invalid seq=%q", cmdType, seqStr)
		}

		exec := kv["exec"]
		if exec == "" {
			return fmt.Errorf("cmd:%s missing exec=", cmdType)
		}

		rc := RunCommand{
			Mode:    cmdType,
			Seq:     seq,
			Exec:    exec,
			Stdin:   kv["stdin"],
			Timeout: kv["timeout"],
		}
		r.RunCommands = append(r.RunCommands, rc)

	default:
		return fmt.Errorf("unknown cmd type %q", cmdType)
	}
	return nil
}

// parseConnSeq extracts conn and seq from key-value pairs.
// Validates: conn must be 1-4, seq must be >= 1.
func parseConnSeq(kv map[string]string) (conn, seq int, err error) {
	connStr := kv["conn"]
	seqStr := kv["seq"]

	if connStr == "" {
		return 0, 0, fmt.Errorf("missing conn=")
	}
	if seqStr == "" {
		return 0, 0, fmt.Errorf("missing seq=")
	}

	conn, err = strconv.Atoi(connStr)
	if err != nil || conn < 1 || conn > 4 {
		return 0, 0, fmt.Errorf("invalid conn=%q (must be 1-4)", connStr)
	}
	seq, err = strconv.Atoi(seqStr)
	if err != nil || seq < 1 {
		return 0, 0, fmt.Errorf("invalid seq=%q (must be >= 1)", seqStr)
	}

	return conn, seq, nil
}

// connSeqToIndex converts conn+seq to a unique message index.
// conn=1:seq=1 → 101, conn=1:seq=2 → 102, conn=2:seq=1 → 201, etc.
func connSeqToIndex(conn, seq int) int {
	return conn*100 + seq
}

// List prints available tests.
func (ts *Tests) List() {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	fmt.Println("\nAvailable tests:")
	fmt.Println()

	for _, nick := range ts.ordered {
		r := ts.byNick[nick]
		fmt.Printf("  %s  %s\n", r.Nick, r.Name)
	}
	fmt.Println()
}
