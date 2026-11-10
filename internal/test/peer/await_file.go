// Design: docs/architecture/testing/ci-format.md -- option=await-file
// Related: peer.go -- Config.AwaitFile, runMessageLoop calls awaitFile before any send
// Related: established_file_test.go -- option=established-file, the file another peer block writes

package peer

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"time"
)

// awaitFileMax is the default bound of the option=await-file wait, used when
// the option declares no timeout= of its own. It covers the daemon bringing up
// a second session under a loaded verify run, and it stays well inside the 20s
// timeout a .ci peer block normally declares, so a missing file fails with
// this package's message rather than as a bare test timeout. A wait on a
// slower writer (an interop checker that first reads two other daemons'
// sessions) declares its own bound rather than raising this one for every
// test.
const awaitFileMax = 15 * time.Second

// awaitFilePoll is the interval between two checks for the awaited file. It is
// also the read deadline of each read made while waiting.
const awaitFilePoll = 20 * time.Millisecond

// awaitFileHeldMax bounds the octets of non-KEEPALIVE frames a wait holds for
// the script. Past it the wait stops reading, and TCP holds the rest.
const awaitFileHeldMax = 1 << 20

// awaitFileBound is the bound of this peer's wait: the option's timeout= when
// it declared one, awaitFileMax otherwise. A zero Config.AwaitFileBound is the
// undeclared bound, never a zero-length wait: parsePeerOptionAwaitFile refuses
// a declared bound that is not positive.
func (p *Peer) awaitFileBound() time.Duration {
	if p.config.AwaitFileBound > 0 {
		return p.config.AwaitFileBound
	}
	return awaitFileMax
}

// awaitFile blocks until Config.AwaitFile exists. It returns conn unchanged
// and nil at once when no file was declared or the file is already there, and
// an error naming the file when the bound passes or ctx ends first.
//
// The session stays alive while it waits. Each KEEPALIVE the daemon sends is
// answered and consumed, exactly as the check-mode loop answers and silently
// accepts one, so the daemon's receive hold timer never expires under a wait
// longer than the hold time. RFC 4271 Section 4.4: "Instead, KEEPALIVE
// messages are exchanged between peers often enough not to cause the Hold
// Timer to expire." A script that waits on a file therefore never sees a
// KEEPALIVE that arrived before the file existed. Every other frame (an
// End-of-RIB, a forwarded UPDATE) is held, in arrival order, and the returned
// connection replays the held frames, then any partial frame read behind
// them, to the reader that comes after, which answers and judges each one as
// it always did. A read error, or held frames reaching awaitFileHeldMax,
// stops the service, and the reader after the wait meets the rest of the
// stream itself.
//
// The caller MUST use the returned connection for every later read.
func (p *Peer) awaitFile(ctx context.Context, conn net.Conn) (net.Conn, error) {
	if p.config.AwaitFile == "" {
		return conn, nil
	}
	bound := p.awaitFileBound()
	deadline := time.Now().Add(bound)
	reader := newMessageReader(conn)
	serving := true
	var held []byte
	for {
		_, err := os.Stat(p.config.AwaitFile)
		if err == nil {
			p.printf("\nawaited %s, sending\n", p.config.AwaitFile)
			return replayHeld(conn, append(held, reader.partial()...)), nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return conn, fmt.Errorf("await-file %q: %w", p.config.AwaitFile, err)
		}
		if time.Now().After(deadline) {
			return conn, fmt.Errorf("await-file %q did not appear within %s: its writer never wrote it. When a "+
				"peer block declares option=established-file:path=%s, that session never reached Established; "+
				"when the test's observer writes it, read the observer's ZE-OBSERVER-FAIL line for why it stopped",
				p.config.AwaitFile, bound, p.config.AwaitFile)
		}
		select {
		case <-ctx.Done():
			return conn, fmt.Errorf("await-file %q: %w", p.config.AwaitFile, ctx.Err())
		default:
		}
		if !serving {
			select {
			case <-ctx.Done():
				return conn, fmt.Errorf("await-file %q: %w", p.config.AwaitFile, ctx.Err())
			case <-time.After(awaitFilePoll):
			}
			continue
		}
		_ = conn.SetReadDeadline(time.Now().Add(awaitFilePoll))
		header, body, err := reader.readMessage()
		if err != nil {
			if isTimeout(err) {
				continue
			}
			// The reader after the wait meets this closed connection itself and
			// decides, as it always did, whether a close was expected.
			serving = false
			continue
		}
		if header[18] == MsgKEEPALIVE {
			p.replyKeepalive(conn)
			continue
		}
		held = append(append(held, header...), body...)
		if len(held) >= awaitFileHeldMax {
			serving = false
		}
	}
}

// replayHeld returns conn unchanged when nothing was read past the KEEPALIVEs
// the wait consumed, and otherwise a connection whose first reads return held.
func replayHeld(conn net.Conn, held []byte) net.Conn {
	_ = conn.SetReadDeadline(time.Time{})
	if len(held) == 0 {
		return conn
	}
	return &replayConn{Conn: conn, held: held}
}

// replayConn is a connection whose Read returns held before it reads the
// network again. It carries the frames awaitFile read and did not consume to
// the readers that run after the wait. It is not safe for concurrent use; a
// peer reads one connection from one goroutine.
type replayConn struct {
	net.Conn
	held []byte
}

// Read returns the held bytes first, then the network's.
func (c *replayConn) Read(b []byte) (int, error) {
	if len(c.held) > 0 {
		n := copy(b, c.held)
		c.held = c.held[n:]
		return n, nil
	}
	return c.Conn.Read(b)
}
