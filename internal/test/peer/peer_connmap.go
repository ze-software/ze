// Design: docs/architecture/testing/ci-format.md -- connection mapping
// Overview: peer.go -- Peer.Run dispatches here when ConnMap is set
//
// Connection mapping accepts a batch of TCP connections, completes OPEN on
// each, sorts the batch by a stable key, then runs the checker's conn=N rules
// in that sorted order. Reload tests can use several batches: after a SIGHUP
// closes the current batch, ze-peer accepts the next batch and continues with
// the remaining conn=N expectations.
package peer

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"slices"
	"strconv"
	"sync"
	"time"
)

const (
	connMapRouterID = "router-id"
	connMapRemoteIP = "remote-ip"
)

// connWithID pairs a TCP connection with the values used for conn_map sorting.
type connWithID struct {
	conn     net.Conn
	routerID uint32
	remoteIP netip.Addr
}

// runConnMap accepts mapped connection batches and processes expect/send rules
// sequentially inside each sorted batch.
func (p *Peer) runConnMap(ctx context.Context) Result {
	host := p.config.BindAddr
	if host == "" {
		host = "127.0.0.1"
		if p.config.IPv6 {
			host = "::1"
		}
	}
	addr := net.JoinHostPort(host, strconv.Itoa(p.config.Port))

	ln, err := p.listen(ctx, addr)
	if err != nil {
		return Result{Success: false, Error: fmt.Errorf("listen: %w", err)}
	}
	defer func() {
		if cErr := ln.Close(); cErr != nil && !errors.Is(cErr, net.ErrClosed) {
			p.printf("listener close: %v\n", cErr)
		}
	}()

	p.printf("listening on %s\n", addr)
	p.readyOnce.Do(func() { close(p.ready) })

	go func() {
		<-ctx.Done()
		ln.Close() //nolint:errcheck // best-effort on context cancel
	}()

	batchSize := p.config.TCPConnections
	if batchSize <= 0 {
		batchSize = 1
	}

	for {
		conns, result, done := p.acceptConnMapBatch(ctx, ln, batchSize)
		if done {
			return result
		}
		sortConnBatch(conns, p.config.ConnMap)
		p.printConnBatch(conns)

		result = p.processConnBatch(ctx, conns)
		// Only when another batch follows: the wait exists to keep the NEXT
		// batch free of connections the daemon opened behind our back.
		if result.Success && !p.checker.Completed() && p.signalFired.Swap(false) {
			p.waitBatchClosed(ctx, conns)
		}
		closeConnBatch(conns)
		if !result.Success {
			return result
		}
		if p.checker.Completed() {
			return Result{Success: true}
		}
		p.printf("\nwaiting for next mapped connection batch (%d)...\n", batchSize)
	}
}

// awaitEOR reads until ze's End-of-RIB arrives, tolerating the KEEPALIVEs that
// normal BGP interleaves and nothing else.
//
// It refuses rather than skips any other frame. Silently swallowing an UPDATE
// here would take it out of the expectation stream, and the test would then fail
// somewhere else entirely -- Checker.consumeMatches runs BEFORE the
// silent-accept arm, so a consumed frame is a frame the test can no longer
// match. A caller that trips this has opted a test into AwaitEOR that should not
// be, and the error says which frame arrived.
func awaitEOR(conn net.Conn) error {
	for {
		header, body, err := ReadMessage(conn)
		if err != nil {
			return fmt.Errorf("read while awaiting end-of-rib: %w", err)
		}
		msg := &Message{Header: header, Body: body}
		if msg.IsEOR() {
			return nil
		}
		if msg.IsKeepalive() {
			continue
		}
		return fmt.Errorf(
			"awaiting end-of-rib: got message type %d before it; option=await_eor may only be set"+
				" on a test that expects no other frame during establishment", msg.Kind())
	}
}

