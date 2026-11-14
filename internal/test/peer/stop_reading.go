// Design: docs/architecture/testing/ci-format.md -- stop-reading peer option
// Related: peer.go -- handleConnection and runMessageLoop, the two callers
// Related: reject.go -- endSequence, the linger hold this one mirrors without reading

package peer

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"syscall"
	"time"
)

// keyStopReadingConn is the key naming the connection option=stop-reading
// applies to.
const keyStopReadingConn = "conn"

// stopReadingReceiveBufferOctets is the receive buffer the peer asks the kernel
// for on the connection it stops reading. A kernel rounds the request up to its
// own floor, which is still a few kilobytes. Setting it also switches receive
// buffer autotuning off for the socket, so the number of bytes the daemon can
// park on the peer's side no longer depends on the host's default sizes.
const stopReadingReceiveBufferOctets = 4096

// stopReadingKeepaliveInterval is how often the peer writes a KEEPALIVE on a
// connection it stopped reading. The smallest hold time a session can
// negotiate is three seconds (RFC 4271 Section 4.2), so one KEEPALIVE a second
// keeps every legal receive hold timer from firing.
const stopReadingKeepaliveInterval = time.Second

// errUnreadHeldToTeardown is what a peer answers when the test ended while it
// still held a connection it had stopped reading. The daemon was expected to
// drop that session, and it never did.
var errUnreadHeldToTeardown = errors.New(
	"the daemon never closed the connection the peer stopped reading, so the session it could not write to never ended (option=stop-reading)")

// StopReadingDecl is option=stop-reading: the one connection on which the peer
// stops reading once the daemon's first UPDATE has arrived.
type StopReadingDecl struct {
	// Conn is the connection the option applies to, numbered from 1 like the
	// conn= of an expect=bgp line. Every other connection is read normally.
	Conn int
}

// parsePeerOptionStopReading reads option=stop-reading. A value that is not a
// boolean, or a conn= that names no connection, is refused: either would run a
// session other than the one the .ci describes.
func parsePeerOptionStopReading(config *Config, kv map[string]string) error {
	enabled, err := strconv.ParseBool(kv[keyValue])
	if err != nil {
		return fmt.Errorf("option=stop-reading:value=%q is not a boolean; "+
			"write value=true to make the peer stop reading after the daemon's first UPDATE", kv[keyValue])
	}
	if !enabled {
		config.StopReading = nil
		return nil
	}
	decl := &StopReadingDecl{Conn: 1}
	if text, declared := kv[keyStopReadingConn]; declared {
		conn, err := strconv.Atoi(text)
		if err != nil {
			return fmt.Errorf("option=stop-reading:conn=%q is not a connection number; "+
				"write conn=<N>, counted from 1 like the conn= of an expect=bgp line", text)
		}
		if conn < 1 {
			return fmt.Errorf("option=stop-reading:conn=%d names no connection; "+
				"connections are counted from 1, so write conn=1 for the first", conn)
		}
		decl.Conn = conn
	}
	config.StopReading = decl
	return nil
}

// validate refuses a peer configuration the option cannot run under. The
// option belongs to a check peer that serves its connections one at a time
// and tells the remote's close from its own writes.
func (d *StopReadingDecl) validate(config *Config) error {
	if config.Mode != ModeCheck {
		return fmt.Errorf("option=stop-reading requires check mode, not %q: "+
			"sink and echo peers hold no expectation that says which UPDATE is the first", config.Mode)
	}
	if config.ConnMap != "" {
		return errors.New("option=stop-reading cannot be combined with conn_map: " +
			"a conn_map peer orders a batch of connections by identity, so conn=<N> names no accept order; " +
			"remove one of the two options")
	}
	if config.Silent {
		return errors.New("option=stop-reading cannot be combined with silent: " +
			"stop-reading keeps writing KEEPALIVEs so the daemon's receive hold timer never fires, and silent stops them; " +
			"remove one of the two options")
	}
	if config.RemoteClose {
		return errors.New("option=stop-reading cannot be combined with remote_close: " +
			"remote_close reads to observe the remote's end, and stop-reading reads nothing; " +
			"remove one of the two options")
	}
	served := max(config.TCPConnections, 1)
	if d.Conn > served {
		return fmt.Errorf("option=stop-reading:conn=%d names a connection the peer never serves: "+
			"it serves %d; raise option=tcp_connections:value=<N> or lower conn=", d.Conn, served)
	}
	return nil
}

