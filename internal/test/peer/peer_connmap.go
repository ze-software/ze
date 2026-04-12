// Design: docs/architecture/testing/ci-format.md -- router-id based connection mapping
// Overview: peer.go -- Peer.Run dispatches here when ConnMap == "router-id"
//
// When option=conn_map:value=router-id is set, ze-peer accepts all expected
// connections concurrently, completes the OPEN handshake on each, extracts
// the router-id, and sorts connections by router-id (lowest = conn=1). This
// gives deterministic conn= assignment regardless of TCP accept order,
// enabling multi-peer forwarding tests where action=send targets a specific
// peer and expect=bgp verifies what another peer receives.
package peer

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sort"
	"sync"
)

// connWithID pairs a TCP connection with the router-id from its OPEN.
type connWithID struct {
	conn     net.Conn
	routerID uint32
}

// runConnMapRouterID accepts N connections, handshakes each, sorts by
// router-id, and processes expect/send rules sequentially per connection.
func (p *Peer) runConnMapRouterID(ctx context.Context) Result {
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

	maxConns := p.config.TCPConnections
	if maxConns <= 0 {
		maxConns = 1
	}

	// Phase 1: Accept all connections and complete OPEN handshake concurrently.
	conns := make([]connWithID, maxConns)
	var wg sync.WaitGroup
	var acceptErr error
	var errOnce sync.Once

	for i := range maxConns {
		conn, aErr := ln.Accept()
		if aErr != nil {
			select {
			case <-ctx.Done():
				return Result{Success: true}
			default:
				return Result{Success: false, Error: fmt.Errorf("accept: %w", aErr)}
			}
		}
		if tcpConn, ok := conn.(*net.TCPConn); ok {
			if err := tcpConn.SetNoDelay(true); err != nil {
				p.printf("set nodelay: %v\n", err)
			}
		}

		wg.Add(1)
		go func(idx int, c net.Conn) {
			defer wg.Done()
			p.printf("\nnew connection from %s\n", c.RemoteAddr())
			_, _, rid, hErr := p.doOpenHandshake(c)
			if hErr != nil {
				errOnce.Do(func() { acceptErr = hErr })
				c.Close() //nolint:errcheck // cleanup on handshake failure
				return
			}
			conns[idx] = connWithID{conn: c, routerID: rid}
		}(i, conn)
	}
	wg.Wait()

	if acceptErr != nil {
		for _, c := range conns {
			if c.conn != nil {
				c.conn.Close() //nolint:errcheck // cleanup
			}
		}
		return Result{Success: false, Error: acceptErr}
	}

	// Phase 2: Sort by router-id (lowest = conn=1).
	sort.Slice(conns, func(i, j int) bool { return conns[i].routerID < conns[j].routerID })

	for i, c := range conns {
		p.printf("\nconn=%d router-id=%d.%d.%d.%d\n", i+1,
			(c.routerID>>24)&0xFF, (c.routerID>>16)&0xFF,
			(c.routerID>>8)&0xFF, c.routerID&0xFF)
	}

	// Close all connections when done, regardless of which path returns.
	defer func() {
		for _, c := range conns {
			if c.conn != nil {
				c.conn.Close() //nolint:errcheck // cleanup
			}
		}
	}()

	// Phase 3: Process connections sequentially. The checker's SequenceEnded()
	// signals when the current connection's rules are exhausted, so conn=1's
	// loop returns after its send actions complete. Conn=2's loop then starts
	// and reads the forwarded message. All TCP connections stay alive (deferred
	// close) so ze can forward between them.
	for i, c := range conns {
		ok := p.checker.Init()
		p.printf("\nphase3: conn=%d router-id=%08X init=%v\n", i+1, c.routerID, ok)
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
	return Result{Success: false, Error: errors.New("not all expectations met after all connections")}
}
