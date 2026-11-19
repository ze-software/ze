// Design: docs/architecture/config/transaction-protocol.md -- partial root delivery.
// Related: rib_event_settlement_test.go -- the real RIB's SDK startup handshake.
package rib

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// ac17ForkedConfigRuntime uses the external-style SDK transport with no bridge
// and reads the actual runRIBPlugin declaration. The shared startup helper owns
// the handshake and runtime-ready join; no compiled registry adds read roots.
// Closing the engine connection MUST join the real RIB and its workers.
func ac17ForkedConfigRuntime(t *testing.T) (context.Context, *ipc.PluginConn, *RIBManager, rpc.DeclareRegistrationInput) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	pluginEnd, engineEnd := net.Pipe()
	connection := ipc.NewMuxPluginConn(rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd)))
	done := make(chan int, 1)
	go func() { done <- runRIBPlugin(pluginEnd) }()
	t.Cleanup(func() {
		defer cancel()
		// testing cancels t.Context before cleanup begins. Shutdown still owes
		// the actual RIB return, so its bounded join needs an independent wait.
		cleanup, finish := context.WithTimeout(context.WithoutCancel(t.Context()), 5*time.Second)
		defer finish()
		closeError := connection.Close()
		select {
		case <-done:
		case <-cleanup.Done():
			t.Error("the real RIB did not stop after its configuration connection closed", cleanup.Err())
		}
		require.NoError(t, closeError)
	})
	declaration := quiesceRIBStartup(t, ctx, connection, nil)
	r := activeManager.Load()
	require.NotNil(t, r)
	return ctx, connection, r, declaration
}

// TestAC17ForkedRIBDeclaresOriginPreferenceConfig requires the real Stage-1
// declaration to request both consumed roots before an external engine can
// construct Stage 2. The compiled ConfigReads row cannot repair this transport.
// MUTATION: omit rib from runtime WantsConfig; configured static200 is then
// absent from the forked provider even though CLI and internal RIB can read it.
func TestAC17ForkedRIBDeclaresOriginPreferenceConfig(t *testing.T) {
	_, _, _, declaration := ac17ForkedConfigRuntime(t)
	require.Contains(t, declaration.WantsConfig, "bgp")
	require.Contains(t, declaration.WantsConfig, "rib")
}

// TestAC17ForkedRIBPartialConfigCallbacksPreserveOtherRoot delivers a complete
// config through the actual SDK configure callback, then verifies, applies and
// rolls back each changed-root-only delivery the external engine may send.
// Missing roots retain their installed values; an explicitly removed rib root
// restores defaults. Verify itself MUST change no running value.
// MUTATION: parse each partial delivery over defaults rather than the last
// installed config; either distances or the nondefault BGP multipath rule falls.
func TestAC17ForkedRIBPartialConfigCallbacksPreserveOtherRoot(t *testing.T) {
	ctx, connection, r, _ := ac17ForkedConfigRuntime(t)
	baseline := []rpc.ConfigSection{
		{Root: "bgp", Data: `{"bgp":{"multipath":{"maximum-paths":"4","relax-as-path":"true"}}}`},
		{Root: "rib", Data: `{"rib":{"distance":{"static":"200","ebgp":"20"}}}`},
	}
	bgpOnly := rpc.ConfigSection{Root: "bgp", Data: `{"bgp":{"multipath":{"maximum-paths":"8","relax-as-path":"false"}}}`}
	// The shared startup has completed with no sections. Send the same Stage-2
	// configure RPC with actual data to exercise the registered installer.
	require.NoError(t, connection.SendConfigure(ctx, baseline, nil))
	requireConfig := func(t *testing.T, maximum uint32, relax bool, static uint8) {
		t.Helper()
		require.Equal(t, maximum, r.maximumPaths.Load())
		require.Equal(t, relax, r.relaxASPath.Load())
		r.selectedDispatch.mu.Lock()
		staticDistance := r.selectedDispatch.distances["static"]
		ebgpDistance := r.selectedDispatch.distances["ebgp"]
		r.selectedDispatch.mu.Unlock()
		require.Equal(t, static, staticDistance)
		require.Equal(t, uint8(20), ebgpDistance)
	}
	requireConfig(t, 4, true, 200)
	for _, question := range []struct {
		name    string
		section rpc.ConfigSection
		maximum uint32
		relax   bool
		static  uint8
	}{
		{
			name:    "BGP-only retains distances",
			section: bgpOnly,
			maximum: 8, relax: false, static: 200,
		},
		{
			name:    "RIB-only retains BGP rules",
			section: rpc.ConfigSection{Root: "rib", Data: `{"rib":{"distance":{"static":"250","ebgp":"20"}}}`},
			maximum: 4, relax: true, static: 250,
		},
		{
			name:    "removed RIB restores defaults",
			section: rpc.ConfigSection{Root: "rib", Data: `{}`},
			maximum: 4, relax: true, static: 10,
		},
	} {
		t.Run(question.name, func(t *testing.T) {
			require.NoError(t, connection.SendConfigure(ctx, baseline, nil), "each callback question must be independent of an earlier failure")
			verified, err := connection.SendConfigVerify(ctx, []rpc.ConfigSection{question.section})
			require.NoError(t, err)
			require.Equal(t, rpc.StatusOK, verified.Status)
			requireConfig(t, 4, true, 200)
			applied, err := connection.SendConfigApply(ctx, nil)
			require.NoError(t, err)
			require.Equal(t, rpc.StatusOK, applied.Status)
			requireConfig(t, question.maximum, question.relax, question.static)
			require.NoError(t, connection.SendConfigRollback(ctx, "AC17 partial root rollback"))
			requireConfig(t, 4, true, 200)
			if question.section.Root != "rib" {
				return
			}
			// No Configure intervenes: this second partial edit must merge
			// against the rolled-back installed value, not the undone apply.
			verified, err = connection.SendConfigVerify(ctx, []rpc.ConfigSection{bgpOnly})
			require.NoError(t, err)
			require.Equal(t, rpc.StatusOK, verified.Status)
			requireConfig(t, 4, true, 200)
			applied, err = connection.SendConfigApply(ctx, nil)
			require.NoError(t, err)
			require.Equal(t, rpc.StatusOK, applied.Status)
			requireConfig(t, 8, false, 200)
			require.NoError(t, connection.SendConfigRollback(ctx, "AC17 post-rollback partial edit"))
			requireConfig(t, 4, true, 200)
		})
	}
}
