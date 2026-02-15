package peer

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/netip"
	"os"
	"time"

	"codeberg.org/thomas-mangin/ze/cmd/ze-bgp-chaos/scenario"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/message"
)

// SimProfile holds the peer identity and route parameters for a simulator.
// It mirrors the fields from scenario.PeerProfile needed at runtime.
type SimProfile struct {
	Index      int
	ASN        uint32
	RouterID   netip.Addr
	IsIBGP     bool
	HoldTime   uint16
	RouteCount int
}

// SimulatorConfig holds all parameters for running a single peer simulator.
type SimulatorConfig struct {
	// Profile is the peer's identity and route parameters.
	Profile SimProfile

	// Seed is the scenario seed for deterministic route generation.
	Seed uint64

	// Addr is the TCP address to connect to (host:port).
	Addr string

	// Events is the channel to send lifecycle and route events on.
	Events chan<- Event

	// Verbose enables extra debug output.
	Verbose bool

	// Quiet suppresses non-error output.
	Quiet bool
}

// RunSimulator runs a single BGP peer simulator. It connects to Ze, performs
// the OPEN/KEEPALIVE handshake, sends routes, reads incoming messages, and
// maintains the KEEPALIVE loop. All lifecycle events are reported via cfg.Events.
//
// RunSimulator blocks until ctx is cancelled or a fatal error occurs.
func RunSimulator(ctx context.Context, cfg SimulatorConfig) {
	p := cfg.Profile

	emit := func(ev Event) {
		ev.PeerIndex = p.Index
		if ev.Time.IsZero() {
			ev.Time = time.Now()
		}
		// Try non-blocking send first (succeeds when channel has buffer space).
		// Fall back to blocking select with cancellation.
		select {
		case cfg.Events <- ev:
			return
		default:
		}
		select {
		case cfg.Events <- ev:
		case <-ctx.Done():
		}
	}

	// Connect to Ze.
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", cfg.Addr)
	if err != nil {
		if ctx.Err() != nil {
			emit(Event{Type: EventDisconnected})
			return
		}
		emit(Event{Type: EventError, Err: fmt.Errorf("connecting to %s: %w", cfg.Addr, err)})
		return
	}
	defer func() { _ = conn.Close() }()

	// OPEN exchange.
	open := BuildOpen(SessionConfig{
		ASN:      p.ASN,
		RouterID: p.RouterID,
		HoldTime: p.HoldTime,
	})
	if writeErr := writeMsg(conn, open); writeErr != nil {
		emit(Event{Type: EventError, Err: fmt.Errorf("sending OPEN: %w", writeErr)})
		return
	}

	if readErr := readMsg(conn); readErr != nil {
		emit(Event{Type: EventError, Err: fmt.Errorf("reading OPEN: %w", readErr)})
		return
	}

	// KEEPALIVE exchange.
	if writeErr := writeMsg(conn, message.NewKeepalive()); writeErr != nil {
		emit(Event{Type: EventError, Err: fmt.Errorf("sending KEEPALIVE: %w", writeErr)})
		return
	}

	if readErr := readMsg(conn); readErr != nil {
		emit(Event{Type: EventError, Err: fmt.Errorf("reading KEEPALIVE: %w", readErr)})
		return
	}

	emit(Event{Type: EventEstablished})

	if cfg.Verbose {
		fmt.Fprintf(os.Stderr, "ze-bgp-chaos | peer %d | session established\n", p.Index)
	}

	// Send routes.
	routes := scenario.GenerateIPv4Routes(cfg.Seed, p.Index, p.RouteCount)
	sender := NewSender(SenderConfig{
		ASN:     p.ASN,
		IsIBGP:  p.IsIBGP,
		NextHop: p.RouterID,
	})

	for _, prefix := range routes {
		select {
		case <-ctx.Done():
			sendCease(conn, p.Index, cfg.Quiet)
			emit(Event{Type: EventDisconnected})
			return
		default:
		}

		data := sender.BuildRoute(prefix)
		if _, writeErr := conn.Write(data); writeErr != nil {
			emit(Event{Type: EventError, Err: fmt.Errorf("sending UPDATE: %w", writeErr)})
			return
		}
		emit(Event{Type: EventRouteSent, Prefix: prefix})
	}

	// Send End-of-RIB.
	eor := BuildEORIPv4Unicast()
	if _, writeErr := conn.Write(eor); writeErr != nil {
		emit(Event{Type: EventError, Err: fmt.Errorf("sending EOR: %w", writeErr)})
		return
	}
	emit(Event{Type: EventEORSent, Count: len(routes)})

	// Start reader goroutine for incoming messages from RR.
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		readLoop(ctx, conn, p.Index, cfg.Events)
	}()

	// KEEPALIVE loop.
	keepaliveInterval := time.Duration(p.HoldTime/3) * time.Second
	ticker := time.NewTicker(keepaliveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			sendCease(conn, p.Index, cfg.Quiet)
			conn.Close() //nolint:errcheck,gosec // best-effort close to unblock readLoop
			<-readerDone
			emit(Event{Type: EventDisconnected})
			return
		case <-readerDone:
			// Reader closed — connection lost.
			emit(Event{Type: EventDisconnected})
			return
		case <-ticker.C:
			if writeErr := writeMsg(conn, message.NewKeepalive()); writeErr != nil {
				conn.Close() //nolint:errcheck,gosec // best-effort close to unblock readLoop
				<-readerDone
				if ctx.Err() != nil {
					emit(Event{Type: EventDisconnected})
					return
				}
				emit(Event{Type: EventError, Err: fmt.Errorf("sending KEEPALIVE: %w", writeErr)})
				return
			}
		}
	}
}

