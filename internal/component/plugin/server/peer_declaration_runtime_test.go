// Design: docs/architecture/api/process-protocol.md -- runtime peer declarations.
// Related: filter_insertion_change.go -- serialized peer configuration rebuilds.
package server

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestRestartWithChangedClaimsRebuildsPeers drives the existing restart entry
// point. A newly declared opaque role must rebuild the running peer settings;
// a second restart with the same declaration must leave them alone.
func TestRestartWithChangedClaimsRebuildsPeers(t *testing.T) {
	snapshot := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snapshot) })
	const name = "runtime-peer-claims-restart"
	var claim atomic.Bool
	registerLifecyclePlugin(t, name, nil, func(conn net.Conn) int {
		p := sdk.NewWithConn(name, conn)
		registration := &sdk.Registration{}
		if claim.Load() {
			registration.Claims = []string{"runtime-peer-owner"}
		}
		if err := p.Run(context.Background(), registration); err != nil {
			return 1
		}
		return 0
	})
	s, _ := newLifecycleStartupServer(t)
	require.NoError(t, s.runPluginPhase([]plugin.PluginConfig{{Name: name, Internal: true, Encoder: plugin.EncodingJSON}}))
	s.signalStartupComplete()
	calls, unlocked := insertionListener(t, s)
	claim.Store(true)
	require.NoError(t, s.restartPlugin(name))
	require.Equal(t, int64(1), calls.Load(), "Claims change is peer configuration, even without an insertion")
	require.Zero(t, unlocked.Load(), "the callback must run under transaction exclusion")
	require.NoError(t, s.restartPlugin(name))
	require.Equal(t, int64(1), calls.Load(), "equal runtime metadata rebuilds nothing")
}

// TestReloadStoppingAClaimantRebuildsPeers proves removing a runtime claim
// reaches the same serialized rebuild as removing a filter insertion.
func TestReloadStoppingAClaimantRebuildsPeers(t *testing.T) {
	snapshot := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snapshot) })
	const name = "runtime-peer-claims-stop"
	const root = "runtimepeerclaimstop"
	require.NoError(t, registry.Register(registry.Registration{
		Name: name, Description: "runtime peer declaration removal test", ConfigRoots: []string{root},
		CLIHandler: func([]string) int { return 0 },
		RunEngine: func(conn net.Conn) int {
			p := sdk.NewWithConn(name, conn)
			if err := p.Run(context.Background(), &sdk.Registration{Claims: []string{"runtime-peer-owner"}}); err != nil {
				return 1
			}
			return 0
		},
	}))
	s, _ := newLifecycleStartupServer(t)
	require.NoError(t, s.runPluginPhase([]plugin.PluginConfig{{Name: name, Internal: true, Encoder: plugin.EncodingJSON}}))
	s.signalStartupComplete()
	calls, unlocked := insertionListener(t, s)
	require.True(t, s.txLock.tryAcquire())
	require.NoError(t, s.stopCollectedProcesses(s.collectProcessesForRemovedConfigPaths([]string{root}, nil)))
	require.Zero(t, calls.Load(), "removal waits for the transaction's resulting running tree")
	s.txLock.release()
	require.Equal(t, int64(1), calls.Load())
	require.Zero(t, unlocked.Load())
}

// TestRestartWithChangedDisseminationRebuildsPeers proves the role's grant
// requirements are runtime peer metadata even when Claims stay empty.
func TestRestartWithChangedDisseminationRebuildsPeers(t *testing.T) {
	snapshot := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snapshot) })
	const name = "runtime-peer-dissemination-restart"
	var declares atomic.Bool
	registerLifecyclePlugin(t, name, nil, func(conn net.Conn) int {
		p := sdk.NewWithConn(name, conn)
		registration := &sdk.Registration{}
		if declares.Load() {
			registration.Dissemination = []rpc.DisseminationDecl{disseminationTestDeclaration()}
		}
		if err := p.Run(context.Background(), registration); err != nil {
			return 1
		}
		return 0
	})
	s, _ := newLifecycleStartupServer(t)
	s.UpdateDeliveryGraph(disseminationTestNamespace, nil)
	require.NoError(t, s.runPluginPhase([]plugin.PluginConfig{{Name: name, Internal: true, Encoder: plugin.EncodingJSON}}))
	s.signalStartupComplete()
	calls, unlocked := insertionListener(t, s)
	declares.Store(true)
	require.NoError(t, s.restartPlugin(name))
	require.Equal(t, int64(1), calls.Load(), "the runtime role declaration must rebuild peers")
	require.Zero(t, unlocked.Load())
	require.NoError(t, s.restartPlugin(name))
	require.Equal(t, int64(1), calls.Load(), "unchanged role requirements rebuild nothing")
}

