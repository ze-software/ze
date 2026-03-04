package integration

import (
	"context"
	"net"
	"net/netip"
	"strings"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/capability"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/fsm"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/reactor"
)

// TestDirectSession tests session establishment without reactor.
func TestDirectSession(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create listener.
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()

	addr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatal("not a TCP address")
	}
	port := uint16(addr.Port) //nolint:gosec // Port is always < 65536
	t.Logf("Listening on port %d", port)

	// Neighbor configs.
	neighbor1 := &reactor.PeerSettings{
		Address:    netip.MustParseAddr("127.0.0.1"),
		Port:       port,
		LocalAS:    65001,
		PeerAS:     65002,
		RouterID:   0x01010101,
		HoldTime:   30 * time.Second,
		Connection: reactor.ConnectionBoth,
	}

	neighbor2 := &reactor.PeerSettings{
		Address:    netip.MustParseAddr("127.0.0.1"),
		Port:       0, // Not used for passive
		LocalAS:    65002,
		PeerAS:     65001,
		RouterID:   0x02020202,
		HoldTime:   30 * time.Second,
		Connection: reactor.ConnectionPassive,
	}

	// Create sessions.
	session1 := reactor.NewSession(neighbor1)
	session2 := reactor.NewSession(neighbor2)

	// Start FSMs.
	if err := session1.Start(); err != nil {
		t.Fatalf("start session1: %v", err)
	}
	if err := session2.Start(); err != nil {
		t.Fatalf("start session2: %v", err)
	}

	// Accept connection in goroutine.
	acceptDone := make(chan error, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			acceptDone <- err
			return
		}
		t.Logf("Accepted connection from %s", conn.RemoteAddr())
		if err := session2.Accept(conn); err != nil {
			acceptDone <- err
			return
		}
		acceptDone <- nil
	}()

	// Connect session1.
	if err := session1.Connect(ctx); err != nil {
		t.Fatalf("connect session1: %v", err)
	}
	t.Logf("Session1 connected, state: %s", session1.State())

	// Wait for accept.
	select {
	case err := <-acceptDone:
		if err != nil {
			t.Fatalf("accept: %v", err)
		}
	case <-ctx.Done():
		t.Fatalf("timeout waiting for accept")
	}
	t.Logf("Session2 accepted, state: %s", session2.State())

	// Run both sessions in goroutines.
	errChan := make(chan error, 2)
	go func() {
		errChan <- session1.Run(ctx)
	}()
	go func() {
		errChan <- session2.Run(ctx)
	}()

	// Wait for established state.
	established := make(chan struct{})
	go func() {
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s1 := session1.State()
				s2 := session2.State()
				t.Logf("States: session1=%s, session2=%s", s1, s2)
				if s1 == fsm.StateEstablished && s2 == fsm.StateEstablished {
					close(established)
					return
				}
			}
		}
	}()

	select {
	case <-established:
		t.Log("✅ Both sessions established")
	case err := <-errChan:
		t.Fatalf("session error: %v", err)
	case <-ctx.Done():
		t.Fatalf("timeout: session1=%s, session2=%s", session1.State(), session2.State())
	}
}

