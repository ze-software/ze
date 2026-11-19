// Design: docs/architecture/api/process-protocol.md -- original physical read gate.
package reactor

import (
	"context"
	"github.com/stretchr/testify/require"
	"net/netip"
	"sync"
	"testing"
	"time"
)

// Same-address original owners and physical successors remain separate. These
// are actual Session read gates, not bookkeeping hooks or a fake controller.
// MUTATION: resolve by address and call Peer.pauseReading/resumeReading.
func TestPhysicalPeerReadControlKeepsOriginalOwnerAndConnection(t *testing.T) {
	addr := netip.MustParseAddr("192.0.2.249")
	first := NewPeer(NewPeerSettings(addr, 65000, 65001, 0x01010101))
	sibling := NewPeer(NewPeerSettings(addr, 65000, 65002, 0x02020202))
	first.session = NewSession(first.Settings())
	sibling.session = NewSession(sibling.Settings())
	original := first.session
	r := &Reactor{peers: map[netip.AddrPort]*Peer{netip.AddrPortFrom(addr, 179): first, netip.AddrPortFrom(addr, 180): sibling}}
	ctx := t.Context()
	require.NoError(t, r.PausePeer(ctx, addr, first.sourceOwner, original.connectionID))
	require.True(t, original.IsPaused())
	require.False(t, sibling.session.IsPaused())
	require.NoError(t, r.PausePeer(ctx, addr, sibling.sourceOwner, sibling.session.connectionID))
	require.True(t, sibling.session.IsPaused())
	require.NoError(t, r.ResumePeer(ctx, addr, first.sourceOwner, original.connectionID))
	require.False(t, original.IsPaused())
	require.True(t, sibling.session.IsPaused())
	first.mu.Lock()
	fresh := NewSession(first.settings)
	first.session = fresh
	first.mu.Unlock()
	require.Error(t, r.PausePeer(ctx, addr, first.sourceOwner, original.connectionID))
	require.False(t, fresh.IsPaused(), "old physical pause touched the successor")
	require.NoError(t, r.PausePeer(ctx, addr, first.sourceOwner, fresh.connectionID))
	require.Error(t, r.ResumePeer(ctx, addr, first.sourceOwner, original.connectionID))
	require.True(t, fresh.IsPaused(), "old physical resume released the successor")
	require.NoError(t, r.ResumePeer(ctx, addr, first.sourceOwner, fresh.connectionID))
	require.Error(t, r.PausePeer(ctx, addr, 0, original.connectionID))
	require.Error(t, r.PausePeer(ctx, addr, first.sourceOwner, 0))
	canceled, stop := context.WithCancel(ctx)
	stop()
	require.ErrorIs(t, r.PausePeer(canceled, addr, first.sourceOwner, fresh.connectionID), context.Canceled)
	require.False(t, fresh.IsPaused())
	r.mu.Lock()
	delete(r.peers, netip.AddrPortFrom(addr, 179))
	r.mu.Unlock()
	require.Error(t, r.ResumePeer(ctx, addr, first.sourceOwner, fresh.connectionID))
	require.True(t, sibling.session.IsPaused(), "retired owner borrowed same-address sibling")
	require.NoError(t, r.ResumePeer(ctx, addr, sibling.sourceOwner, sibling.session.connectionID))
	require.False(t, sibling.session.IsPaused())
	// Explicit operator scope remains available on the existing lifecycle API.
	require.NoError(t, r.PausePeer(ctx, addr, 0, 0))
	require.True(t, sibling.session.IsPaused())
	require.NoError(t, r.ResumePeer(ctx, addr, 0, 0))
	require.False(t, sibling.session.IsPaused())
}

type physicalReadContext struct {
	context.Context
	checked chan struct{}
	once    sync.Once
}

func (c *physicalReadContext) Err() error {
	err := c.Context.Err()
	c.once.Do(func() { close(c.checked) })
	return err
}

// An expired/canceled caller waiting on the real Session read-gate mutex must
// join that accepted call and refuse before mutating the original read gate.
// A peer lock alone is insufficient because the Session owns a second mutex.
func TestPhysicalPeerReadControlCancellationAtFinalReadGate(t *testing.T) {
	addr := netip.MustParseAddr("192.0.2.250")
	peer := NewPeer(NewPeerSettings(addr, 65000, 65001, 0x01010101))
	peer.session = NewSession(peer.Settings())
	session := peer.session
	r := &Reactor{peers: map[netip.AddrPort]*Peer{netip.AddrPortFrom(addr, 179): peer}}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	captured := &physicalReadContext{Context: ctx, checked: make(chan struct{})}
	session.pauseMu.Lock()
	locked := true
	done := make(chan error, 1)
	joined := false
	t.Cleanup(func() {
		cancel()
		if locked {
			session.pauseMu.Unlock()
		}
		if joined {
			return
		}
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("physical read-gate caller did not join")
		}
	})
	go func() { done <- r.PausePeer(captured, addr, peer.sourceOwner, session.connectionID) }()
	// This positive callback means the real outer guard captured a live Err
	// result under the Peer lock. Cancellation follows that already-decided
	// outer guard, so only the final Session guard can refuse mutation.
	select {
	case <-captured.checked:
	case <-time.After(5 * time.Second):
		t.Fatal("actual controller never checked its original Peer context")
	}
	cancel()
	select {
	case err := <-done:
		t.Fatalf("physical call returned before its accepted final gate joined: %v", err)
	default:
	}
	locked = false
	session.pauseMu.Unlock()
	select {
	case err := <-done:
		joined = true
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(5 * time.Second):
		t.Fatal("canceled physical read-gate call never joined")
	}
	require.False(t, session.IsPaused(), "canceled call mutated the final read gate")
}
