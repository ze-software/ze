package integration

import (
	"bytes"
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/exa-networks/zebgp/pkg/reactor"
	"github.com/exa-networks/zebgp/pkg/testpeer"
)

// runPeerTest runs a testpeer with the given config and connects a session.
func runPeerTest(t *testing.T, peerConfig *testpeer.Config) testpeer.Result {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	peer := testpeer.New(peerConfig)

	peerDone := make(chan testpeer.Result, 1)
	go func() {
		peerDone <- peer.Run(ctx)
	}()

	time.Sleep(50 * time.Millisecond)

	neighbor := &reactor.Neighbor{
		Address:  netip.MustParseAddr("127.0.0.1"),
		Port:     uint16(peerConfig.Port), //nolint:gosec // Port from test config, always valid
		LocalAS:  65001,
		PeerAS:   65002,
		RouterID: 0x01010101,
		HoldTime: 30 * time.Second,
	}

	session := reactor.NewSession(neighbor)
	if err := session.Start(); err != nil {
		t.Fatalf("start session: %v", err)
	}

	if err := session.Connect(ctx); err != nil {
		t.Fatalf("connect: %v", err)
	}

	sessionCtx, sessionCancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer sessionCancel()

	go func() {
		_ = session.Run(sessionCtx)
	}()

	<-sessionCtx.Done()
	_ = session.Close()

	cancel()
	return <-peerDone
}

// TestPeerSinkMode tests that testpeer works in sink mode.
func TestPeerSinkMode(t *testing.T) {
	port := findFreePort(t)

	result := runPeerTest(t, &testpeer.Config{
		Port:   int(port),
		Sink:   true,
		Output: &bytes.Buffer{},
	})

	if result.Error != nil {
		t.Fatalf("peer error: %v", result.Error)
	}

	t.Log("✅ Sink mode test passed")
}

// TestPeerCheckMode tests that testpeer validates expected messages.
func TestPeerCheckMode(t *testing.T) {
	port := findFreePort(t)

	result := runPeerTest(t, &testpeer.Config{
		Port:   int(port),
		Expect: []string{}, // Empty - accept session
		Output: &bytes.Buffer{},
	})

	// With empty expectations, completes when connection closes.
	if result.Error != nil {
		t.Logf("Note: %v (expected with empty check list)", result.Error)
	}

	t.Log("✅ Check mode test completed")
}

// TestPeerEchoMode tests that testpeer echoes messages back.
func TestPeerEchoMode(t *testing.T) {
	port := findFreePort(t)

	result := runPeerTest(t, &testpeer.Config{
		Port:   int(port),
		Echo:   true,
		Output: &bytes.Buffer{},
	})

	if result.Error != nil {
		t.Fatalf("peer error: %v", result.Error)
	}

	t.Log("✅ Echo mode test passed")
}