// TestRequiredFamilyRejection tests that a session is rejected when a required
// address family is not supported by the peer.
//
// Scenario:
//   - Session1 requires ipv6/unicast.
//   - Session2 only advertises ipv4/unicast.
//   - Session1 should send NOTIFICATION (2/7) and reject the session.
func TestRequiredFamilyRejection(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create listener.
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()

	addr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatal("not a TCP address")
	}
	port := uint16(addr.Port) //nolint:gosec // Port is always < 65536
	t.Logf("Listening on port %d", port)

	// Session1: requires ipv6/unicast (will be the one rejecting)
	neighbor1 := &reactor.PeerSettings{
		Address:    netip.MustParseAddr("127.0.0.1"),
		Port:       port,
		LocalAS:    65001,
		PeerAS:     65002,
		RouterID:   0x01010101,
		HoldTime:   30 * time.Second,
		Connection: reactor.ConnectionBoth,
		Capabilities: []capability.Capability{
			&capability.Multiprotocol{AFI: capability.AFIIPv6, SAFI: capability.SAFIUnicast},
		},
		RequiredFamilies: []capability.Family{
			{AFI: capability.AFIIPv6, SAFI: capability.SAFIUnicast},
		},
	}

	// Session2: only advertises ipv4/unicast (doesn't have what session1 requires)
	neighbor2 := &reactor.PeerSettings{
		Address:    netip.MustParseAddr("127.0.0.1"),
		Port:       0,
		LocalAS:    65002,
		PeerAS:     65001,
		RouterID:   0x02020202,
		HoldTime:   30 * time.Second,
		Connection: reactor.ConnectionPassive,
		Capabilities: []capability.Capability{
			&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
		},
	}

	// Create sessions.
	session1 := reactor.NewSession(neighbor1)
	session2 := reactor.NewSession(neighbor2)

	// Start FSMs.
	if err := session1.Start(); err != nil {
		t.Fatalf("start session1: %v", err)
	}
	if err := session2.Start(); err != nil {
		t.Fatalf("start session2: %v", err)
	}

	// Accept connection in goroutine.
	acceptDone := make(chan error, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			acceptDone <- err
			return
		}
		t.Logf("Accepted connection from %s", conn.RemoteAddr())
		if err := session2.Accept(conn); err != nil {
			acceptDone <- err
			return
		}
		acceptDone <- nil
	}()

	// Connect session1.
	if err := session1.Connect(ctx); err != nil {
		t.Fatalf("connect session1: %v", err)
	}
	t.Logf("Session1 connected, state: %s", session1.State())

	// Wait for accept.
	select {
	case err := <-acceptDone:
		if err != nil {
			t.Fatalf("accept: %v", err)
		}
	case <-ctx.Done():
		t.Fatalf("timeout waiting for accept")
	}
	t.Logf("Session2 accepted, state: %s", session2.State())

	// Run both sessions - session1 should fail with "required families not negotiated"
	errChan1 := make(chan error, 1)
	errChan2 := make(chan error, 1)
	go func() {
		errChan1 <- session1.Run(ctx)
	}()
	go func() {
		errChan2 <- session2.Run(ctx)
	}()

	// Wait for session1 to fail (it should reject due to missing required family)
	select {
	case err := <-errChan1:
		if err == nil {
			t.Fatal("expected session1 to fail, but got nil error")
		}
		if !strings.Contains(err.Error(), "required families not negotiated") {
			t.Fatalf("expected 'required families not negotiated' error, got: %v", err)
		}
		t.Logf("✅ Session1 correctly rejected: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatalf("timeout waiting for session1 rejection, state=%s", session1.State())
	}

	// Session2 should also fail (received NOTIFICATION from session1)
	select {
	case err := <-errChan2:
		t.Logf("✅ Session2 terminated: %v", err)
	case <-time.After(1 * time.Second):
		// May have already exited, that's fine
		t.Log("Session2 may have already exited")
	}
}

