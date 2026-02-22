// Design: docs/architecture/testing/ci-format.md — message validation against expectations
// Related: peer.go — test peer that drives the checker
// Related: expect.go — .ci file loading that produces checker inputs
// Related: message.go — Message type checked against expectations

package peer

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"codeberg.org/thomas-mangin/ze/internal/test/ci"
)

// Action type identifiers used in .ci test files.
const (
	actionSighup  = "sighup"
	actionSigterm = "sigterm"
)

// Checker validates received messages against expected patterns.
type Checker struct {
	messages            []string
	sequences           [][]string
	connectionIDs       []int  // Connection number (1-4) for each sequence
	currentConnection   int    // Current connection number (0 = none)
	lastExpected        string // For diff output on mismatch
	lastReceived        string // For diff output on mismatch
	connectionJustEnded bool   // True if last match ended a connection (not just sequence)
	expectClose         bool   // True after sighup action — next EOF is expected (daemon restarts peer)
	mu                  sync.Mutex
}

// NewChecker creates a new checker from expected messages.
// Returns error if any expected rule is invalid.
func NewChecker(expected []string) (*Checker, error) {
	c := &Checker{}
	sequences, connIDs, err := c.groupMessages(expected)
	if err != nil {
		return nil, err
	}
	c.sequences = sequences
	c.connectionIDs = connIDs
	return c, nil
}

func (c *Checker) groupMessages(expected []string) ([][]string, []int, error) {
	groups := make(map[int]map[int][]string) // conn -> seq -> messages

	for _, rule := range expected {
		conn, seq, content, err := parseExpectRule(rule)
		if err != nil {
			return nil, nil, err
		}

		if groups[conn] == nil {
			groups[conn] = make(map[int][]string)
		}
		groups[conn][seq] = append(groups[conn][seq], content)
	}

	var result [][]string
	var connIDs []int
	for conn := 1; conn <= 4; conn++ {
		if groups[conn] == nil {
			continue
		}
		for seq := 1; seq <= 100; seq++ {
			if msgs := groups[conn][seq]; len(msgs) > 0 {
				result = append(result, msgs)
				connIDs = append(connIDs, conn)
			}
		}
	}

	return result, connIDs, nil
}