// readLoop reads BGP messages from conn and emits route events.
// It runs until the connection closes or ctx is cancelled.
func readLoop(ctx context.Context, conn net.Conn, peerIndex int, events chan<- Event) {
	for {
		if ctx.Err() != nil {
			return
		}

		header := make([]byte, message.HeaderLen)
		if _, err := io.ReadFull(conn, header); err != nil {
			return // Connection closed.
		}

		msgLen := int(binary.BigEndian.Uint16(header[16:18]))
		if msgLen < message.HeaderLen {
			return
		}

		var body []byte
		if msgLen > message.HeaderLen {
			body = make([]byte, msgLen-message.HeaderLen)
			if _, err := io.ReadFull(conn, body); err != nil {
				return
			}
		}

		if len(header) < 19 {
			return
		}
		msgType := header[18]
		if msgType != 2 { // Not UPDATE — skip (KEEPALIVE, etc.)
			continue
		}

		// Parse IPv4/unicast UPDATE for announced and withdrawn prefixes.
		parseUpdatePrefixes(body, peerIndex, events, ctx)
	}
}

// parseUpdatePrefixes extracts IPv4/unicast announced and withdrawn prefixes
// from an UPDATE message body (after the 19-byte header).
func parseUpdatePrefixes(body []byte, peerIndex int, events chan<- Event, ctx context.Context) {
	if len(body) < 4 {
		return
	}

	// Withdrawn routes length (2 bytes).
	withdrawnLen := int(binary.BigEndian.Uint16(body[0:2]))
	off := 2

	// Parse withdrawn prefixes.
	end := off + withdrawnLen
	if end > len(body) {
		return
	}
	for off < end {
		prefix, n := parseIPv4Prefix(body[off:end])
		if n <= 0 {
			break
		}
		off += n
		select {
		case events <- Event{Type: EventRouteWithdrawn, PeerIndex: peerIndex, Time: time.Now(), Prefix: prefix}:
		case <-ctx.Done():
			return
		}
	}

	// Total path attribute length (2 bytes).
	if off+2 > len(body) {
		return
	}
	attrLen := int(binary.BigEndian.Uint16(body[off : off+2]))
	off += 2

	// Skip attributes.
	off += attrLen

	// Parse NLRI (announced prefixes).
	for off < len(body) {
		prefix, n := parseIPv4Prefix(body[off:])
		if n <= 0 {
			break
		}
		off += n
		select {
		case events <- Event{Type: EventRouteReceived, PeerIndex: peerIndex, Time: time.Now(), Prefix: prefix}:
		case <-ctx.Done():
			return
		}
	}
}

// parseIPv4Prefix parses a single IPv4 prefix from wire format.
// Returns the prefix and the number of bytes consumed, or 0 on error.
func parseIPv4Prefix(data []byte) (netip.Prefix, int) {
	if len(data) < 1 {
		return netip.Prefix{}, 0
	}

	prefixLen := int(data[0])
	if prefixLen > 32 {
		return netip.Prefix{}, 0
	}

	byteLen := (prefixLen + 7) / 8
	if len(data) < 1+byteLen {
		return netip.Prefix{}, 0
	}

	var addr [4]byte
	copy(addr[:], data[1:1+byteLen])
	prefix := netip.PrefixFrom(netip.AddrFrom4(addr), prefixLen)

	return prefix, 1 + byteLen
}

// writeMsg serializes and sends a BGP message on a connection.
func writeMsg(conn net.Conn, msg message.Message) error {
	data := SerializeMessage(msg)
	_, err := conn.Write(data)
	return err
}

// readMsg reads and discards a single BGP message from the connection.
func readMsg(conn net.Conn) error {
	header := make([]byte, message.HeaderLen)
	if _, err := io.ReadFull(conn, header); err != nil {
		return fmt.Errorf("reading header: %w", err)
	}

	msgLen := int(binary.BigEndian.Uint16(header[16:18]))
	if msgLen < message.HeaderLen {
		return fmt.Errorf("invalid message length: %d", msgLen)
	}

	if msgLen > message.HeaderLen {
		body := make([]byte, msgLen-message.HeaderLen)
		if _, err := io.ReadFull(conn, body); err != nil {
			return fmt.Errorf("reading body: %w", err)
		}
	}

	return nil
}

// sendCease sends a NOTIFICATION Cease (best-effort on shutdown).
func sendCease(conn net.Conn, peerIndex int, quiet bool) {
	notif := BuildCeaseNotification()
	_ = writeMsg(conn, notif)

	if !quiet {
		fmt.Fprintf(os.Stderr, "ze-bgp-chaos | peer %d | sent NOTIFICATION cease\n", peerIndex)
	}
}
