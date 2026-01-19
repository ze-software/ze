// Package testpeer provides a BGP test peer for functional testing.
//
// It can operate in several modes:
//   - Sink: Accept any BGP messages, reply with keepalive
//   - Echo: Accept any BGP messages, echo them back
//   - Check: Validate received messages against expected patterns
package peer

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"codeberg.org/thomas-mangin/zebgp/internal/test/ci"
)

// BGP message types.
const (
	MsgOPEN         = 1
	MsgUPDATE       = 2
	MsgNOTIFICATION = 3
	MsgKEEPALIVE    = 4
	MsgROUTEREFRESH = 5
)

// Mode specifies testpeer operation mode.
type Mode int

const (
	ModeCheck Mode = iota // Validate received messages against expectations (default)
	ModeSink              // Accept any BGP messages, reply with keepalive
	ModeEcho              // Accept any BGP messages, echo them back
)

// Mode name constants.
const (
	modeNameCheck   = "check"
	modeNameSink    = "sink"
	modeNameEcho    = "echo"
	modeNameUnknown = "unknown"
)

// String returns the mode name.
func (m Mode) String() string {
	switch m {
	case ModeCheck:
		return modeNameCheck
	case ModeSink:
		return modeNameSink
	case ModeEcho:
		return modeNameEcho
	default:
		return modeNameUnknown
	}
}

// ParseMode parses a mode string (case-insensitive).
// Returns the mode and true if valid, or ModeCheck and false if invalid.
func ParseMode(s string) (Mode, bool) {
	switch strings.ToLower(s) {
	case modeNameCheck:
		return ModeCheck, true
	case modeNameSink:
		return ModeSink, true
	case modeNameEcho:
		return ModeEcho, true
	default:
		return ModeCheck, false
	}
}

// BGP header length.
const HeaderLen = 19

// BGP marker (16 bytes of 0xFF).
var Marker = []byte{
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
}

// Config holds test peer configuration.
type Config struct {
	// Port to listen on (default 179)
	Port int
	// ASN to use in OPEN message (0 = extract from peer OPEN)
	ASN int
	// TCPConnections is the number of TCP connections to accept for multi-connection tests.
	// Used with option:tcp_connections:N in .ci files. Default 1.
	TCPConnections int
	// Mode: operation mode (check, sink, echo). Default ModeCheck.
	Mode Mode
	// IPv6: bind to IPv6 instead of IPv4
	IPv6 bool
	// Decode: decode messages to human-readable format in output
	Decode bool
	// SendUnknownCapability: add unknown capability 66 to OPEN message
	SendUnknownCapability bool
	// SendDefaultRoute: send a default route (0.0.0.0/0) UPDATE after OPEN
	SendDefaultRoute bool
	// InspectOpenMessage: validate received OPEN message against expectations
	InspectOpenMessage bool
	// SendUnknownMessage: send an unknown message type (255) after OPEN
	SendUnknownMessage bool
	// Expect: list of expected messages from .ci file
	Expect []string
	// Output: writer for logging (defaults to os.Stdout)
	Output io.Writer
}

// Result holds the test result.
type Result struct {
	Success bool
	Error   error
}

// Peer is a BGP test peer.
type Peer struct {
	config  *Config
	checker *Checker
	output  io.Writer
}

// New creates a new test peer.
// Returns error if expect rules are invalid.
func New(config *Config) (*Peer, error) {
	output := config.Output
	if output == nil {
		output = os.Stdout
	}
	checker, err := NewChecker(config.Expect)
	if err != nil {
		return nil, fmt.Errorf("invalid expect rules: %w", err)
	}
	return &Peer{
		config:  config,
		checker: checker,
		output:  output,
	}, nil
}

