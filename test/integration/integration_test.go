// Package integration provides integration tests for ZeBGP.
//
// These tests verify BGP session establishment and message exchange
// between two ZeBGP instances.
package integration

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/reactor"
)

// testTimeout is the maximum time for test operations.
const testTimeout = 10 * time.Second

// findFreePort returns an available TCP port.
func findFreePort(t *testing.T) uint16 {
	t.Helper()
	var lc net.ListenConfig
	l, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("find free port: %v", err)
	}
	defer func() { _ = l.Close() }()
	addr, ok := l.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatal("not a TCP address")
	}
	return uint16(addr.Port) //nolint:gosec // Port is always < 65536
}

// peerConfig holds configuration for a test peer.
type peerConfig struct {
	localAS    uint32
	peerAS     uint32
	routerID   uint32
	connection reactor.ConnectionMode
}

// cleanupReactors stops both reactors and waits for full cleanup.
// This prevents port reuse races between tests.
func cleanupReactors(t *testing.T, r1, r2 *reactor.Reactor) {
	t.Helper()
	r1.Stop()
	r2.Stop()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = r1.Wait(ctx)
	_ = r2.Wait(ctx)
}

// listenPort returns the TCP port the reactor is listening on.
func listenPort(t *testing.T, r *reactor.Reactor) uint16 {
	t.Helper()
	addrs := r.ListenAddrs()
	if len(addrs) == 0 {
		t.Fatal("reactor has no listen addresses")
	}
	tcpAddr, ok := addrs[0].(*net.TCPAddr)
	if !ok {
		t.Fatalf("listen address is not TCP: %T", addrs[0])
	}
	return uint16(tcpAddr.Port) //nolint:gosec // Port is always < 65536
}

// setupPeers creates two reactors with configured neighbors.
// Reactors bind to port 0 (OS-assigned) and peers are added after start
// to eliminate the TOCTOU race between port discovery and binding.
func setupPeers(t *testing.T, ctx context.Context, cfg1, cfg2 peerConfig) (*reactor.Reactor, *reactor.Reactor) {
	t.Helper()

	r1 := reactor.New(&reactor.Config{
		ListenAddr: "127.0.0.1:0",
		RouterID:   cfg1.routerID,
		LocalAS:    cfg1.localAS,
	})

	r2 := reactor.New(&reactor.Config{
		ListenAddr: "127.0.0.1:0",
		RouterID:   cfg2.routerID,
		LocalAS:    cfg2.localAS,
	})

	// Start both reactors first so they bind to OS-assigned ports.
	if err := r1.StartWithContext(ctx); err != nil {
		t.Fatalf("start r1: %v", err)
	}

	if err := r2.StartWithContext(ctx); err != nil {
		r1.Stop()
		t.Fatalf("start r2: %v", err)
	}

	// Extract actual ports from bound listeners.
	port1 := listenPort(t, r1)
	port2 := listenPort(t, r2)

	// Add peers using actual bound ports -- no TOCTOU race.
	neighbor1 := &reactor.PeerSettings{
		Address:    netip.MustParseAddr("127.0.0.1"),
		Port:       port2,
		LocalAS:    cfg1.localAS,
		PeerAS:     cfg1.peerAS,
		RouterID:   cfg1.routerID,
		HoldTime:   30 * time.Second,
		Connection: cfg1.connection,
	}

	neighbor2 := &reactor.PeerSettings{
		Address:    netip.MustParseAddr("127.0.0.1"),
		Port:       port1,
		LocalAS:    cfg2.localAS,
		PeerAS:     cfg2.peerAS,
		RouterID:   cfg2.routerID,
		HoldTime:   30 * time.Second,
		Connection: cfg2.connection,
	}

	if err := r1.AddPeer(neighbor1); err != nil {
		cleanupReactors(t, r1, r2)
		t.Fatalf("add peer to r1: %v", err)
	}
	if err := r2.AddPeer(neighbor2); err != nil {
		cleanupReactors(t, r1, r2)
		t.Fatalf("add peer to r2: %v", err)
	}

	return r1, r2
}

// waitForEstablished waits for any peer in the reactor to reach established state.
func waitForEstablished(ctx context.Context, t *testing.T, r *reactor.Reactor) bool {
	t.Helper()

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
			for _, p := range r.Peers() {
				if p.State() == reactor.PeerStateEstablished {
					return true
				}
			}
		}
	}
}

