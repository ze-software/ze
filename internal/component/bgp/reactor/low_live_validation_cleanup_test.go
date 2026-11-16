// Design: docs/architecture/plugin/rib-storage-design.md -- validation ownership survives a daemon's shutdown.
// Related: rfc7705_live_behavior_test.go -- each live fixture owns one fictional daemon.
package reactor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin/process"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
)

// TestLowLiveRouterReleasesValidationAfterCleanup asks whether one fictional
// daemon's enabled validation gate survives into the next test's baseline.
func TestLowLiveRouterReleasesValidationAfterCleanup(t *testing.T) {
	require.False(t, ribevents.ValidationEnabled(), "the fixture must start without another validation owner")
	var srv *pluginserver.Server
	var processes []*process.Process
	// A red regression must also leave the test process usable. These handles
	// outlive the manager's map, which Stop clears. Never clear a live owner.
	t.Cleanup(func() {
		if srv == nil {
			return // The child failed before it constructed a server or enabled validation.
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		for _, proc := range processes {
			require.NoError(t, proc.WaitEngine(ctx), "the regression's plugin engine survived fixture cleanup")
			require.NoError(t, proc.Wait(ctx), "the regression's plugin workers survived fixture cleanup")
		}
		require.NoError(t, srv.Wait(ctx), "the regression's server handlers survived fixture cleanup")
		ribevents.RegisterValidationLookup(nil, nil)
	})

	t.Run("enabled fixture", func(t *testing.T) {
		peers := lowLiveRouter(t, acceptingLiveSettings(t, "192.0.2.1", 65000, 65002))
		srv = peers[0].peer.reactor.api
		processes = srv.ProcessManager().AllProcesses()
		proc := srv.ProcessManager().GetProcess("bgp-adj-rib-in")
		require.NotNil(t, proc)
		llgrWriterCommand(t, proc, "request bgp adj-rib-in enable-validation")
		require.True(t, ribevents.ValidationEnabled(), "a live fixture must keep its enabled validation gate")
	})

	require.False(t, ribevents.ValidationEnabled(), "a joined fixture must restore the next daemon's ungated baseline")
}