// Run starts the test peer and blocks until completion or context cancellation.
func (p *Peer) Run(ctx context.Context) Result {
	host := "127.0.0.1"
	if p.config.IPv6 {
		host = "::1"
	}
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", p.config.Port))

	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", addr)
	if err != nil {
		return Result{Success: false, Error: fmt.Errorf("listen: %w", err)}
	}
	defer func() { _ = ln.Close() }()

	p.printf("listening on %s\n", addr)

	resultCh := make(chan Result, 1)

	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()

	// Track connection count for multi-connection tests
	connCount := 0
	maxConns := p.config.TCPConnections
	if maxConns <= 0 {
		maxConns = 1
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return Result{Success: true}
			default:
				if !errors.Is(err, net.ErrClosed) {
					p.printf("accept error: %v\n", err)
				}
				continue
			}
		}

		connCount++

		// Disable Nagle's algorithm to reduce message batching delays
		if tcpConn, ok := conn.(*net.TCPConn); ok {
			_ = tcpConn.SetNoDelay(true)
		}

		go func(c net.Conn) {
			defer func() { _ = c.Close() }()
			result := p.handleConnection(ctx, c)
			select {
			case resultCh <- result:
			default:
			}
		}(conn)

		// In sink/echo mode, don't wait for completion
		if p.config.Mode != ModeCheck {
			continue
		}

		// Wait for connection to complete
		select {
		case result := <-resultCh:
			if !result.Success {
				return result
			}
			// Check if all expectations are met
			if p.checker.Completed() {
				return Result{Success: true}
			}
			// More connections expected - continue accepting
			p.printf("\nwaiting for next connection (%d/%d)...\n", connCount, maxConns)
			continue
		case <-ctx.Done():
			return Result{Success: false, Error: fmt.Errorf("context cancelled")}
		}
	}
}

func (p *Peer) printf(format string, args ...interface{}) {
	_, _ = fmt.Fprintf(p.output, format, args...)
}