// parseExpectRule parses new format expect rules.
// Returns conn (1-4), seq, and normalized content.
// Only handles: expect=bgp:conn=N:seq=N:hex=... and action=notification:conn=N:seq=N:text=...
// Returns error for invalid or incomplete rules.
func parseExpectRule(rule string) (conn, seq int, content string, err error) {
	// expect=bgp:conn=N:seq=N:hex=...
	if after, ok := strings.CutPrefix(rule, "expect=bgp:"); ok {
		kv := parseKV(after)

		connStr := kv["conn"]
		if connStr == "" {
			return 0, 0, "", fmt.Errorf("expect=bgp missing conn: %q", rule)
		}
		conn, err = strconv.Atoi(connStr)
		if err != nil || conn < 1 || conn > 4 {
			return 0, 0, "", fmt.Errorf("expect=bgp invalid conn=%q (must be 1-4): %q", connStr, rule)
		}

		seqStr := kv["seq"]
		if seqStr == "" {
			return 0, 0, "", fmt.Errorf("expect=bgp missing seq: %q", rule)
		}
		seq, err = strconv.Atoi(seqStr)
		if err != nil || seq < 1 {
			return 0, 0, "", fmt.Errorf("expect=bgp invalid seq=%q (must be >= 1): %q", seqStr, rule)
		}

		hex := kv["hex"]
		if hex == "" {
			return 0, 0, "", fmt.Errorf("expect=bgp missing hex: %q", rule)
		}
		content = strings.ToUpper(strings.ReplaceAll(hex, ":", ""))
		return conn, seq, content, nil
	}

	// action=notification:conn=N:seq=N:text=...
	if after, ok := strings.CutPrefix(rule, "action=notification:"); ok {
		kv := parseKV(after)

		connStr := kv["conn"]
		if connStr == "" {
			return 0, 0, "", fmt.Errorf("action:notification missing conn: %q", rule)
		}
		conn, err = strconv.Atoi(connStr)
		if err != nil || conn < 1 || conn > 4 {
			return 0, 0, "", fmt.Errorf("action:notification invalid conn=%q (must be 1-4): %q", connStr, rule)
		}

		seqStr := kv["seq"]
		if seqStr == "" {
			return 0, 0, "", fmt.Errorf("action:notification missing seq: %q", rule)
		}
		seq, err = strconv.Atoi(seqStr)
		if err != nil || seq < 1 {
			return 0, 0, "", fmt.Errorf("action:notification invalid seq=%q (must be >= 1): %q", seqStr, rule)
		}

		text := kv["text"]
		if text == "" {
			return 0, 0, "", fmt.Errorf("action:notification missing text: %q", rule)
		}
		content = "notification:" + text
		return conn, seq, content, nil
	}

	// action=send:conn=N:seq=N:hex=...
	if after, ok := strings.CutPrefix(rule, "action=send:"); ok {
		kv := parseKV(after)

		connStr := kv["conn"]
		if connStr == "" {
			return 0, 0, "", fmt.Errorf("action:send missing conn: %q", rule)
		}
		conn, err = strconv.Atoi(connStr)
		if err != nil || conn < 1 || conn > 4 {
			return 0, 0, "", fmt.Errorf("action:send invalid conn=%q (must be 1-4): %q", connStr, rule)
		}

		seqStr := kv["seq"]
		if seqStr == "" {
			return 0, 0, "", fmt.Errorf("action:send missing seq: %q", rule)
		}
		seq, err = strconv.Atoi(seqStr)
		if err != nil || seq < 1 {
			return 0, 0, "", fmt.Errorf("action:send invalid seq=%q (must be >= 1): %q", seqStr, rule)
		}

		hex := kv["hex"]
		if hex == "" {
			return 0, 0, "", fmt.Errorf("action:send missing hex: %q", rule)
		}
		content = "send:" + strings.ToUpper(strings.ReplaceAll(hex, ":", ""))
		return conn, seq, content, nil
	}

	// action=rewrite:conn=N:seq=N:source=FILE:dest=FILE
	if after, ok := strings.CutPrefix(rule, "action=rewrite:"); ok {
		kv := parseKV(after)

		connStr := kv["conn"]
		if connStr == "" {
			return 0, 0, "", fmt.Errorf("action:rewrite missing conn: %q", rule)
		}
		conn, err = strconv.Atoi(connStr)
		if err != nil || conn < 1 || conn > 4 {
			return 0, 0, "", fmt.Errorf("action:rewrite invalid conn=%q (must be 1-4): %q", connStr, rule)
		}

		seqStr := kv["seq"]
		if seqStr == "" {
			return 0, 0, "", fmt.Errorf("action:rewrite missing seq: %q", rule)
		}
		seq, err = strconv.Atoi(seqStr)
		if err != nil || seq < 1 {
			return 0, 0, "", fmt.Errorf("action:rewrite invalid seq=%q (must be >= 1): %q", seqStr, rule)
		}

		source := kv["source"]
		if source == "" {
			return 0, 0, "", fmt.Errorf("action:rewrite missing source: %q", rule)
		}
		dest := kv["dest"]
		if dest == "" {
			return 0, 0, "", fmt.Errorf("action:rewrite missing dest: %q", rule)
		}
		content = "rewrite:" + source + ":" + dest
		return conn, seq, content, nil
	}

	// action=sighup:conn=N:seq=N
	if after, ok := strings.CutPrefix(rule, "action=sighup:"); ok {
		kv := parseKV(after)

		connStr := kv["conn"]
		if connStr == "" {
			return 0, 0, "", fmt.Errorf("action:sighup missing conn: %q", rule)
		}
		conn, err = strconv.Atoi(connStr)
		if err != nil || conn < 1 || conn > 4 {
			return 0, 0, "", fmt.Errorf("action:sighup invalid conn=%q (must be 1-4): %q", connStr, rule)
		}

		seqStr := kv["seq"]
		if seqStr == "" {
			return 0, 0, "", fmt.Errorf("action:sighup missing seq: %q", rule)
		}
		seq, err = strconv.Atoi(seqStr)
		if err != nil || seq < 1 {
			return 0, 0, "", fmt.Errorf("action:sighup invalid seq=%q (must be >= 1): %q", seqStr, rule)
		}

		content = actionSighup
		return conn, seq, content, nil
	}

	// action=sigterm:conn=N:seq=N
	if after, ok := strings.CutPrefix(rule, "action=sigterm:"); ok {
		kv := parseKV(after)

		connStr := kv["conn"]
		if connStr == "" {
			return 0, 0, "", fmt.Errorf("action:sigterm missing conn: %q", rule)
		}
		conn, err = strconv.Atoi(connStr)
		if err != nil || conn < 1 || conn > 4 {
			return 0, 0, "", fmt.Errorf("action:sigterm invalid conn=%q (must be 1-4): %q", connStr, rule)
		}

		seqStr := kv["seq"]
		if seqStr == "" {
			return 0, 0, "", fmt.Errorf("action:sigterm missing seq: %q", rule)
		}
		seq, err = strconv.Atoi(seqStr)
		if err != nil || seq < 1 {
			return 0, 0, "", fmt.Errorf("action:sigterm invalid seq=%q (must be >= 1): %q", seqStr, rule)
		}

		content = actionSigterm
		return conn, seq, content, nil
	}

	return 0, 0, "", fmt.Errorf("unknown expect format: %q", rule)
}

