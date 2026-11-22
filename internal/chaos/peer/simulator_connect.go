// Design: docs/architecture/chaos-web-dashboard.md — BGP peer simulation
// Overview: simulator.go — the peer simulator, which starts its session here
// Related: simulator_reader.go — writeMsg and readMsg, the handshake's I/O

package peer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
)

// The bounds of the connection retry. A simulated speaker stands for a router
// in a test, so the wait is short: the one known cause of an early close is a
// peer that Ze removes and adds again within a few milliseconds while it
// rebuilds its peers before starting them. Four waits of 50, 100, 200 and 400
// ms, 750 ms in all, cover that window many times over, and a Ze that refuses
// the speaker for good is reported in under a second.
const (
	// connectAttemptsMax is the number of connections a simulator opens for one
	// session before it gives up, the first one included.
	connectAttemptsMax = 5

	// connectBackoffMin is the wait before the second attempt. It doubles
	// after each attempt, up to connectBackoffMax.
	connectBackoffMin = 50 * time.Millisecond

	// connectBackoffMax is the longest wait between two attempts.
	connectBackoffMax = 400 * time.Millisecond

	// handshakeDeadline bounds each read of the OPEN and KEEPALIVE exchange, so
	// a peer that accepts the connection and never answers ends the run.
	handshakeDeadline = 10 * time.Second
)

// errClosedBeforeOpen marks a connection the peer closed, or refused, before
// the simulator read any byte of the peer's OPEN. It is the only failure
// establishSession retries.
var errClosedBeforeOpen = errors.New("the peer closed the connection before its OPEN")

// establishSession answers a connection on which OPEN and KEEPALIVE were
// exchanged, and true. Otherwise it has emitted the event that ends the run,
// EventError or EventDisconnected, and answers false. The caller owns the
// connection it gets and MUST close it.
//
// A connection the peer closes before its OPEN is a failed connection attempt,
// and the simulator makes another one, as a BGP speaker does: at most
// connectAttemptsMax connections, with a wait between them that doubles from
// connectBackoffMin to connectBackoffMax. Each retry is reported as an
// EventReconnecting carrying the cause in Err and the number of the attempt
// that failed in Count. Every other failure ends the run as before: a read
// that reaches handshakeDeadline, a close after the peer's OPEN began to
// arrive, a NOTIFICATION. A retry would hide a peer that accepted the session
// and then failed it, or one that never answers.
//
// The first attempt uses cfg.Conn when the caller gave one. A retry dials
// cfg.Addr, so a caller that gave a connection and no address gets no retry:
// there is nothing to dial.
//
// RFC 4271 Section 8.2.2, OpenSent state: "If a TcpConnectionFails event
// (Event 18) is received, the local system: - closes the BGP connection, -
// restarts the ConnectRetryTimer, - continues to listen for a connection that
// may be initiated by the remote BGP peer, and - changes its state to Active."
//
// RFC 4271 Section 8.2.2, Active state: "In response to a
// ConnectRetryTimer_Expires event (Event 9), the local system: - restarts the
// ConnectRetryTimer (with initial value), - initiates a TCP connection to the
// other BGP peer, - continues to listen for a TCP connection that may be
// initiated by a remote BGP peer, and - changes its state to Connect."
//
// RFC 4271 Section 8.2.2: "The exact value of the ConnectRetryTimer is a local
// matter, but it SHOULD be sufficiently large to allow TCP initialization."
func establishSession(ctx context.Context, cfg SimulatorConfig, emit func(Event)) (net.Conn, bool) {
	conn := cfg.Conn
	backoff := connectBackoffMin
	var lastErr error
	for attempt := 1; attempt <= connectAttemptsMax; attempt++ {
		var err error
		if conn == nil {
			conn, err = dialPeer(ctx, cfg)
			if err != nil {
				if ctx.Err() != nil {
					emit(Event{Type: EventDisconnected})
					return nil, false
				}
				if !errors.Is(err, syscall.ECONNREFUSED) {
					emit(Event{Type: EventError, Err: fmt.Errorf("connecting to %s: %w", cfg.Addr, err)})
					return nil, false
				}
				err = fmt.Errorf("%w: connecting: %w", errClosedBeforeOpen, err)
			}
		}
		if err == nil {
			err = exchangeOpen(ctx, cfg, conn)
			if err == nil {
				return conn, true
			}
			conn.Close() //nolint:errcheck,gosec // closing the connection whose handshake failed
			if ctx.Err() != nil {
				emit(Event{Type: EventError, Err: err})
				return nil, false
			}
			if !errors.Is(err, errClosedBeforeOpen) {
				emit(Event{Type: EventError, Err: err})
				return nil, false
			}
			if cfg.Addr == "" {
				// The caller's own connection, and no address to dial again.
				emit(Event{Type: EventError, Err: err})
				return nil, false
			}
		}
		// A failed attempt leaves no connection: the dial answered none, or
		// the one it answered was closed above.
		conn = nil
		lastErr = err

		if attempt == connectAttemptsMax {
			break
		}
		emit(Event{Type: EventReconnecting, Count: attempt, Err: lastErr})
		select {
		case <-time.After(backoff):
		case <-ctx.Done():
			emit(Event{Type: EventDisconnected})
			return nil, false
		}
		backoff = min(backoff*2, connectBackoffMax)
	}
	emit(Event{Type: EventError, Err: fmt.Errorf("connecting to %s: gave up after %d attempts, the last: %w",
		cfg.Addr, connectAttemptsMax, lastErr)})
	return nil, false
}