func (p *Peer) handleConnection(ctx context.Context, conn net.Conn) Result {
	p.printf("\nnew connection from %s\n", conn.RemoteAddr())

	p.checker.Init()

	// Read OPEN.
	header, body, err := ReadMessage(conn)
	if err != nil {
		return Result{Success: false, Error: fmt.Errorf("read OPEN: %w", err)}
	}

	if header[18] != MsgOPEN {
		return Result{Success: false, Error: fmt.Errorf("expected OPEN, got type %d", header[18])}
	}

	p.printf("\nnew session:\n")
	p.printPayload("open recv", header, body)

	// Generate and send our OPEN.
	ourOpen := p.generateOpen(header, body)
	p.printPayload("open sent", ourOpen[:19], ourOpen[19:])
	if _, err := conn.Write(ourOpen); err != nil {
		return Result{Success: false, Error: fmt.Errorf("write OPEN: %w", err)}
	}

	// Send KEEPALIVE.
	if _, err := conn.Write(KeepaliveMsg()); err != nil {
		return Result{Success: false, Error: fmt.Errorf("write KEEPALIVE: %w", err)}
	}

	// Send unknown message type if requested.
	if p.config.SendUnknownMessage {
		unknown := make([]byte, 19)
		copy(unknown, Marker)
		binary.BigEndian.PutUint16(unknown[16:], 19)
		unknown[18] = 255
		_, _ = conn.Write(unknown)
	}

	// Check OPEN if requested.
	if p.config.InspectOpenMessage {
		msg := &Message{Header: header, Body: body}
		if !p.checker.Expected(msg) {
			return Result{Success: false, Error: errors.New("OPEN mismatch")}
		}
		if p.checker.Completed() {
			return Result{Success: true}
		}
	}

	// Send default route if requested.
	if p.config.SendDefaultRoute {
		p.printf("sending default-route\n")
		if _, err := conn.Write(DefaultRouteMsg()); err != nil {
			return Result{Success: false, Error: fmt.Errorf("write default route: %w", err)}
		}
	}

	// Check for notification action after OPEN handshake.
	if ok, text := p.checker.NextNotificationAction(); ok {
		p.printf("\nsending notification: %q\n", text)
		if _, err := conn.Write(NotificationMsg(text)); err != nil {
			return Result{Success: false, Error: fmt.Errorf("write notification: %w", err)}
		}
		// Notification closes the session.
		if p.checker.Completed() {
			return Result{Success: true}
		}
		// More sequences expected - connection will close and client should reconnect.
		return Result{Success: true}
	}

	// Check for send action after OPEN handshake.
	for {
		ok, hexData := p.checker.NextSendAction()
		if !ok {
			break
		}
		data, err := hex.DecodeString(hexData)
		if err != nil {
			return Result{Success: false, Error: fmt.Errorf("invalid send hex: %w", err)}
		}
		p.printf("\nsending %d bytes to peer\n", len(data))
		if _, err := conn.Write(data); err != nil {
			return Result{Success: false, Error: fmt.Errorf("write send: %w", err)}
		}
		if p.checker.Completed() {
			return Result{Success: true}
		}
	}

	// Main message loop.
	counter := 0
	for {
		select {
		case <-ctx.Done():
			return Result{Success: false, Error: fmt.Errorf("context cancelled")}
		default:
		}

		_ = conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		header, body, err := ReadMessage(conn)
		if err != nil {
			if isTimeout(err) {
				continue
			}
			if errors.Is(err, io.EOF) {
				if p.checker.Completed() {
					return Result{Success: true}
				}
				return Result{Success: false, Error: errors.New("connection closed before completion")}
			}
			return Result{Success: false, Error: fmt.Errorf("read: %w", err)}
		}

		msg := &Message{Header: header, Body: body}

		// For sink/echo modes, handle all messages
		if p.config.Mode == ModeSink {
			counter++
			p.printPayload("msg  recv", header, body)
			p.printPayload(fmt.Sprintf("sank    #%d", counter), header, body)
			_, _ = conn.Write(KeepaliveMsg())
			continue
		}

		if p.config.Mode == ModeEcho {
			counter++
			p.printPayload("msg  recv", header, body)
			p.printPayload(fmt.Sprintf("echo'd  #%d", counter), header, body)
			_, _ = conn.Write(append(header, body...))
			continue
		}

		// Check mode: try to match message against expectations
		_, _ = conn.Write(KeepaliveMsg())

		matched, silentAccept := p.checker.ExpectedOrKeepalive(msg)
		if silentAccept {
			// KEEPALIVE not in expectations, silently accept
			continue
		}

		// Count and print non-silent messages
		counter++
		p.printPayload("msg  recv", header, body)

		if !matched {
			expected, received := p.checker.LastMismatch()
			diff := Diff(expected, received)
			return Result{Success: false, Error: fmt.Errorf("message mismatch%s", diff)}
		}

		if p.checker.Completed() {
			return Result{Success: true}
		}

		// Check if this message completed a sequence - connection should close
		// and a new connection is expected.
		if p.checker.SequenceEnded() {
			// More sequences expected - let connection close and wait for reconnect.
			return Result{Success: true}
		}

		// Check for notification action after matched message.
		if ok, text := p.checker.NextNotificationAction(); ok {
			p.printf("\nsending notification: %q\n", text)
			if _, err := conn.Write(NotificationMsg(text)); err != nil {
				return Result{Success: false, Error: fmt.Errorf("write notification: %w", err)}
			}
			if p.checker.Completed() {
				return Result{Success: true}
			}
			// More sequences expected - connection will close and client should reconnect.
			return Result{Success: true}
		}

		// Check for send action after matched message.
		// Unlike notification, send doesn't close connection - continue loop.
		for {
			ok, hexData := p.checker.NextSendAction()
			if !ok {
				break
			}
			data, err := hex.DecodeString(hexData)
			if err != nil {
				return Result{Success: false, Error: fmt.Errorf("invalid send hex: %w", err)}
			}
			p.printf("\nsending %d bytes to peer\n", len(data))
			if _, err := conn.Write(data); err != nil {
				return Result{Success: false, Error: fmt.Errorf("write send: %w", err)}
			}
			if p.checker.Completed() {
				return Result{Success: true}
			}
		}
	}
}