// TestOpenNegotiation verifies end-to-end OPEN negotiation over real TCP.
//
// VALIDATES: Two sessions exchange OPEN messages with specific, non-default
// capabilities and both arrive at the correct negotiated result:
//   - Hold time = min(60s, 90s) = 60s
//   - ASN4 = true (both advertise)
//   - Families = intersection (ipv4/unicast only — ipv6/unicast is local-only)
//   - Route Refresh = false (only one side advertises)
//
// PREVENTS: Capability negotiation bugs hiding behind tests that only check
// "did we reach Established?" without verifying the negotiated parameters.
func TestOpenNegotiation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create listener.
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()

	addr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatal("not a TCP address")
	}
	port := uint16(addr.Port) //nolint:gosec // Port is always < 65536

	// Session1: active, HoldTime=60s, ASN4 + ipv4/unicast + ipv6/unicast + RouteRefresh
	neighbor1 := &reactor.PeerSettings{
		Address:    netip.MustParseAddr("127.0.0.1"),
		Port:       port,
		LocalAS:    65001,
		PeerAS:     65002,
		RouterID:   0x01010101,
		HoldTime:   60 * time.Second,
		Connection: reactor.ConnectionBoth,
		Capabilities: []capability.Capability{
			&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
			&capability.Multiprotocol{AFI: capability.AFIIPv6, SAFI: capability.SAFIUnicast},
			&capability.RouteRefresh{},
		},
	}

	// Session2: passive, HoldTime=90s, ASN4 + ipv4/unicast only (no RouteRefresh)
	neighbor2 := &reactor.PeerSettings{
		Address:    netip.MustParseAddr("127.0.0.1"),
		Port:       0,
		LocalAS:    65002,
		PeerAS:     65001,
		RouterID:   0x02020202,
		HoldTime:   90 * time.Second,
		Connection: reactor.ConnectionPassive,
		Capabilities: []capability.Capability{
			&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
		},
	}

	session1 := reactor.NewSession(neighbor1)
	session2 := reactor.NewSession(neighbor2)

	if err := session1.Start(); err != nil {
		t.Fatalf("start session1: %v", err)
	}
	if err := session2.Start(); err != nil {
		t.Fatalf("start session2: %v", err)
	}

	// Accept connection in goroutine.
	acceptDone := make(chan error, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			acceptDone <- err
			return
		}
		if err := session2.Accept(conn); err != nil {
			acceptDone <- err
			return
		}
		acceptDone <- nil
	}()

	// Connect session1.
	if err := session1.Connect(ctx); err != nil {
		t.Fatalf("connect session1: %v", err)
	}

	// Wait for accept.
	select {
	case err := <-acceptDone:
		if err != nil {
			t.Fatalf("accept: %v", err)
		}
	case <-ctx.Done():
		t.Fatalf("timeout waiting for accept")
	}

	// Run both sessions.
	errChan := make(chan error, 2)
	go func() { errChan <- session1.Run(ctx) }()
	go func() { errChan <- session2.Run(ctx) }()

	// Wait for both to reach Established.
	established := make(chan struct{})
	go func() {
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if session1.State() == fsm.StateEstablished && session2.State() == fsm.StateEstablished {
					close(established)
					return
				}
			}
		}
	}()

	select {
	case <-established:
	case err := <-errChan:
		t.Fatalf("session error before established: %v", err)
	case <-ctx.Done():
		t.Fatalf("timeout: session1=%s, session2=%s", session1.State(), session2.State())
	}

	// Verify negotiated capabilities on BOTH sides.
	neg1 := session1.Negotiated()
	neg2 := session2.Negotiated()

	if neg1 == nil {
		t.Fatal("session1 Negotiated() is nil after Established")
	}
	if neg2 == nil {
		t.Fatal("session2 Negotiated() is nil after Established")
	}

	// RFC 6793: ASN4 — both advertise (implicit via DisableASN4=false default).
	if !neg1.ASN4 {
		t.Error("session1: expected ASN4=true")
	}
	if !neg2.ASN4 {
		t.Error("session2: expected ASN4=true")
	}

	// RFC 4271 Section 4.2: Hold time = min(60, 90) = 60.
	if neg1.HoldTime != 60 {
		t.Errorf("session1: expected HoldTime=60, got %d", neg1.HoldTime)
	}
	if neg2.HoldTime != 60 {
		t.Errorf("session2: expected HoldTime=60, got %d", neg2.HoldTime)
	}

	// RFC 4760: Family intersection — only ipv4/unicast (both advertise).
	// Session1 also advertises ipv6/unicast, but session2 does not.
	ipv4uni := capability.Family{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast}
	ipv6uni := capability.Family{AFI: capability.AFIIPv6, SAFI: capability.SAFIUnicast}

	if !neg1.SupportsFamily(ipv4uni) {
		t.Error("session1: expected ipv4/unicast negotiated")
	}
	if neg1.SupportsFamily(ipv6uni) {
		t.Error("session1: ipv6/unicast should NOT be negotiated (peer lacks it)")
	}

	if !neg2.SupportsFamily(ipv4uni) {
		t.Error("session2: expected ipv4/unicast negotiated")
	}
	if neg2.SupportsFamily(ipv6uni) {
		t.Error("session2: ipv6/unicast should NOT be negotiated")
	}

	// RFC 2918: Route Refresh — session1 advertises, session2 does not → false.
	if neg1.RouteRefresh {
		t.Error("session1: expected RouteRefresh=false (peer lacks it)")
	}
	if neg2.RouteRefresh {
		t.Error("session2: expected RouteRefresh=false (local lacks it)")
	}

	// Mismatches: ipv6/unicast and RouteRefresh should appear.
	if len(neg1.Mismatches) == 0 {
		t.Error("session1: expected mismatches for ipv6/unicast and RouteRefresh")
	}

	t.Log("✅ Both sessions established with correct negotiated capabilities")
}
