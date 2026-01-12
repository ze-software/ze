package functional

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

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
}

// NewRecord creates a new test record.
func NewRecord(name string) *Record {
	return &Record{
		Name:   name,
		Nick:   generateNick(name),
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

func generateNick(_ string) string {
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
			continue
		}
	}

	return nil
}

// parseAndAdd parses a .ci file and adds it as a test.
func (et *EncodingTests) parseAndAdd(ciFile string) error {
	f, err := os.Open(ciFile) //nolint:gosec // Test files from known directory
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	name := strings.TrimSuffix(filepath.Base(ciFile), ".ci")
	r := et.Add(name)
	r.Port = et.port
	et.port++
	r.CIFile = ciFile
	r.Files = append(r.Files, ciFile)

	// Track next available index for each base index (handles multiple C1:raw: lines)
	nextAvailableIdx := make(map[int]int)

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		switch {
		case strings.HasPrefix(line, "option:file:"):
			configName := strings.TrimPrefix(line, "option:file:")
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

		case strings.HasPrefix(line, "option:asn:"):
			r.Extra["asn"] = strings.TrimPrefix(line, "option:asn:")
			r.Options = append(r.Options, line)

		case strings.HasPrefix(line, "option:bind:"):
			r.Extra["bind"] = strings.TrimPrefix(line, "option:bind:")
			r.Options = append(r.Options, line)

		case strings.HasPrefix(line, "option:timeout:"):
			r.Extra["timeout"] = strings.TrimPrefix(line, "option:timeout:")
			// Don't add to Options - this is for the runner, not testpeer

		case strings.HasPrefix(line, "option:"):
			r.Options = append(r.Options, line)

		case strings.Contains(line, ":cmd:"):
			// Parse: "1:cmd:update text..." (command documentation)
			parts := strings.SplitN(line, ":", 3)
			if len(parts) >= 3 {
				idx := parseMessageIndex(parts[0])
				msg := r.getOrCreateMessage(idx)
				msg.Cmd = parts[2]
			}

		case strings.Contains(line, ":raw:"):
			// Parse: "1:raw:FFFF..." or "C1:raw:FFFF..."
			// Multiple lines with same prefix (e.g., C1, C1, C1, C1) get sequential indices
			parts := strings.SplitN(line, ":", 3)
			if len(parts) >= 3 {
				baseIdx := parseMessageIndex(parts[0])
				// Get or initialize the next available index for this base
				if _, ok := nextAvailableIdx[baseIdx]; !ok {
					nextAvailableIdx[baseIdx] = baseIdx
				}
				idx := nextAvailableIdx[baseIdx]
				nextAvailableIdx[baseIdx]++

				msg := r.getOrCreateMessage(idx)
				msg.RawHex = strings.ReplaceAll(parts[2], ":", "")
				if rawBytes, err := hex.DecodeString(msg.RawHex); err == nil {
					msg.Raw = rawBytes
				}
			}
			r.Expects = append(r.Expects, line)

		case strings.Contains(line, ":json:"):
			// Parse: "1:json:{...}"
			parts := strings.SplitN(line, ":", 3)
			if len(parts) >= 3 {
				idx := parseMessageIndex(parts[0])
				msg := r.getOrCreateMessage(idx)
				msg.JSON = parts[2]
			}

		case strings.Contains(line, ":notification:"):
			r.Expects = append(r.Expects, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	// Verify config exists
	if configPath, ok := r.Conf["config"].(string); ok {
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			return fmt.Errorf("config not found: %s", configPath)
		}
	}

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

// parseMessageIndex extracts the message index from a prefix like "1", "A1", "B2".
// Connection letters are encoded as offsets: A=0, B=100, C=200, D=300.
// So A1→1, B1→101, C1→201, ensuring unique indices per connection.
func parseMessageIndex(prefix string) int {
	connOffset := 0
	// Extract connection letter offset
	if len(prefix) > 0 {
		first := prefix[0]
		if first >= 'A' && first <= 'Z' {
			connOffset = int(first-'A') * 100
			prefix = prefix[1:]
		} else if first >= 'a' && first <= 'z' {
			connOffset = int(first-'a') * 100
			prefix = prefix[1:]
		}
	}
	idx, _ := strconv.Atoi(prefix)
	if idx == 0 {
		idx = 1
	}
	return connOffset + idx
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