func (p *Peer) generateOpen(peerHeader, peerBody []byte) []byte {
	open := make([]byte, len(peerHeader)+len(peerBody))
	copy(open, peerHeader)
	copy(open[19:], peerBody)

	if len(peerBody) > 8 {
		open[19+8] = (peerBody[8] + 1) & 0xFF
	}

	if p.config.ASN > 0 && p.config.ASN <= 65535 {
		binary.BigEndian.PutUint16(open[19+1:], uint16(p.config.ASN)) //nolint:gosec // ASN validated
	}

	if p.config.SendUnknownCapability {
		cap66 := []byte{66, 10, 'l', 'o', 'r', 'e', 'm', 'i', 'p', 's', 'u', 'm'}
		param := append([]byte{2, byte(len(cap66))}, cap66...)

		oldLen := binary.BigEndian.Uint16(open[16:])
		paramLen := len(param)
		if paramLen > 65535-int(oldLen) {
			paramLen = 65535 - int(oldLen)
		}
		newLen := oldLen + uint16(paramLen) //nolint:gosec // Bounds checked
		binary.BigEndian.PutUint16(open[16:], newLen)
		open[19+9] += byte(len(param))
		open = append(open, param...)
	}

	return open
}

func (p *Peer) printPayload(prefix string, header, body []byte) {
	h := strings.ToUpper(hex.EncodeToString(header))
	b := strings.ToUpper(hex.EncodeToString(body))

	if len(h) >= 38 {
		p.printf("%-12s%s:%s:%s:%s\n", prefix, h[:32], h[32:36], h[36:38], b)
	} else {
		p.printf("%-12s%s%s\n", prefix, h, b)
	}

	// Show decoded output if enabled
	if p.config.Decode {
		fullMsg := make([]byte, len(header)+len(body))
		copy(fullMsg, header)
		copy(fullMsg[len(header):], body)
		if decoded, err := DecodeMessageBytes(fullMsg); err == nil {
			for _, line := range strings.Split(decoded.String(), "\n") {
				if line != "" {
					p.printf("             %s\n", line)
				}
			}
		}
	}
}

// ReadMessage reads a BGP message from a connection.
func ReadMessage(conn net.Conn) ([]byte, []byte, error) {
	header := make([]byte, HeaderLen)
	if _, err := io.ReadFull(conn, header); err != nil {
		return nil, nil, err
	}

	length := binary.BigEndian.Uint16(header[16:18])
	if length < HeaderLen {
		return nil, nil, fmt.Errorf("invalid message length: %d", length)
	}

	bodyLen := int(length) - HeaderLen
	body := make([]byte, bodyLen)
	if bodyLen > 0 {
		if _, err := io.ReadFull(conn, body); err != nil {
			return nil, nil, err
		}
	}

	return header, body, nil
}

// KeepaliveMsg returns a BGP KEEPALIVE message.
func KeepaliveMsg() []byte {
	msg := make([]byte, 19)
	copy(msg, Marker)
	binary.BigEndian.PutUint16(msg[16:], 19)
	msg[18] = MsgKEEPALIVE
	return msg
}

// DefaultRouteMsg returns an UPDATE with route 0.0.0.0/32.
// Used for testing UPDATE receive handling.
func DefaultRouteMsg() []byte {
	return []byte{
		// BGP Header (16 bytes marker + 2 bytes length + 1 byte type)
		0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
		0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
		0x00, 0x31, // Length: 49 bytes (19 header + 30 body)
		0x02, // Type: UPDATE
		// UPDATE body (30 bytes)
		0x00, 0x00, // Withdrawn routes length: 0
		0x00, 0x15, // Path attributes length: 21
		// ORIGIN: IGP (0) - 4 bytes
		0x40, 0x01, 0x01, 0x00,
		// AS_PATH: empty - 3 bytes
		0x40, 0x02, 0x00,
		// NEXT_HOP: 127.0.0.1 - 7 bytes
		0x40, 0x03, 0x04, 0x7F, 0x00, 0x00, 0x01,
		// LOCAL_PREF: 100 - 7 bytes
		0x40, 0x05, 0x04, 0x00, 0x00, 0x00, 0x64,
		// NLRI: 0.0.0.0/32 - 5 bytes
		0x20,                   // Prefix length: 32 bits
		0x00, 0x00, 0x00, 0x00, // Prefix: 0.0.0.0
	}
}

