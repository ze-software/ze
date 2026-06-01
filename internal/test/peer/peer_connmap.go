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

func (p *Peer) acceptConnMapBatch(ctx context.Context, ln net.Listener, batchSize int) ([]connWithID, Result, bool) {
	conns := make([]connWithID, batchSize)
	var wg sync.WaitGroup
	var acceptErr error
	var errOnce sync.Once

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
		return nil, Result{Success: false, Error: acceptErr}, true
	}
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