// acceptConnMapBatch accepts batchSize connections and completes the OPEN
// handshake on each concurrently.
//
// Accepted sockets are registered with a watcher that closes them when ctx is
// canceled. Without it a connection that is accepted but never sends an OPEN
// blocks doOpenHandshake forever: its ReadMessage (peer.go:348) sets no
// deadline, and the ln.Close() on cancel in runConnMap only stops NEW accepts.
// The batch then hangs until the runner's outer timeout kills the test, which
// reports a bare timeout and names neither the stuck connection nor the fact
// that a handshake was the thing waiting. Closing the accepted sockets turns
// that silent hang into a named error.
func (p *Peer) acceptConnMapBatch(ctx context.Context, ln net.Listener, batchSize int) ([]connWithID, Result, bool) {
	conns := make([]connWithID, batchSize)
	var wg sync.WaitGroup
	var acceptErr error
	var errOnce sync.Once

	// Sockets accepted so far. The watcher below reads this while the accept
	// loop appends to it, so it is mutex-guarded.
	var mu sync.Mutex
	accepted := make([]net.Conn, 0, batchSize)
	handedOff := false

	// handedOff is the real interlock, not batchDone. Closing batchDone at return
	// does NOT stop the watcher from closing connections we are handing back: the
	// deferred close runs at return, leaving a window after wg.Wait(), and even
	// once it is closed a select whose ctx.Done() is also ready picks uniformly at
	// random (Go spec). Losing that draw would close the very conns returned to
	// processConnBatch, which then fails on "use of closed network connection" --
	// a spurious write failure from the code meant to REMOVE spurious failures.
	// The flag is checked under the same mutex the closes take.
	batchDone := make(chan struct{})
	defer close(batchDone)
	go func() {
		select {
		case <-ctx.Done():
			mu.Lock()
			if !handedOff {
				for _, c := range accepted {
					c.Close() //nolint:errcheck // unblocking a stuck handshake on cancel
				}
			}
			mu.Unlock()
		case <-batchDone:
		}
	}()

	for i := range batchSize {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return nil, Result{Success: true}, true
			default:
				return nil, Result{Success: false, Error: fmt.Errorf("accept: %w", err)}, true
			}
		}
		if tcpConn, ok := conn.(*net.TCPConn); ok {
			if err := tcpConn.SetNoDelay(true); err != nil {
				p.printf("set nodelay: %v\n", err)
			}
		}
		mu.Lock()
		accepted = append(accepted, conn)
		mu.Unlock()

		wg.Add(1)
		go func(idx int, c net.Conn) {
			defer wg.Done()
			remote := c.RemoteAddr()
			p.printf("\nnew connection from %s\n", remote)
			_, _, rid, hErr := p.doOpenHandshake(c)
			if hErr != nil {
				// Name the connection: with a batch of N a bare "read OPEN"
				// says nothing about which peer failed to hand over its OPEN.
				errOnce.Do(func() {
					acceptErr = fmt.Errorf("open handshake on batch slot %d (remote %s): %w", idx+1, remote, hErr)
				})
				c.Close() //nolint:errcheck // cleanup on handshake failure
				return
			}
			// Read ze's KEEPALIVE before handing the batch over. doOpenHandshake
			// only WRITES our OPEN and KEEPALIVE and returns, so without this the
			// batch handed back connections whose sessions ze had not finished
			// bringing up -- and batching exists precisely so a script on one slot
			// does not send before another slot is a live target. Receiving this
			// frame proves ze consumed our OPEN and left OpenSent; our KEEPALIVE
			// is already on the wire, so its FSM reaches Established with no
			// further input from us. Same gate inject mode already uses
			// (inject.go, read peer KEEPALIVE). Safe to consume: the checker
			// skips KEEPALIVE when matching (checker.go), and no conn_map test
			// asserts the establishment KEEPALIVE.
			kaHeader, _, kaErr := ReadMessage(c)
			if kaErr != nil {
				errOnce.Do(func() {
					acceptErr = fmt.Errorf("read KEEPALIVE on batch slot %d (remote %s): %w", idx+1, remote, kaErr)
				})
				c.Close() //nolint:errcheck // cleanup on handshake failure
				return
			}
			if kaHeader[18] != MsgKEEPALIVE {
				errOnce.Do(func() {
					acceptErr = fmt.Errorf("batch slot %d (remote %s): expected KEEPALIVE after OPEN, got type %d", idx+1, remote, kaHeader[18])
				})
				c.Close() //nolint:errcheck // cleanup on handshake failure
				return
			}
			if p.config.AwaitEOR {
				if eErr := awaitEOR(c); eErr != nil {
					errOnce.Do(func() {
						acceptErr = fmt.Errorf("batch slot %d (remote %s): %w", idx+1, remote, eErr)
					})
					c.Close() //nolint:errcheck // cleanup on handshake failure
					return
				}
			}
			conns[idx] = connWithID{
				conn:     c,
				routerID: rid,
				remoteIP: remoteIPFromConn(c),
			}
		}(i, conn)
	}
	wg.Wait()

	if acceptErr != nil {
		closeConnBatch(conns)
		if ctx.Err() != nil {
			// Canceling closed the sockets underneath the handshake, so the
			// I/O error is the teardown's own doing, not a peer failure. Match
			// the accept loop above and let the runner decide the verdict.
			return nil, Result{Success: true}, true
		}
		return nil, Result{Success: false, Error: acceptErr}, true
	}

	// From here the batch belongs to the caller; the watcher must not touch it.
	mu.Lock()
	handedOff = true
	mu.Unlock()
	return conns, Result{}, false
}

