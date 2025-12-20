// Package testpeer provides a BGP test peer for functional testing.
//
// It can operate in several modes:
//   - Sink: Accept any BGP messages, reply with keepalive
//   - Echo: Accept any BGP messages, echo them back
//   - Check: Validate received messages against expected patterns
package testpeer

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
)

// BGP message types.
const (
	MsgOPEN         = 1
	MsgUPDATE       = 2
	MsgNOTIFICATION = 3
	MsgKEEPALIVE    = 4
	MsgROUTEREFRESH = 5
)

// BGP header length.
const HeaderLen = 19

// BGP marker (16 bytes of 0xFF).
var Marker = []byte{
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
}

// Config holds test peer configuration.
type Config struct {
	Port                  int
	ASN                   int
	Sink                  bool
	Echo                  bool
	IPv6                  bool
	SendUnknownCapability bool
	SendDefaultRoute      bool
	InspectOpenMessage    bool
	SendUnknownMessage    bool
	Expect                []string
	Output                io.Writer // For logging (defaults to os.Stdout)
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
func New(config *Config) *Peer {
	output := config.Output
	if output == nil {
		output = os.Stdout
	}
	return &Peer{
		config:  config,
		checker: NewChecker(config.Expect),
		output:  output,
	}
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

		go func(c net.Conn) {
			defer func() { _ = c.Close() }()
			result := p.handleConnection(ctx, c)
			select {
			case resultCh <- result:
			default:
			}
		}(conn)

		// In check mode, wait for first connection to complete.
		if !p.config.Sink && !p.config.Echo {
			select {
			case result := <-resultCh:
				return result
			case <-ctx.Done():
				return Result{Success: true}
			}
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

	// Main message loop.
	counter := 0
	for {
		select {
		case <-ctx.Done():
			return Result{Success: true}
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

		counter++
		msg := &Message{Header: header, Body: body}
		p.printPayload("msg  recv", header, body)

		if p.config.Sink {
			p.printPayload(fmt.Sprintf("sank    #%d", counter), header, body)
			_, _ = conn.Write(KeepaliveMsg())
			continue
		}

		if p.config.Echo {
			p.printPayload(fmt.Sprintf("echo'd  #%d", counter), header, body)
			_, _ = conn.Write(append(header, body...))
			continue
		}

		_, _ = conn.Write(KeepaliveMsg())

		if !p.checker.Expected(msg) {
			return Result{Success: false, Error: errors.New("message mismatch")}
		}

		if p.checker.Completed() {
			return Result{Success: true}
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

// DefaultRouteMsg returns an UPDATE with default route 0.0.0.0/0.
func DefaultRouteMsg() []byte {
	return []byte{
		0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
		0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
		0x00, 0x31, 0x02, 0x00, 0x00, 0x00, 0x15,
		0x40, 0x01, 0x01, 0x00,
		0x40, 0x02, 0x00,
		0x40, 0x03, 0x04, 0x7F, 0x00, 0x00, 0x01,
		0x40, 0x05, 0x04, 0x00, 0x00, 0x00, 0x64,
		0x00,
	}
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
	messages  []string
	sequences [][]string
	mu        sync.Mutex
}

// NewChecker creates a new checker from expected messages.
func NewChecker(expected []string) *Checker {
	c := &Checker{}
	c.sequences = c.groupMessages(expected)
	return c
}

func (c *Checker) groupMessages(expected []string) [][]string {
	groups := make(map[string]map[int][]string)

	for _, rule := range expected {
		if !strings.Contains(rule, "notification:") {
			rule = strings.ToLower(strings.ReplaceAll(rule, " ", ""))
		}

		parts := strings.SplitN(rule, ":", 3)
		if len(parts) < 3 {
			continue
		}

		prefix, encoding, content := parts[0], parts[1], parts[2]

		conn := "A"
		var seq int
		if len(prefix) > 0 && prefix[0] >= 'A' && prefix[0] <= 'Z' {
			conn = string(prefix[0])
			seq, _ = strconv.Atoi(prefix[1:])
		} else {
			seq, _ = strconv.Atoi(prefix)
		}
		if seq == 0 {
			seq = 1
		}

		if groups[conn] == nil {
			groups[conn] = make(map[int][]string)
		}

		if encoding == "raw" {
			content = strings.ToUpper(strings.ReplaceAll(content, ":", ""))
			groups[conn][seq] = append(groups[conn][seq], content)
		} else {
			groups[conn][seq] = append(groups[conn][seq], fmt.Sprintf("%s:%s", strings.ToLower(encoding), strings.ToLower(content)))
		}
	}

	var result [][]string
	for _, conn := range []string{"A", "B", "C", "D"} {
		if groups[conn] == nil {
			continue
		}
		for seq := 1; seq <= 100; seq++ {
			if msgs := groups[conn][seq]; len(msgs) > 0 {
				result = append(result, msgs)
			}
		}
	}

	return result
}

// Init initializes the checker for a new session.
func (c *Checker) Init() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.messages) > 0 {
		return false
	}
	if len(c.sequences) == 0 {
		return false
	}

	c.messages = c.sequences[0]
	c.sequences = c.sequences[1:]
	return true
}

// Expected checks if the received message matches expectations.
func (c *Checker) Expected(msg *Message) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	// If no expectations, accept keepalives and EOR
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

	return false
}

func (c *Checker) updateMessagesIfRequired() {
	if len(c.messages) == 0 && len(c.sequences) > 0 {
		c.messages = c.sequences[0]
		c.sequences = c.sequences[1:]
	}
}

// Completed returns true if all expected messages have been received.
func (c *Checker) Completed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.messages) == 0 && len(c.sequences) == 0
}

// LoadExpectFile loads expected messages from a file.
func LoadExpectFile(path string) ([]string, *Config, error) {
	f, err := os.Open(path) //nolint:gosec // Path from user input (CLI arg)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = f.Close() }()

	config := &Config{}
	var expect []string

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		switch {
		case line == "option:bind:ipv6":
			config.IPv6 = true
		case line == "option:open:send-unknown-capability":
			config.SendUnknownCapability = true
		case line == "option:open:inspect-open-message":
			config.InspectOpenMessage = true
		case line == "option:update:send-default-route":
			config.SendDefaultRoute = true
		case line == "option:open:send-unknown-message":
			config.SendUnknownMessage = true
		case strings.HasPrefix(line, "option:asn:"):
			if v, err := strconv.Atoi(strings.TrimPrefix(line, "option:asn:")); err == nil {
				config.ASN = v
			}
		default:
			expect = append(expect, line)
		}
	}

	return expect, config, scanner.Err()
}