// TestChangedPeerDeclarationRefusesUnreconciledReplacement exercises the
// listener error on both paths. Deferred refusal follows the current exact
// replacement through an unchanged second respawn, leaving its useful sibling.
func TestChangedPeerDeclarationRefusesUnreconciledReplacement(t *testing.T) {
	for _, test := range []struct {
		name                    string
		deferred, duringRebuild bool
	}{
		{name: "immediate"}, {name: "deferred successor", deferred: true},
		{name: "successor during rebuild", deferred: true, duringRebuild: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			deferred := test.deferred
			snapshot := registry.Snapshot()
			t.Cleanup(func() { registry.Restore(snapshot) })
			const name = "runtime-peer-refusal"
			const sibling = "runtime-peer-refusal-sibling"
			var claims atomic.Bool
			registerLifecyclePlugin(t, name, nil, func(conn net.Conn) int {
				p := sdk.NewWithConn(name, conn)
				registration := &sdk.Registration{}
				if claims.Load() {
					registration.Claims = []string{"runtime-peer-owner"}
				}
				if err := p.Run(context.Background(), registration); err != nil {
					return 1
				}
				return 0
			})
			registerLifecyclePlugin(t, sibling, nil, func(conn net.Conn) int {
				p := sdk.NewWithConn(sibling, conn)
				if err := p.Run(context.Background(), &sdk.Registration{}); err != nil {
					return 1
				}
				return 0
			})
			s, _ := newLifecycleStartupServer(t)
			require.NoError(t, s.runPluginPhase([]plugin.PluginConfig{
				{Name: name, Internal: true, Encoder: plugin.EncodingJSON},
				{Name: sibling, Internal: true, Encoder: plugin.EncodingJSON},
			}))
			s.signalStartupComplete()
			pm := s.procManager.Load()
			useful := pm.GetProcess(sibling)
			var calls atomic.Int64
			registry.SetPeerDeclarationsChanged(func() error {
				calls.Add(1)
				require.False(t, s.txLock.tryAcquire(), "reconciliation holds config exclusion")
				if test.duringRebuild {
					before := pm.GetProcess(name)
					require.NoError(t, s.restartPlugin(name))
					require.NotSame(t, before, pm.GetProcess(name), "the callback must refuse the latest exact runtime")
				}
				return errors.New("runtime candidate refused")
			})
			t.Cleanup(func() { registry.SetPeerDeclarationsChanged(nil) })
			claims.Store(true)
			if deferred {
				require.True(t, s.txLock.tryAcquire())
			}
			err := s.restartPlugin(name)
			if deferred {
				require.NoError(t, err)
				first := pm.GetProcess(name)
				require.NotNil(t, first)
				require.NoError(t, s.restartPlugin(name))
				require.NotSame(t, first, pm.GetProcess(name), "the pending failure must follow its exact successor")
				require.Zero(t, calls.Load())
				s.txLock.release()
			} else {
				require.ErrorContains(t, err, "runtime candidate refused")
			}
			require.Equal(t, int64(1), calls.Load())
			require.Nil(t, pm.GetProcess(name), "unreconciled runtime authority must be stopped")
			stopped, down := pm.Stopped(name)
			require.True(t, down)
			require.Contains(t, stopped.Detail, "runtime candidate refused")
			require.Same(t, useful, pm.GetProcess(sibling), "the independent process remains useful")
			require.True(t, s.txLock.tryAcquire(), "refusal releases the transaction")
			s.txLock.release()
		})
	}
}
