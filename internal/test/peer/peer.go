// Design: docs/architecture/testing/ci-format.md — test BGP peer
// Detail: message.go — BGP message types, wire helpers, and Message struct
// Detail: checker.go — message validation against expected patterns
// Detail: expect.go — .ci file loading and option parsing
//
// Package testpeer provides a BGP test peer for functional testing.
//
// It can operate in several modes:
//   - Sink: Accept any BGP messages, reply with keepalive
//   - Echo: Accept any BGP messages, echo them back
//   - Check: Validate received messages against expected patterns
package peer

import (
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
	"syscall"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/test/decode"
)

// ErrOpenMismatch is returned when the received OPEN message doesn't match expectations.
var ErrOpenMismatch = errors.New("OPEN mismatch")

// ErrConnectionClosed is returned when the connection closes before all expected messages are received.
var ErrConnectionClosed = errors.New("connection closed before completion")

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

// CapabilityOverride specifies a capability to add or remove from the mirrored OPEN.
// Used to test require/refuse enforcement by controlling which capabilities ze-peer advertises.
type CapabilityOverride struct {
	Code  uint8  // Capability code (e.g., 65 for ASN4)
	Value []byte // Capability value bytes (only for Add)
	Add   bool   // true=add capability, false=drop capability
}

// Config holds test peer configuration.
type Config struct {
	// Port to listen on (default 179)
	Port int
	// ASN to use in OPEN message (0 = extract from peer OPEN)
	ASN int
	// BindAddr overrides the listen address (default "127.0.0.1", or "::1" if IPv6).
	// Useful for multi-peer tests where each peer listens on a different loopback address.
	BindAddr string
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
	// SendRoutes: custom routes to send after OPEN (option=update:value=send-route:...)
	SendRoutes []RouteToSend
	// InspectOpenMessage: validate received OPEN message against expectations
	InspectOpenMessage bool
	// SendUnknownMessage: send an unknown message type (255) after OPEN
	SendUnknownMessage bool
	// CapabilityOverrides: capabilities to add/remove from mirrored OPEN response
	CapabilityOverrides []CapabilityOverride
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
	config    *Config
	checker   *Checker
	output    io.Writer
	mu        sync.Mutex    // protects output writes from concurrent connection handlers
	ready     chan struct{} // closed when listener is bound and accepting
	readyOnce sync.Once     // ensures ready is closed exactly once
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
		ready:   make(chan struct{}),
	}, nil
}

// Ready returns a channel that is closed when the listener is bound and accepting.
func (p *Peer) Ready() <-chan struct{} {
	return p.ready
}

// Run starts the test peer and blocks until completion or context cancellation.
func (p *Peer) Run(ctx context.Context) Result {
	host := p.config.BindAddr
	if host == "" {
		host = "127.0.0.1"
		if p.config.IPv6 {
			host = "::1"
		}
	}
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", p.config.Port))

	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", addr)
	if err != nil {
		return Result{Success: false, Error: fmt.Errorf("listen: %w", err)}
	}
	defer func() { _ = ln.Close() }()

	p.printf("listening on %s\n", addr)
	p.readyOnce.Do(func() { close(p.ready) }) // signal that listener is bound

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
			return Result{Success: false, Error: fmt.Errorf("context canceled")}
		}
	}
}

