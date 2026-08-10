// Design: docs/architecture/testing/ci-format.md — message validation against expectations
// Overview: peer.go — test peer that drives the checker
// Related: expect.go — .ci file loading that produces checker inputs
// Related: message.go — Message type checked against expectations

package peer

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/test/ci"
)

// Action type identifiers used in .ci test files.
const (
	actionClose   = "close"
	actionSighup  = "sighup"
	actionSigterm = "sigterm"
)

// Checker validates received messages against expected patterns.
type Checker struct {
	messages            []string
	sequences           [][]string
	connectionIDs       []int  // Connection number for each sequence
	currentConnection   int    // Current connection number (0 = none)
	lastExpected        string // For diff output on mismatch
	lastReceived        string // For diff output on mismatch
	connectionJustEnded bool   // True if last match ended a connection (not just sequence)
	expectClose         bool   // True after sighup action — next EOF is expected (daemon restarts peer)
	// misorder holds one note per marker accepted in silence that ALSO matched an
	// expectation the fixture still owed (ExpectedOrKeepalive). misorderPending is
	// the note the caller has not read yet, so the peer can print it where it
	// happened rather than only where a later frame fails.
	misorder        []string
	misorderPending string
	mu              sync.Mutex
}

// newChecker creates a new checker from expected messages.
// Returns error if any expected rule is invalid.
func newChecker(expected []string) (*Checker, error) {
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
	connKeys := make([]int, 0, len(groups))
	for conn := range groups {
		connKeys = append(connKeys, conn)
	}
	slices.Sort(connKeys)
	for _, conn := range connKeys {
		seqs := groups[conn]
		seqKeys := make([]int, 0, len(seqs))
		for seq := range seqs {
			seqKeys = append(seqKeys, seq)
		}
		slices.Sort(seqKeys)
		for _, seq := range seqKeys {
			result = append(result, seqs[seq])
			connIDs = append(connIDs, conn)
		}
	}

	return result, connIDs, nil
}