// TestSessionEstablishment verifies that two ZeBGP peers can establish a session.
//
// VALIDATES: Two peers with active+passive roles reach Established state.
// PREVENTS: Regression in BGP session handshake (OPEN/KEEPALIVE exchange).
func TestSessionEstablishment(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	r1, r2 := setupPeers(t, ctx,
		peerConfig{localAS: 65001, peerAS: 65002, routerID: 0x01010101, connection: reactor.ConnectionBoth},
		peerConfig{localAS: 65002, peerAS: 65001, routerID: 0x02020202, connection: reactor.ConnectionPassive},
	)
	defer cleanupReactors(t, r1, r2)

	if !waitForEstablished(ctx, t, r1) {
		t.Fatal("peer 1 did not reach Established state")
	}

	if !waitForEstablished(ctx, t, r2) {
		t.Fatal("peer 2 did not reach Established state")
	}

	t.Log("✅ Session established between peers")
}

// TestSessionActiveActive verifies two active peers can establish a session.
// NOTE: This requires BGP collision detection (RFC 4271 Section 6.8) which
// is not yet implemented. Skipping for now.
//
// VALIDATES: Active-active peers establish session via collision detection.
// PREVENTS: Dual-active connections causing deadlock or crash.
func TestSessionActiveActive(t *testing.T) {
	t.Skip("collision detection not implemented")
}

// TestSessionIBGP verifies iBGP session (same AS).
//
// VALIDATES: iBGP peers (same AS number) establish session successfully.
// PREVENTS: iBGP session rejection due to same-AS check error.
func TestSessionIBGP(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	r1, r2 := setupPeers(t, ctx,
		peerConfig{localAS: 65001, peerAS: 65001, routerID: 0x01010101, connection: reactor.ConnectionBoth},
		peerConfig{localAS: 65001, peerAS: 65001, routerID: 0x02020202, connection: reactor.ConnectionPassive},
	)
	defer cleanupReactors(t, r1, r2)

	if !waitForEstablished(ctx, t, r1) {
		t.Fatal("iBGP session not established")
	}

	t.Log("✅ iBGP session established")
}

// TestSession4ByteAS verifies session with 4-byte AS numbers.
//
// VALIDATES: 4-byte AS numbers negotiate and establish correctly.
// PREVENTS: ASN4 capability negotiation failure for large AS numbers.
func TestSession4ByteAS(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	r1, r2 := setupPeers(t, ctx,
		peerConfig{localAS: 4200000001, peerAS: 4200000002, routerID: 0x01010101, connection: reactor.ConnectionBoth},
		peerConfig{localAS: 4200000002, peerAS: 4200000001, routerID: 0x02020202, connection: reactor.ConnectionPassive},
	)
	defer cleanupReactors(t, r1, r2)

	if !waitForEstablished(ctx, t, r1) {
		t.Fatal("4-byte AS session not established")
	}

	t.Log("✅ 4-byte AS session established")
}

// TestSessionReconnect verifies reconnection after disconnect.
//
// VALIDATES: Peer reconnects and establishes after delayed peer startup.
// PREVENTS: Permanent connection failure after initial connect attempt fails.
func TestSessionReconnect(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	port1 := findFreePort(t)
	port2 := findFreePort(t)

	r1 := reactor.New(&reactor.Config{
		ListenAddr: fmt.Sprintf("127.0.0.1:%d", port1),
		RouterID:   0x01010101,
		LocalAS:    65001,
	})

	r2 := reactor.New(&reactor.Config{
		ListenAddr: fmt.Sprintf("127.0.0.1:%d", port2),
		RouterID:   0x02020202,
		LocalAS:    65002,
	})

	neighbor1 := &reactor.PeerSettings{
		Address:    netip.MustParseAddr("127.0.0.1"),
		Port:       port2,
		LocalAS:    65001,
		PeerAS:     65002,
		RouterID:   0x01010101,
		HoldTime:   30 * time.Second,
		Connection: reactor.ConnectionBoth,
	}

	neighbor2 := &reactor.PeerSettings{
		Address:    netip.MustParseAddr("127.0.0.1"),
		Port:       port1,
		LocalAS:    65002,
		PeerAS:     65001,
		RouterID:   0x02020202,
		HoldTime:   30 * time.Second,
		Connection: reactor.ConnectionPassive,
	}

	if err := r1.AddPeer(neighbor1); err != nil {
		t.Fatalf("add peer to r1: %v", err)
	}
	if err := r2.AddPeer(neighbor2); err != nil {
		t.Fatalf("add peer to r2: %v", err)
	}

	// Start r1 first.
	if err := r1.StartWithContext(ctx); err != nil {
		t.Fatalf("start r1: %v", err)
	}
	defer cleanupReactors(t, r1, r2)

	// Wait a bit, then start r2 (r1 should reconnect).
	time.Sleep(500 * time.Millisecond)

	if err := r2.StartWithContext(ctx); err != nil {
		t.Fatalf("start r2: %v", err)
	}

	// Should establish after reconnect.
	if !waitForEstablished(ctx, t, r1) {
		t.Fatal("session not established after reconnect")
	}

	t.Log("✅ Session established after reconnect")
}