// NotificationMsg builds a BGP NOTIFICATION message with Cease/Administrative Shutdown.
// RFC 4271 Section 4.5 - NOTIFICATION Message Format.
// RFC 9003 - Extended BGP Administrative Shutdown Communication.
//
// Format: [Error Code 6][Subcode 2][Length][Shutdown Communication]
// - Error Code: 6 (Cease)
// - Subcode: 2 (Administrative Shutdown)
// - Length: 1 byte (0-255)
// - Shutdown Communication: UTF-8, max 255 bytes per RFC 9003.
func NotificationMsg(text string) []byte {
	textBytes := []byte(text)

	// RFC 9003: max 255 octets for shutdown communication
	// Must truncate at valid UTF-8 boundary to maintain RFC compliance
	if len(textBytes) > 255 {
		textBytes = truncateUTF8(textBytes, 255)
	}

	// Header (19) + Error Code (1) + Subcode (1) + Length (1) + Text
	msgLen := 19 + 3 + len(textBytes)

	msg := make([]byte, msgLen)
	copy(msg, Marker)
	binary.BigEndian.PutUint16(msg[16:], uint16(msgLen)) //nolint:gosec // msgLen max 277
	msg[18] = MsgNOTIFICATION
	msg[19] = 6                    // Cease
	msg[20] = 2                    // Administrative Shutdown (RFC 9003)
	msg[21] = byte(len(textBytes)) // Length of shutdown communication
	copy(msg[22:], textBytes)

	return msg
}

// truncateUTF8 truncates bytes to maxLen while preserving valid UTF-8.
// It finds the last valid rune boundary at or before maxLen.
func truncateUTF8(b []byte, maxLen int) []byte {
	if len(b) <= maxLen {
		return b
	}

	// Start at maxLen and work backwards to find valid UTF-8 boundary
	for i := maxLen; i > 0; i-- {
		if utf8.RuneStart(b[i]) {
			// Found a rune start - check if there's room for the full rune
			_, size := utf8.DecodeRune(b[i:])
			if i+size <= maxLen {
				return b[:i+size]
			}
			// Rune would exceed maxLen, try previous position
			continue
		}
	}

	// Fallback: no valid boundary found (shouldn't happen with valid UTF-8)
	return b[:maxLen]
}

func isTimeout(err error) bool {
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout()
	}
	return false
}

// Message represents a BGP message.
type Message struct {
	Header []byte
	Body   []byte
}

// Kind returns the message type.
func (m *Message) Kind() byte {
	if len(m.Header) > 18 {
		return m.Header[18]
	}
	return 0
}

// IsKeepalive returns true if this is a KEEPALIVE message.
func (m *Message) IsKeepalive() bool { return m.Kind() == MsgKEEPALIVE }

// IsUpdate returns true if this is an UPDATE message.
func (m *Message) IsUpdate() bool { return m.Kind() == MsgUPDATE }

// IsEOR returns true if this is an End-of-RIB marker.
func (m *Message) IsEOR() bool {
	if !m.IsUpdate() {
		return false
	}
	return len(m.Body) == 4 || len(m.Body) == 11
}

// Stream returns the hex-encoded message.
func (m *Message) Stream() string {
	return strings.ToUpper(hex.EncodeToString(append(m.Header, m.Body...)))
}

