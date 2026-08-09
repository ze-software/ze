// Design: docs/architecture/ldp/mpls-ldp.md -- dynamic interface reload (AC-9) tests
package ldp

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/slogutil"
)

// trackingStart records each started interface and signals when its context is
// canceled, so the test can assert start/stop without real network I/O.
type trackingStart struct {
	mu       sync.Mutex
	done     map[string]chan struct{}
	startedC chan string
}

func newTrackingStart() *trackingStart {
	return &trackingStart{done: make(map[string]chan struct{}), startedC: make(chan string, 16)}
}

func (s *trackingStart) start(ctx context.Context, ifName string, _ ldpConfig) {
	s.mu.Lock()
	ch := make(chan struct{})
	s.done[ifName] = ch
	s.mu.Unlock()
	s.startedC <- ifName
	<-ctx.Done()
	close(ch)
}

// waitStarted blocks until n discovery goroutines have recorded themselves.
func (s *trackingStart) waitStarted(t *testing.T, n int) {
	t.Helper()
	for range n {
		select {
		case <-s.startedC:
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for discovery to start")
		}
	}
}

func (s *trackingStart) waitStopped(t *testing.T, ifName string) {
	t.Helper()
	s.mu.Lock()
	ch := s.done[ifName]
	s.mu.Unlock()
	require.NotNil(t, ch, "interface %s was never started", ifName)
	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for discovery on %s to stop", ifName)
	}
}

func TestDiscoveryManagerReconcileStartsAndStops(t *testing.T) {
	// VALIDATES: AC-9 -- a config reload that adds/removes an LDP interface starts
	// or stops discovery on it without restarting the engine.
	ts := newTrackingStart()
	mgr := newDiscoveryManager(context.Background(), slogutil.DiscardLogger(), ts.start)

	mgr.reconcile(ldpConfig{Interfaces: []string{"eth0", "eth1"}})
	assert.Equal(t, 2, mgr.runningCount(), "both configured interfaces start discovery")
	ts.waitStarted(t, 2)

	// Reload: drop eth1, add eth2. eth0 keeps running, eth1 stops, eth2 starts.
	mgr.reconcile(ldpConfig{Interfaces: []string{"eth0", "eth2"}})
	assert.Equal(t, 2, mgr.runningCount())
	ts.waitStarted(t, 1) // eth2
	ts.waitStopped(t, "eth1")

	// Re-applying the same set is idempotent (no duplicate discovery goroutines).
	mgr.reconcile(ldpConfig{Interfaces: []string{"eth0", "eth2"}})
	assert.Equal(t, 2, mgr.runningCount())

	mgr.stopAll()
	assert.Equal(t, 0, mgr.runningCount())
	ts.waitStopped(t, "eth0")
	ts.waitStopped(t, "eth2")
}

func TestDiscoveryManagerNoInterfacesFallsBack(t *testing.T) {
	// VALIDATES: with no interfaces configured a single system-default listener
	// runs (the prior single-socket behavior), keyed by the empty interface name.
	ts := newTrackingStart()
	mgr := newDiscoveryManager(context.Background(), slogutil.DiscardLogger(), ts.start)
	mgr.reconcile(ldpConfig{})
	assert.Equal(t, 1, mgr.runningCount())
	ts.waitStarted(t, 1)
	mgr.stopAll()
}