// stopsReading reports whether the connection in hand is the one
// option=stop-reading names.
func (p *Peer) stopsReading() bool {
	decl := p.config.StopReading
	return decl != nil && int(p.connections.Load()) == decl.Conn
}

// shrinkReceiveBuffer asks the kernel for a small receive buffer on conn, so a
// modest number of routes fills it once the peer stops reading. A connection
// that is not TCP (the in-memory pipe of a unit test) holds no kernel buffer to
// shrink, and its writer already blocks on the first unread byte.
func shrinkReceiveBuffer(conn net.Conn) error {
	if tcp, ok := conn.(*net.TCPConn); ok {
		if err := tcp.SetReadBuffer(stopReadingReceiveBufferOctets); err != nil {
			return fmt.Errorf("option=stop-reading: shrink the receive buffer to %d octets: %w",
				stopReadingReceiveBufferOctets, err)
		}
	}
	return nil
}

// stopReading is what runMessageLoop returns once the daemon's first UPDATE has
// been judged on the connection option=stop-reading names. That UPDATE MUST
// meet the last expectation the connection holds: the peer reads nothing after
// it, so an expectation left behind could never be met.
func (p *Peer) stopReading(ctx context.Context, conn net.Conn) Result {
	if !p.checker.sequenceEnded() {
		if !p.checker.Completed() {
			return Result{Success: false, Error: fmt.Errorf(
				"option=stop-reading: the peer reads nothing on conn=%d after the daemon's first UPDATE, "+
					"so that UPDATE must meet the last expectation the connection holds, but %s; "+
					"give conn=%d one expectation the first UPDATE meets and move the rest to a later connection",
				p.config.StopReading.Conn, p.checker.unmet(), p.config.StopReading.Conn)}
		}
	}
	p.printf("\nreading nothing more on this connection, and sending a KEEPALIVE every %s "+
		"until the remote closes it (option=stop-reading)\n", stopReadingKeepaliveInterval)
	result := p.holdUnread(ctx, conn, stopReadingKeepaliveInterval)
	if result.Success {
		p.printf("\nthe remote closed the connection the peer had stopped reading\n")
	}
	return result
}

// holdUnread keeps conn open without reading it, writing one KEEPALIVE every
// interval, until the remote closes the connection or the test ends.
//
// The remote's close is observed on the WRITE side, and only there. A read
// would defeat the stall, and a peek cannot see the remote's FIN behind the
// bytes the peer left unread. A KEEPALIVE written to a connection the remote
// closed is answered with a reset, and the write after it fails, so the close
// is seen within two intervals.
//
// A write that times out says the remote stopped reading as well, never that
// it closed, so the loop carries the unwritten remainder of that KEEPALIVE
// into the next write: a frame cut short and followed by a fresh one would
// desynchronise the stream the daemon parses.
//
// The loop ends with ctx, which the test's teardown cancels, or with the
// remote's close.
func (p *Peer) holdUnread(ctx context.Context, conn net.Conn, interval time.Duration) Result {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var unwritten []byte
	for {
		select {
		case <-ctx.Done():
			return Result{Success: false, Error: errUnreadHeldToTeardown}
		case <-ticker.C:
		}
		if len(unwritten) == 0 {
			unwritten = KeepaliveMsg()
		}
		if err := conn.SetWriteDeadline(time.Now().Add(interval)); err != nil {
			return Result{Success: false, Error: fmt.Errorf("option=stop-reading: set the KEEPALIVE write deadline: %w", err)}
		}
		written, err := conn.Write(unwritten)
		unwritten = unwritten[written:]
		if err == nil {
			continue
		}
		if isTimeout(err) {
			continue
		}
		if remoteClosedWrite(err) {
			return Result{Success: true}
		}
		return Result{Success: false, Error: fmt.Errorf("option=stop-reading: write KEEPALIVE: %w", err)}
	}
}

// remoteClosedWrite reports whether a write failed because the remote had
// closed the connection: a reset, or a broken pipe once the reset was seen.
func remoteClosedWrite(err error) bool {
	if isConnReset(err) {
		return true
	}
	return errors.Is(err, syscall.EPIPE)
}