// waitBatchClosed blocks until the daemon has closed every connection in the
// batch, or until ctx ends. Called when a sighup/sigterm action fired inside
// this batch and conn=N expectations remain, i.e. another batch is coming.
//
// The peer must never close a session the daemon still holds. Only ONE
// connection in a batch carries the signal action, and its message loop returns
// as soon as THAT connection reaches EOF; the batch's other connections went
// idle earlier, and closeConnBatch would close them right behind it. Whether
// the daemon has torn those down yet is decided by the order it stops its
// peers, and that order is randomized by construction -- reconcilePeersJournaled
// builds its remove set by ranging a Go map
// (internal/component/bgp/reactor/reactor_api.go:508). Close a session ze still
// holds and ze redials it, immediately on a teardown
// (internal/component/bgp/reactor/peer_run.go:78). That extra connection joins
// the next accepted batch, which shifts every conn=N slot, so the sorted batch
// hands the checker another peer's routes: the test then fails as a hex
// mismatch, naming neither the reopened session nor the reload.
//
// Waiting for the closes makes the next batch exact rather than probable. ze
// runs every peer removal before any peer add (the remove loop at
// reactor_api.go:528 completes before the add loop at :557), so once all of this
// batch's sockets are at EOF the only connections that can be pending are the
// new generation's -- however long the daemon took to get there.
func (p *Peer) waitBatchClosed(ctx context.Context, conns []connWithID) {
	p.printf("\nwaiting for the daemon to close all %d connections of this batch\n", len(conns))
	var wg sync.WaitGroup
	for i, c := range conns {
		if c.conn == nil {
			continue
		}
		wg.Add(1)
		go func(slot int, conn net.Conn) {
			defer wg.Done()
			buf := make([]byte, 4096)
			for {
				select {
				case <-ctx.Done():
					p.printf("conn=%d still open when the test ended\n", slot)
					return
				default:
				}
				// Read and discard: the session is going away, so anything
				// still arriving on it (KEEPALIVEs) is not the test's business.
				// The deadline only bounds how often ctx is rechecked.
				_ = conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond)) //nolint:errcheck // deadline on a socket being drained
				if _, err := conn.Read(buf); err != nil {
					if isTimeout(err) {
						continue
					}
					p.printf("conn=%d closed by the daemon\n", slot)
					return
				}
			}
		}(i+1, c.conn)
	}
	wg.Wait()
}

func (p *Peer) processConnBatch(ctx context.Context, conns []connWithID) Result {
	for _, c := range conns {
		p.checker.Init()
		result := p.runMessageLoop(ctx, c.conn)
		if !result.Success {
			return result
		}
		if p.checker.Completed() {
			return Result{Success: true}
		}
	}
	if p.checker.Completed() {
		return Result{Success: true}
	}
	return Result{Success: true}
}

func sortConnBatch(conns []connWithID, mode string) {
	slices.SortFunc(conns, func(a, b connWithID) int {
		switch mode {
		case connMapRemoteIP:
			if cmp := a.remoteIP.Compare(b.remoteIP); cmp != 0 {
				return cmp
			}
			return compareRouterID(a.routerID, b.routerID)
		default:
			if cmp := compareRouterID(a.routerID, b.routerID); cmp != 0 {
				return cmp
			}
			return a.remoteIP.Compare(b.remoteIP)
		}
	})
}

func compareRouterID(a, b uint32) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

func remoteIPFromConn(conn net.Conn) netip.Addr {
	if conn == nil {
		return netip.Addr{}
	}
	if tcpAddr, ok := conn.RemoteAddr().(*net.TCPAddr); ok {
		return tcpAddr.AddrPort().Addr().Unmap()
	}
	addrPort, err := netip.ParseAddrPort(conn.RemoteAddr().String())
	if err != nil {
		return netip.Addr{}
	}
	return addrPort.Addr().Unmap()
}

func (p *Peer) printConnBatch(conns []connWithID) {
	for i, c := range conns {
		p.printf("\nconn=%d remote-ip=%s router-id=%d.%d.%d.%d\n", i+1, c.remoteIP,
			(c.routerID>>24)&0xFF, (c.routerID>>16)&0xFF,
			(c.routerID>>8)&0xFF, c.routerID&0xFF)
	}
}

func closeConnBatch(conns []connWithID) {
	for _, c := range conns {
		if c.conn != nil {
			c.conn.Close() //nolint:errcheck // cleanup
		}
	}
}
