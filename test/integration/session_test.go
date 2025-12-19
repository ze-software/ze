package integration

import (
	"context"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/exa-networks/zebgp/pkg/bgp/fsm"
	"github.com/exa-networks/zebgp/pkg/reactor"
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
	neighbor1 := &reactor.Neighbor{
		Address:  netip.MustParseAddr("127.0.0.1"),
		Port:     port,
		LocalAS:  65001,
		PeerAS:   65002,
		RouterID: 0x01010101,
		HoldTime: 30 * time.Second,
		Passive:  false,
	}

	neighbor2 := &reactor.Neighbor{
		Address:  netip.MustParseAddr("127.0.0.1"),
		Port:     0, // Not used for passive
		LocalAS:  65002,
		PeerAS:   65001,
		RouterID: 0x02020202,
		HoldTime: 30 * time.Second,
		Passive:  true,
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