// parseKV parses key=value pairs from a colon-separated string.
// Handles values that may contain colons (like hex=...).
func parseKV(s string) map[string]string {
	return ci.ParseKVPairs(strings.Split(s, ":"))
}

// Init initializes the checker for a new session.
func (c *Checker) Init() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Always clear connectionJustEnded at start of new connection.
	// This flag may have been set when loading the next sequence in updateMessagesIfRequired(),
	// but the actual connection transition happens here.
	c.connectionJustEnded = false

	if len(c.messages) > 0 {
		return false
	}
	if len(c.sequences) == 0 {
		return false
	}

	c.currentConnection = c.connectionIDs[0]
	c.messages = c.sequences[0]
	c.sequences = c.sequences[1:]
	c.connectionIDs = c.connectionIDs[1:]
	return true
}

// Expected checks if the received message matches expectations.
func (c *Checker) Expected(msg *Message) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	// If no expectations, accept KEEPALIVE or EOR.
	if len(c.sequences) == 0 && len(c.messages) == 0 {
		return msg.IsKeepalive() || msg.IsEOR()
	}

	stream := msg.Stream()

	for i, check := range c.messages {
		received := stream
		if !strings.HasPrefix(check, strings.Repeat("F", 32)) && !strings.Contains(check, ":") {
			received = received[32:]
		}

		if strings.EqualFold(check, received) {
			c.messages = append(c.messages[:i], c.messages[i+1:]...)
			c.updateMessagesIfRequired()
			return true
		}
	}

	// No match - accept KEEPALIVE anyway (normal BGP operation).
	if msg.IsKeepalive() {
		return true
	}

	// Store mismatch details for diff output.
	c.lastReceived = stream
	if len(c.messages) > 0 {
		c.lastExpected = c.messages[0]
	}

	return false
}

// ExpectedOrKeepalive checks if message matches expectations.
// Returns (matched, silentAccept):
//   - (true, false): message matched and was consumed
//   - (false, true): KEEPALIVE not in expectations, silently accepted
//   - (false, false): message doesn't match, should fail
func (c *Checker) ExpectedOrKeepalive(msg *Message) (matched, silentAccept bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// If no expectations, accept KEEPALIVE or EOR silently.
	if len(c.sequences) == 0 && len(c.messages) == 0 {
		if msg.IsKeepalive() || msg.IsEOR() {
			return false, true
		}
		return false, false
	}

	stream := msg.Stream()

	for i, check := range c.messages {
		received := stream
		if !strings.HasPrefix(check, strings.Repeat("F", 32)) && !strings.Contains(check, ":") {
			received = received[32:]
		}

		if strings.EqualFold(check, received) {
			c.messages = append(c.messages[:i], c.messages[i+1:]...)
			c.updateMessagesIfRequired()
			return true, false
		}
	}

	// No match - if KEEPALIVE, silently accept
	if msg.IsKeepalive() {
		return false, true
	}

	// Store mismatch details for diff output.
	c.lastReceived = stream
	if len(c.messages) > 0 {
		c.lastExpected = c.messages[0]
	}

	return false, false
}

