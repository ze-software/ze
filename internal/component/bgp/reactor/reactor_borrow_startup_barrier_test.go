// Design: docs/architecture/api/architecture.md -- the reactor's startup barrier
// Detail: api_sync.go -- SetAPIProcessCount, SignalPluginStartupComplete
// Related: reactor.go -- Reactor.startAPIServer, where the barrier is armed

package reactor

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// VALIDATES: spec-bgp-reactor-startup-race -- a borrow-mode reactor arms no
// startup barrier, so a borrowed server that signals while the reactor starts
// touches nothing the reactor is writing.
//
// METHOD: the borrowed server is already running when the reactor starts, as it
// is under the hub. One goroutine plays the server's startup and session
// goroutines, calling SignalPluginStartupComplete and AddAPIProcessCount in a
// loop for the whole of StartWithContext. Under -race, a start that re-creates
// the barrier fields (SetAPIProcessCount) reports a data race with that loop.
// Without -race, the assertion that start left startupComplete nil is the same
// defect seen as state: borrow mode never waits on the barrier, so arming it
// there serves no reader.
//
// PREVENTS: the race the five live reactor tests reported (TestRFC7705*,
// TestRFC7947AllAttributesReachClient, TestRouteServerTransparencyStopsAtOrdinaryPeer,
// TestRFC9687Event29ReleasesTheLivePeersRIB), reproduced without a plugin.
func TestBorrowModeStartArmsNoStartupBarrier(t *testing.T) {
	r := New(&Config{ListenAddr: "127.0.0.1:0"}) // Standalone=false: borrow mode
	srv := newBorrowedPluginServer(t, r)
	r.SetPluginServer(srv)

	stop := make(chan struct{})
	firstSignal := make(chan struct{})
	var signaller sync.WaitGroup
	signaller.Go(func() {
		signaled := false
		for {
			select {
			case <-stop:
				return
			default:
			}
			r.SignalPluginStartupComplete()
			r.AddAPIProcessCount(0)
			if !signaled {
				signaled = true
				close(firstSignal)
			}
		}
	})
	<-firstSignal

	startErr := r.StartWithContext(context.Background())
	close(stop)
	signaller.Wait()
	require.NoError(t, startErr)

	assert.Nil(t, r.startupComplete,
		"a borrow-mode start armed the startup barrier, which only a standalone start waits on")
	stopAndWait(t, r)
}
