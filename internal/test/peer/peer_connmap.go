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

	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", addr)
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