// Checker validates received messages against expected patterns.
type Checker struct {
	messages            []string
	sequences           [][]string
	connectionIDs       []int  // Connection number (1-4) for each sequence
	currentConnection   int    // Current connection number (0 = none)
	lastExpected        string // For diff output on mismatch
	lastReceived        string // For diff output on mismatch
	connectionJustEnded bool   // True if last match ended a connection (not just sequence)
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
	if strings.HasPrefix(rule, "expect=bgp:") {
		kv := parseKV(strings.TrimPrefix(rule, "expect=bgp:"))

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
	if strings.HasPrefix(rule, "action=notification:") {
		kv := parseKV(strings.TrimPrefix(rule, "action=notification:"))

		connStr := kv["conn"]
		if connStr == "" {
			return 0, 0, "", fmt.Errorf("action=notification missing conn: %q", rule)
		}
		conn, err = strconv.Atoi(connStr)
		if err != nil || conn < 1 || conn > 4 {
			return 0, 0, "", fmt.Errorf("action=notification invalid conn=%q (must be 1-4): %q", connStr, rule)
		}

		seqStr := kv["seq"]
		if seqStr == "" {
			return 0, 0, "", fmt.Errorf("action=notification missing seq: %q", rule)
		}
		seq, err = strconv.Atoi(seqStr)
		if err != nil || seq < 1 {
			return 0, 0, "", fmt.Errorf("action=notification invalid seq=%q (must be >= 1): %q", seqStr, rule)
		}

		text := kv["text"]
		if text == "" {
			return 0, 0, "", fmt.Errorf("action=notification missing text: %q", rule)
		}
		content = "notification:" + text
		return conn, seq, content, nil
	}

	// action=send:conn=N:seq=N:hex=...
	if strings.HasPrefix(rule, "action=send:") {
		kv := parseKV(strings.TrimPrefix(rule, "action=send:"))

		connStr := kv["conn"]
		if connStr == "" {
			return 0, 0, "", fmt.Errorf("action=send missing conn: %q", rule)
		}
		conn, err = strconv.Atoi(connStr)
		if err != nil || conn < 1 || conn > 4 {
			return 0, 0, "", fmt.Errorf("action=send invalid conn=%q (must be 1-4): %q", connStr, rule)
		}

		seqStr := kv["seq"]
		if seqStr == "" {
			return 0, 0, "", fmt.Errorf("action=send missing seq: %q", rule)
		}
		seq, err = strconv.Atoi(seqStr)
		if err != nil || seq < 1 {
			return 0, 0, "", fmt.Errorf("action=send invalid seq=%q (must be >= 1): %q", seqStr, rule)
		}

		hex := kv["hex"]
		if hex == "" {
			return 0, 0, "", fmt.Errorf("action=send missing hex: %q", rule)
		}
		content = "send:" + strings.ToUpper(strings.ReplaceAll(hex, ":", ""))
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

// LoadExpectFile loads expected messages from a file.
// Uses new key=value format: action=type:key=value:key=value:...
func LoadExpectFile(path string) ([]string, *Config, error) {
	f, err := os.Open(path) //nolint:gosec // Path from user input (CLI arg)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = f.Close() }()

	config := &Config{}
	var expect []string

	lineNum := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse new format: action=type:key=value:...
		eqIdx := strings.Index(line, "=")
		if eqIdx == -1 {
			return nil, nil, fmt.Errorf("line %d: invalid format %q", lineNum, line)
		}

		action := line[:eqIdx]
		rest := line[eqIdx+1:]
		parts := strings.Split(rest, ":")
		if len(parts) == 0 {
			continue
		}
		lineType := parts[0]
		kv := ci.ParseKVPairs(parts[1:])

		switch action {
		case "option":
			parseOptionConfig(config, lineType, kv)

		case "expect":
			if lineType == "bgp" {
				// Pass through new format: expect=bgp:conn=N:seq=N:hex=...
				expect = append(expect, line)
			}
			// Ignore json, stderr, syslog - handled by test runner

		case "action":
			if lineType == "notification" {
				// Pass through new format: action=notification:conn=N:seq=N:text=...
				expect = append(expect, line)
			}
			if lineType == "send" {
				// Pass through new format: action=send:conn=N:seq=N:hex=...
				expect = append(expect, line)
			}

		case "cmd":
			// Ignore - documentation only

		case "reject":
			// Ignore - handled by test runner
		}
	}

	return expect, config, scanner.Err()
}

// parseOptionConfig parses option lines into Config.
func parseOptionConfig(config *Config, optType string, kv map[string]string) {
	switch optType {
	case "file":
		// Ignored - handled by test runner

	case "asn":
		if v, err := strconv.Atoi(kv["value"]); err == nil {
			config.ASN = v
		}

	case "bind":
		if kv["value"] == "ipv6" {
			config.IPv6 = true
		}

	case "tcp_connections":
		if v, err := strconv.Atoi(kv["value"]); err == nil {
			config.TCPConnections = v
		}

	case "open":
		switch kv["value"] {
		case "send-unknown-capability":
			config.SendUnknownCapability = true
		case "inspect-open-message":
			config.InspectOpenMessage = true
		case "send-unknown-message":
			config.SendUnknownMessage = true
		}

	case "update":
		if kv["value"] == "send-default-route" {
			config.SendDefaultRoute = true
		}

	case "timeout", "env":
		// Ignored - handled by test runner
	}
}