// dialPeer opens a TCP connection to cfg.Addr, from cfg.LocalAddr when the
// caller named one.
func dialPeer(ctx context.Context, cfg SimulatorConfig) (net.Conn, error) {
	d := net.Dialer{}
	if cfg.LocalAddr != "" {
		d.LocalAddr = &net.TCPAddr{IP: net.ParseIP(cfg.LocalAddr)}
	}
	return d.DialContext(ctx, "tcp", cfg.Addr)
}

// exchangeOpen sends the simulator's OPEN, reads the peer's, then exchanges
// KEEPALIVE. An error wraps errClosedBeforeOpen when the peer closed the
// connection before any byte of its OPEN was read, and only then. It leaves
// the connection open: the caller closes it on an error.
func exchangeOpen(ctx context.Context, cfg SimulatorConfig, conn net.Conn) error {
	// Close the connection when ctx is canceled: the reads below block on
	// io.ReadFull, and nothing else can unblock them.
	stop := context.AfterFunc(ctx, func() {
		conn.Close() //nolint:errcheck,gosec // best-effort close to unblock handshake reads
	})
	defer stop()

	// A read deadline, so that a peer that accepts TCP and never sends its
	// OPEN does not hold the run forever. In in-process mode (Clock != nil)
	// there is none: the close above already ends the reads on cancellation,
	// and a real-time deadline can expire during runner.Run()'s initial sleep,
	// causing spurious handshake failures at scale.
	if cfg.Clock == nil {
		if err := conn.SetReadDeadline(time.Now().Add(handshakeDeadline)); err != nil {
			return fmt.Errorf("setting handshake deadline: %w", err)
		}
	}

	p := cfg.Profile
	open, err := BuildOpen(SessionConfig{
		ASN:      p.ASN,
		RouterID: p.RouterID,
		HoldTime: p.HoldTime,
		Families: p.Families,
	})
	if err != nil {
		return fmt.Errorf("building OPEN: %w", err)
	}
	if writeErr := writeMsg(conn, open); writeErr != nil {
		if closedByPeer(writeErr) {
			return fmt.Errorf("%w: sending OPEN: %w", errClosedBeforeOpen, writeErr)
		}
		return fmt.Errorf("sending OPEN: %w", writeErr)
	}

	peerOpen := &arrivalReader{source: conn}
	if readErr := readMsg(peerOpen, msgtype.TypeOPEN); readErr != nil {
		if peerOpen.arrived {
			return fmt.Errorf("reading OPEN: %w", readErr)
		}
		if closedByPeer(readErr) {
			return fmt.Errorf("%w: reading OPEN: %w", errClosedBeforeOpen, readErr)
		}
		return fmt.Errorf("reading OPEN: %w", readErr)
	}

	if writeErr := writeMsg(conn, message.NewKeepalive()); writeErr != nil {
		return fmt.Errorf("sending KEEPALIVE: %w", writeErr)
	}
	if readErr := readMsg(conn, msgtype.TypeKEEPALIVE); readErr != nil {
		return fmt.Errorf("reading KEEPALIVE: %w", readErr)
	}

	// Clear read deadline for the session phase (keepalive loop handles its own).
	if err := conn.SetReadDeadline(time.Time{}); err != nil {
		return fmt.Errorf("clearing read deadline: %w", err)
	}
	return nil
}

// closedByPeer answers whether err says the peer ended the connection: an end
// of stream, a reset, or a write on a connection the peer already closed. A
// deadline that passed and a connection this side closed are neither.
func closedByPeer(err error) bool {
	if errors.Is(err, io.EOF) {
		return true
	}
	if errors.Is(err, syscall.ECONNRESET) {
		return true
	}
	return errors.Is(err, syscall.EPIPE)
}

// arrivalReader reads from source and records whether any byte arrived, which
// tells a connection closed before the peer's OPEN from one closed inside it.
// Not safe for concurrent use.
type arrivalReader struct {
	source  io.Reader
	arrived bool
}

func (r *arrivalReader) Read(buf []byte) (int, error) {
	n, err := r.source.Read(buf)
	if n > 0 {
		r.arrived = true
	}
	return n, err
}