// parseExpectRule parses new format expect rules.
// Returns conn, seq, and normalized content.
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
		if err != nil || conn < 1 {
			return 0, 0, "", fmt.Errorf("expect=bgp invalid conn=%q (must be >= 1): %q", connStr, rule)
		}

		seqStr := kv["seq"]
		if seqStr == "" {
			return 0, 0, "", fmt.Errorf("expect=bgp missing seq: %q", rule)
		}
		seq, err = strconv.Atoi(seqStr)
		if err != nil || seq < 1 {
			return 0, 0, "", fmt.Errorf("expect=bgp invalid seq=%q (must be >= 1): %q", seqStr, rule)
		}

		if hexVal := kv["hex"]; hexVal != "" {
			content = strings.ToUpper(strings.ReplaceAll(hexVal, ":", ""))
			return conn, seq, content, nil
		}
		if prefixVal := kv["prefix"]; prefixVal != "" {
			var tb textbuf.Buffer
			content = tb.Str("prefix:").Str(strings.ToUpper(strings.ReplaceAll(prefixVal, ":", ""))).String()
			return conn, seq, content, nil
		}
		if containsVal := kv["contains"]; containsVal != "" {
			var tb textbuf.Buffer
			content = tb.Str("contains:").Str(strings.ToUpper(strings.ReplaceAll(containsVal, ":", ""))).String()
			return conn, seq, content, nil
		}
		if orderedVal := kv["ordered"]; orderedVal != "" {
			var tb textbuf.Buffer
			content = tb.Str("ordered:").Str(strings.ToUpper(strings.ReplaceAll(orderedVal, ":", ""))).String()
			return conn, seq, content, nil
		}
		return 0, 0, "", fmt.Errorf("expect=bgp missing hex, prefix, contains, or ordered: %q", rule)
	}

	// action=notification:conn=N:seq=N:text=...
	if after, ok := strings.CutPrefix(rule, "action=notification:"); ok {
		kv := parseKV(after)

		connStr := kv["conn"]
		if connStr == "" {
			return 0, 0, "", fmt.Errorf("action:notification missing conn: %q", rule)
		}
		conn, err = strconv.Atoi(connStr)
		if err != nil || conn < 1 {
			return 0, 0, "", fmt.Errorf("action:notification invalid conn=%q (must be >= 1): %q", connStr, rule)
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
		var tb textbuf.Buffer
		content = tb.Str("notification:").Str(text).String()
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
		if err != nil || conn < 1 {
			return 0, 0, "", fmt.Errorf("action:send invalid conn=%q (must be >= 1): %q", connStr, rule)
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
		var tb textbuf.Buffer
		content = tb.Str("send:").Str(strings.ToUpper(strings.ReplaceAll(hex, ":", ""))).String()
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
		if err != nil || conn < 1 {
			return 0, 0, "", fmt.Errorf("action:rewrite invalid conn=%q (must be >= 1): %q", connStr, rule)
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
		var tb textbuf.Buffer
		content = tb.Str("rewrite:").Str(source).Byte(':').Str(dest).String()
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
		if err != nil || conn < 1 {
			return 0, 0, "", fmt.Errorf("action:sighup invalid conn=%q (must be >= 1): %q", connStr, rule)
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

	// action=close:conn=N:seq=N
	if after, ok := strings.CutPrefix(rule, "action=close:"); ok {
		kv := parseKV(after)

		connStr := kv["conn"]
		if connStr == "" {
			return 0, 0, "", fmt.Errorf("action:close missing conn: %q", rule)
		}
		conn, err = strconv.Atoi(connStr)
		if err != nil || conn < 1 {
			return 0, 0, "", fmt.Errorf("action:close invalid conn=%q (must be >= 1): %q", connStr, rule)
		}

		seqStr := kv["seq"]
		if seqStr == "" {
			return 0, 0, "", fmt.Errorf("action:close missing seq: %q", rule)
		}
		seq, err = strconv.Atoi(seqStr)
		if err != nil || seq < 1 {
			return 0, 0, "", fmt.Errorf("action:close invalid seq=%q (must be >= 1): %q", seqStr, rule)
		}

		content = actionClose
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
		if err != nil || conn < 1 {
			return 0, 0, "", fmt.Errorf("action:sigterm invalid conn=%q (must be >= 1): %q", connStr, rule)
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

// consumeMatches removes every pending check the received message satisfies.
// Caller must hold c.mu. Returns true when at least one check was consumed.
//
// Plain checks (hex, prefix, contains) keep their set semantics: at most one
// is consumed per message, whichever matches first. Ordered checks
// ("ordered:" prefix) form a strict FIFO subqueue: the front needle must be
// found in the message; one packed message may consume several consecutive
// needles, each matched at an advancing offset so intra-message order is
// enforced too. A message whose content matches only a non-front ordered
// needle consumes nothing (out-of-order delivery is a mismatch).
func (c *Checker) consumeMatches(stream string) bool {
	for i, check := range c.messages {
		if strings.HasPrefix(check, "ordered:") {
			continue
		}
		received := stream
		if !strings.HasPrefix(check, strings.Repeat("F", 32)) && !strings.Contains(check, ":") {
			received = received[32:]
		}

		if matchRule(check, received) {
			c.messages = append(c.messages[:i], c.messages[i+1:]...)
			c.updateMessagesIfRequired()
			return true
		}
	}
	return c.consumeOrdered(stream)
}

// consumeOrdered pops ordered needles from the front of the current group's
// ordered subqueue while each is found in the message at or after the
// previous needle's position. Caller must hold c.mu. The group is advanced
// only after the loop, so one message never consumes needles across a seq
// group boundary.
func (c *Checker) consumeOrdered(stream string) bool {
	// Fast path: consumeMatches calls this for every received message in every
	// check-peer test. When the current group declares no ordered needles
	// (the common case), return before allocating the uppercased stream copy.
	hasOrdered := false
	for _, check := range c.messages {
		if strings.HasPrefix(check, "ordered:") {
			hasOrdered = true
			break
		}
	}
	if !hasOrdered {
		return false
	}
	upper := strings.ToUpper(stream)
	pos := 0
	consumed := false
	for {
		front := -1
		needle := ""
		for i, check := range c.messages {
			if after, ok := strings.CutPrefix(check, "ordered:"); ok {
				front = i
				needle = after
				break
			}
		}
		if front < 0 {
			break
		}
		at := indexByteAligned(upper, needle, pos)
		if at < 0 {
			break
		}
		pos = at + len(needle)
		c.messages = append(c.messages[:front], c.messages[front+1:]...)
		consumed = true
	}
	if consumed {
		c.updateMessagesIfRequired()
	}
	return consumed
}

// indexByteAligned returns the first index >= from where needle occurs in
// stream at an EVEN offset. The stream is hex text (two chars per wire byte),
// so a match at an odd offset would straddle byte boundaries and match bytes
// that do not exist on the wire.
func indexByteAligned(stream, needle string, from int) int {
	for {
		at := strings.Index(stream[from:], needle)
		if at < 0 {
			return -1
		}
		idx := from + at
		if idx%2 == 0 {
			return idx
		}
		from = idx + 1
	}
}

// Expected checks if the received message matches expectations.
func (c *Checker) Expected(msg *Message) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	// If no expectations, accept KEEPALIVE or EOR.
	if len(c.sequences) == 0 && len(c.messages) == 0 {
		return msg.isKeepalive() || msg.IsEOR()
	}

	stream := msg.Stream()

	if c.consumeMatches(stream) {
		return true
	}

	// No match - accept KEEPALIVE anyway (normal BGP operation).
	if msg.isKeepalive() {
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
		if msg.isKeepalive() || msg.IsEOR() {
			return false, true
		}
		return false, false
	}

	stream := msg.Stream()

	if c.consumeMatches(stream) {
		return true, false
	}

	// No match against the current group. A KEEPALIVE or an End-of-RIB is
	// tolerated here: both are frames a daemon emits on its own schedule, so a
	// fixture that says nothing about them is not red merely because one arrived.
	//
	// A marker that ALSO matches an expectation the fixture still owes is
	// ambiguous, and the ambiguity cannot be settled at arrival time. Two frames
	// with the same bytes have two different meanings, and only what comes AFTER
	// tells them apart:
	//
	//   the marker the fixture declares later, arrived early. Nothing else will
	//   fill that slot, so the expectation below it fails and this frame is the
	//   one that went out of order (test/plugin/mup4.ci).
	//
	//   an extra marker the daemon emits on its own account. A SECOND, identical
	//   marker fills the declared slot later and the run is correct
	//   (test/plugin/role-otc-rs-withdraw-eor.ci: ze sends the destination its own
	//   End-of-RIB at establishment, byte-identical to the relayed one the fixture
	//   declares at seq 3).
	//
	// So the judgement is DEFERRED rather than guessed: accept the frame, record
	// it, and let the failure that follows name it (noteMisordered). Refusing it
	// here reds the second shape, and accepting it in silence with no record is
	// what left the first shape diffing an End-of-RIB rule against a withdraw,
	// blaming two frames neither of which was the one out of order.
	if msg.isKeepalive() || msg.IsEOR() {
		if c.expectedLater(stream) {
			c.noteMisordered(stream)
		}
		return false, true
	}

	// Store mismatch details for diff output.
	c.lastReceived = stream
	if len(c.messages) > 0 {
		c.lastExpected = c.messages[0]
	}

	return false, false
}

// expectedLater reports whether stream satisfies an expectation the fixture
// still owes, in the current seq group or in any group behind it. Caller must
// hold c.mu. It consumes nothing.
//
// This is what separates a frame nobody asked for from one that arrived out of
// order, and it has to look past the current group because that is exactly where
// consumeMatches cannot see. The current group is re-checked as well: an
// "ordered:" needle that is present but not at the front of its subqueue is a
// match consumeOrdered correctly refuses to consume, and it is still an
// expectation the fixture owes.
func (c *Checker) expectedLater(stream string) bool {
	if groupMatches(c.messages, stream) {
		return true
	}
	for _, group := range c.sequences {
		if groupMatches(group, stream) {
			return true
		}
	}
	return false
}

// groupMatches reports whether stream satisfies any check in one group, without
// consuming it or advancing anything. It mirrors the matching that
// consumeMatches and consumeOrdered perform, including the 32-character marker
// skip for a bare-hex check, so the two can never disagree about what "matches".
func groupMatches(checks []string, stream string) bool {
	upper := ""
	for _, check := range checks {
		if needle, ok := strings.CutPrefix(check, "ordered:"); ok {
			if upper == "" {
				upper = strings.ToUpper(stream)
			}
			if indexByteAligned(upper, needle, 0) >= 0 {
				return true
			}
			continue
		}
		received := stream
		if !strings.HasPrefix(check, strings.Repeat("F", 32)) && !strings.Contains(check, ":") && len(received) >= 32 {
			received = received[32:]
		}
		if matchRule(check, received) {
			return true
		}
	}
	return false
}

// noteMisordered records one marker that was accepted in silence while an
// expectation the fixture still owed matched it. Caller must hold c.mu.
//
// The note names the frame and the expectation that was current when it landed,
// because those two together are what a reader needs: the frame that arrived,
// and the place the fixture was up to when it did.
func (c *Checker) noteMisordered(stream string) {
	var b textbuf.Buffer
	b.Str("out-of-order marker accepted in silence: ").Str(stream)
	if len(c.messages) > 0 {
		b.Str("\n  the fixture was waiting for: ").Str(c.messages[0])
	}
	b.Str("\n  a later expectation matches this frame and the current one does not.")
	b.Str("\n  If an expectation below fails or never matches, this is the frame that arrived in the wrong place.")
	note := b.String()
	c.misorder = append(c.misorder, note)
	c.misorderPending = note
}

// takeMisorderNote returns the note for the most recent marker that arrived out
// of order, and clears it. It returns an empty string when the last silent
// accept matched no remaining expectation.
//
// The peer calls this on every silent accept so the note reaches the log at the
// moment the frame landed. That is the only report a fixture gets when the run
// ends in a TIMEOUT rather than a mismatch, which is the shape a starved host
// produces.
func (c *Checker) takeMisorderNote() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	note := c.misorderPending
	c.misorderPending = ""
	return note
}

// misorderNotes returns every out-of-order marker note recorded so far, ready to
// append to a mismatch report. It returns an empty string when there are none.
func (c *Checker) misorderNotes() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.misorder) == 0 {
		return ""
	}
	var b textbuf.Buffer
	for _, note := range c.misorder {
		b.Str("\n").Str(note)
	}
	return b.String()
}

// lastMismatch returns the expected and received values from the last mismatch.
func (c *Checker) lastMismatch() (expected, received string) {
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

// sequenceEnded returns true if the last matched message ended a connection.
// This indicates the connection should close and a new connection is expected.
// Calling this method clears the flag.
func (c *Checker) sequenceEnded() bool {
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

// nextNotificationAction checks if the next expected item is a notification: action.
// If so, it returns (true, text) and removes the action from the queue.
// If not, it returns (false, "").
func (c *Checker) nextNotificationAction() (bool, string) {
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

// nextSendAction checks if the next expected item is a send: action.
// If so, it returns (true, hexData) and removes the action from the queue.
// If not, it returns (false, "").
func (c *Checker) nextSendAction() (bool, string) {
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

// nextRewriteAction checks if the next expected item is a rewrite: action.
// If so, it returns (true, source, dest) and removes the action from the queue.
// If not, it returns (false, "", "").
func (c *Checker) nextRewriteAction() (bool, string, string) {
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

// nextCloseAction checks if the next expected item is a close action.
// If so, it returns true and removes the action from the queue.
// Close action means: close TCP without sending NOTIFICATION (triggers GR activation).
func (c *Checker) nextCloseAction() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.messages) == 0 {
		return false
	}

	if c.messages[0] != actionClose {
		return false
	}

	c.messages = c.messages[1:]
	c.updateMessagesIfRequired()

	return true
}

// nextSighupAction checks if the next expected item is a sighup action.
// If so, it returns true, removes the action from the queue, and sets
// expectClose so the next EOF is treated as expected (daemon restarts peer).
func (c *Checker) nextSighupAction() bool {
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

// nextSigtermAction checks if the next expected item is a sigterm action.
// If so, it returns true, removes the action from the queue, and sets
// expectClose so the next EOF is treated as expected (daemon shuts down).
func (c *Checker) nextSigtermAction() bool {
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

// matchRule checks if received matches the check rule.
// Rules starting with "prefix:" match if received starts with the suffix.
// Rules starting with "contains:" match if received contains the suffix.
// All other rules use exact case-insensitive comparison.
func matchRule(check, received string) bool {
	if after, ok := strings.CutPrefix(check, "prefix:"); ok {
		return strings.HasPrefix(strings.ToUpper(received), strings.ToUpper(after))
	}
	if after, ok := strings.CutPrefix(check, "contains:"); ok {
		return strings.Contains(strings.ToUpper(received), strings.ToUpper(after))
	}
	return strings.EqualFold(check, received)
}

// expectingClose returns true if the connection is expected to close
// (e.g., after a SIGHUP triggered a daemon reload). Clears the flag.
func (c *Checker) expectingClose() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	v := c.expectClose
	c.expectClose = false
	return v
}