// LastMismatch returns the expected and received values from the last mismatch.
func (c *Checker) LastMismatch() (expected, received string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastExpected, c.lastReceived
}

func (c *Checker) updateMessagesIfRequired() {
	if len(c.messages) == 0 && len(c.sequences) > 0 {
		// Check if the next sequence is from a different connection
		nextConn := c.connectionIDs[0]
		if c.currentConnection != 0 && nextConn != c.currentConnection {
			c.connectionJustEnded = true
		}
		c.currentConnection = nextConn
		c.messages = c.sequences[0]
		c.sequences = c.sequences[1:]
		c.connectionIDs = c.connectionIDs[1:]
	}
}

// SequenceEnded returns true if the last matched message ended a connection.
// This indicates the connection should close and a new connection is expected.
// Calling this method clears the flag.
func (c *Checker) SequenceEnded() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	ended := c.connectionJustEnded
	c.connectionJustEnded = false
	return ended
}

// Completed returns true if all expected messages have been received.
func (c *Checker) Completed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.messages) == 0 && len(c.sequences) == 0
}

// NextNotificationAction checks if the next expected item is a notification: action.
// If so, it returns (true, text) and removes the action from the queue.
// If not, it returns (false, "").
func (c *Checker) NextNotificationAction() (bool, string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.messages) == 0 {
		return false, ""
	}

	msg := c.messages[0]
	if !strings.HasPrefix(msg, "notification:") {
		return false, ""
	}

	// Extract the notification text (everything after "notification:")
	text := strings.TrimPrefix(msg, "notification:")
	c.messages = c.messages[1:]
	c.updateMessagesIfRequired()

	return true, text
}

// NextSendAction checks if the next expected item is a send: action.
// If so, it returns (true, hexData) and removes the action from the queue.
// If not, it returns (false, "").
func (c *Checker) NextSendAction() (bool, string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.messages) == 0 {
		return false, ""
	}

	msg := c.messages[0]
	if !strings.HasPrefix(msg, "send:") {
		return false, ""
	}

	// Extract the hex data (everything after "send:")
	hexData := strings.TrimPrefix(msg, "send:")
	c.messages = c.messages[1:]
	c.updateMessagesIfRequired()

	return true, hexData
}

// NextRewriteAction checks if the next expected item is a rewrite: action.
// If so, it returns (true, source, dest) and removes the action from the queue.
// If not, it returns (false, "", "").
func (c *Checker) NextRewriteAction() (bool, string, string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.messages) == 0 {
		return false, "", ""
	}

	msg := c.messages[0]
	if !strings.HasPrefix(msg, "rewrite:") {
		return false, "", ""
	}

	// Format: "rewrite:SOURCE:DEST"
	parts := strings.SplitN(strings.TrimPrefix(msg, "rewrite:"), ":", 2)
	if len(parts) != 2 {
		return false, "", ""
	}
	c.messages = c.messages[1:]
	c.updateMessagesIfRequired()

	return true, parts[0], parts[1]
}

// NextSighupAction checks if the next expected item is a sighup action.
// If so, it returns true, removes the action from the queue, and sets
// expectClose so the next EOF is treated as expected (daemon restarts peer).
func (c *Checker) NextSighupAction() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.messages) == 0 {
		return false
	}

	if c.messages[0] != actionSighup {
		return false
	}

	c.messages = c.messages[1:]
	c.expectClose = true
	c.updateMessagesIfRequired()

	return true
}

// NextSigtermAction checks if the next expected item is a sigterm action.
// If so, it returns true, removes the action from the queue, and sets
// expectClose so the next EOF is treated as expected (daemon shuts down).
func (c *Checker) NextSigtermAction() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.messages) == 0 {
		return false
	}

	if c.messages[0] != actionSigterm {
		return false
	}

	c.messages = c.messages[1:]
	c.expectClose = true
	c.updateMessagesIfRequired()

	return true
}

// ExpectingClose returns true if the connection is expected to close
// (e.g., after a SIGHUP triggered a daemon reload). Clears the flag.
func (c *Checker) ExpectingClose() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	v := c.expectClose
	c.expectClose = false
	return v
}