func (p *Peer) printf(format string, args ...any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, err := fmt.Fprintf(p.output, format, args...); err != nil {
		return // best-effort test output
	}
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
			return Result{Success: false, Error: ErrOpenMismatch}
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

	// Send custom routes if configured.
	for _, route := range p.config.SendRoutes {
		p.printf("sending route %s origin-as=%d\n", route.Prefix, route.OriginAS)
		msg, err := BuildRouteMsg(route)
		if err != nil {
			return Result{Success: false, Error: fmt.Errorf("build route %s: %w", route.Prefix, err)}
		}
		if _, err := conn.Write(msg); err != nil {
			return Result{Success: false, Error: fmt.Errorf("write route %s: %w", route.Prefix, err)}
		}
	}

	// Check for close action after OPEN handshake.
	// Close without NOTIFICATION — triggers GR activation in ze.
	if p.checker.NextCloseAction() {
		p.printf("\nclosing connection (action=close)\n")
		return Result{Success: true}
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

	// Check for close action after OPEN sends (send → close sequence).
	if p.checker.NextCloseAction() {
		p.printf("\nclosing connection (action=close)\n")
		return Result{Success: true}
	}

	// Main message loop.
	counter := 0
	for {
		select {
		case <-ctx.Done():
			return Result{Success: false, Error: fmt.Errorf("context canceled")}
		default:
		}

		_ = conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		header, body, err := ReadMessage(conn)
		if err != nil {
			if isTimeout(err) {
				continue
			}
			if errors.Is(err, io.EOF) || isConnReset(err) {
				if p.checker.Completed() {
					return Result{Success: true}
				}
				// Connection closed (EOF) or reset (RST). Both mean the remote
				// side is done. If all expectations are not yet met, check if
				// a reconnection is expected (sighup reload, prefix teardown).
				if p.checker.ExpectingClose() {
					return Result{Success: true}
				}
				return Result{Success: false, Error: ErrConnectionClosed}
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
			diff := decode.Diff(expected, received)
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

		// Check for close action after matched message.
		// Close without NOTIFICATION — triggers GR activation in ze.
		if p.checker.NextCloseAction() {
			p.printf("\nclosing connection (action=close)\n")
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

		// Check for close action after sends (send → close sequence).
		if p.checker.NextCloseAction() {
			p.printf("\nclosing connection (action=close)\n")
			return Result{Success: true}
		}

		// Check for rewrite action after matched message.
		// Copies source file to dest in the peer's working directory (tmpfs).
		for {
			ok, source, dest := p.checker.NextRewriteAction()
			if !ok {
				break
			}
			p.printf("\nrewriting %s -> %s\n", source, dest)
			data, err := os.ReadFile(source) //nolint:gosec // test peer, path from .ci file
			if err != nil {
				return Result{Success: false, Error: fmt.Errorf("rewrite read %s: %w", source, err)}
			}
			if err := os.WriteFile(dest, data, 0o600); err != nil {
				return Result{Success: false, Error: fmt.Errorf("rewrite write %s: %w", dest, err)}
			}
		}

		// Check for sighup action after matched message.
		// Reads daemon PID from daemon.pid and sends SIGHUP.
		if p.checker.NextSighupAction() {
			pidData, err := os.ReadFile("daemon.pid")
			if err != nil {
				return Result{Success: false, Error: fmt.Errorf("read daemon.pid: %w", err)}
			}
			pid, err := strconv.Atoi(strings.TrimSpace(string(pidData)))
			if err != nil {
				return Result{Success: false, Error: fmt.Errorf("parse daemon.pid: %w", err)}
			}
			p.printf("\nsending SIGHUP to pid %d\n", pid)
			if err := syscall.Kill(pid, syscall.SIGHUP); err != nil {
				return Result{Success: false, Error: fmt.Errorf("sighup pid %d: %w", pid, err)}
			}
			// Brief pause to let the daemon process the signal before we continue.
			time.Sleep(500 * time.Millisecond)
			if p.checker.Completed() {
				return Result{Success: true}
			}
		}

		// Check for sigterm action after matched message.
		// Reads daemon PID from daemon.pid and sends SIGTERM.
		if p.checker.NextSigtermAction() {
			pidData, err := os.ReadFile("daemon.pid")
			if err != nil {
				return Result{Success: false, Error: fmt.Errorf("read daemon.pid: %w", err)}
			}
			pid, err := strconv.Atoi(strings.TrimSpace(string(pidData)))
			if err != nil {
				return Result{Success: false, Error: fmt.Errorf("parse daemon.pid: %w", err)}
			}
			p.printf("\nsending SIGTERM to pid %d\n", pid)
			if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
				return Result{Success: false, Error: fmt.Errorf("sigterm pid %d: %w", pid, err)}
			}
			// Brief pause to let the daemon shut down before we continue.
			time.Sleep(500 * time.Millisecond)
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

	// Apply capability overrides (drop/add) before SendUnknownCapability.
	if len(p.config.CapabilityOverrides) > 0 {
		open = applyCapabilityOverrides(open, p.config.CapabilityOverrides)
	}

	if p.config.SendUnknownCapability {
		cap66 := []byte{66, 10, 'l', 'o', 'r', 'e', 'm', 'i', 'p', 's', 'u', 'm'}
		param := append([]byte{2, byte(len(cap66))}, cap66...)

		oldLen := binary.BigEndian.Uint16(open[16:])
		paramLen := min(len(param), 65535-int(oldLen))
		newLen := oldLen + uint16(paramLen) //nolint:gosec // Bounds checked
		binary.BigEndian.PutUint16(open[16:], newLen)
		open[19+9] += byte(len(param))
		open = append(open, param...)
	}

	return open
}

// applyCapabilityOverrides modifies OPEN optional parameters by dropping/adding capabilities.
// Handles both per-capability wrapping (each cap in its own type-2 parameter) and RFC 5492
// bundled format (all caps in a single type-2 parameter). In bundled format, the function
// iterates inside the type-2 parameter to filter individual capability TLVs.
func applyCapabilityOverrides(open []byte, overrides []CapabilityOverride) []byte {
	if len(open) < 29 { // 19 header + 10 min body (version+AS+hold+id+optlen)
		return open
	}

	body := open[19:]
	optParamLen := int(body[9])

	// Build set of codes to drop.
	dropCodes := make(map[byte]bool)
	for _, o := range overrides {
		if !o.Add {
			dropCodes[o.Code] = true
		}
	}

	// Iterate optional parameters. For type-2 (Capability), iterate inside the
	// parameter to filter individual capability TLVs (handles bundled format).
	var keptParams []byte
	pos := 10
	for pos+2 <= len(body) && pos < 10+optParamLen {
		paramType := body[pos]
		paramLen := int(body[pos+1])
		if pos+2+paramLen > len(body) {
			break
		}

		if paramType == 2 && paramLen >= 2 {
			// Iterate capability TLVs within this type-2 parameter.
			var keptCaps []byte
			capPos := 0
			paramData := body[pos+2 : pos+2+paramLen]
			for capPos+2 <= len(paramData) {
				capCode := paramData[capPos]
				capLen := int(paramData[capPos+1])
				if capPos+2+capLen > len(paramData) {
					break
				}
				if !dropCodes[capCode] {
					keptCaps = append(keptCaps, paramData[capPos:capPos+2+capLen]...)
				}
				capPos += 2 + capLen
			}
			if len(keptCaps) > 0 {
				keptParams = append(keptParams, 2, byte(len(keptCaps)))
				keptParams = append(keptParams, keptCaps...)
			}
		} else {
			keptParams = append(keptParams, body[pos:pos+2+paramLen]...)
		}
		pos += 2 + paramLen
	}

	// Add new capabilities as a separate type-2 parameter.
	for _, o := range overrides {
		if !o.Add {
			continue
		}

		capTLV := make([]byte, 2+len(o.Value))
		capTLV[0] = o.Code
		capTLV[1] = byte(len(o.Value))
		copy(capTLV[2:], o.Value)

		param := make([]byte, 2+len(capTLV))
		param[0] = 2 // Optional Parameter Type: Capability
		param[1] = byte(len(capTLV))
		copy(param[2:], capTLV)

		keptParams = append(keptParams, param...)
	}

	// Rebuild OPEN message: header + fixed body (9 bytes) + opt param len + params.
	result := make([]byte, 19+10+len(keptParams))
	copy(result, open[:19])     // BGP header (marker + length + type)
	copy(result[19:], body[:9]) // Version, ASN, Hold Time, Router ID
	result[19+9] = byte(len(keptParams))
	copy(result[29:], keptParams)

	// Fix message length in header.
	binary.BigEndian.PutUint16(result[16:], uint16(len(result))) //nolint:gosec // bounded by BGP message size
	return result
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
		if decoded, err := decode.DecodeMessageBytes(fullMsg); err == nil {
			for line := range strings.SplitSeq(decoded.String(), "\n") {
				if line != "" {
					p.printf("             %s\n", line)
				}
			}
		}
	}
}
