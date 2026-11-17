// Design: docs/architecture/api/architecture.md -- runtime ownership at config operations.
package reactor

import (
	"context"
	"encoding/json"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/configop"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestOperationCandidateResolvesRuntimeOwner reaches the real per-transaction
// candidate producer and modify-peer apply. Pure builders supply no owner;
// final-only changes must keep the authenticated live owner and connection.
func TestOperationCandidateResolvesRuntimeOwner(t *testing.T) {
	for _, dynamic := range []bool{false, true} {
		name := "static source"
		if dynamic {
			name = "dynamic group source"
		}
		t.Run(name, func(t *testing.T) {
			f := newSelectedWriterFixture(t, false)
			f.peer.settings.Name = "destination"
			live := f.peer.selectedOwnerSnapshot()
			require.NotNil(t, live)
			require.NoError(t, live.err)
			require.True(t, live.owner.IsSet())
			destination := f.peer.settingsSnapshot()
			destination.DisseminationOwner = pluginserver.DisseminationOwner{}
			destination.ExportPolicy = mustPolicy(t, nil, routepolicy.FinalReject)
			source := &PeerSettings{Name: "source", Address: netip.MustParseAddr("192.0.2.79"),
				ProcessBindings: []ProcessBinding{{PluginName: selectedWriterProcess, Explicit: true,
					Receive: map[string]events.Direction{"update": events.DirReceived, "state": events.DirBoth}}}}
			peers := []*PeerSettings{source, destination}
			var groups []*DynamicGroupConfig
			if dynamic {
				source.Address = netip.Addr{}
				peers = []*PeerSettings{destination}
				groups = []*DynamicGroupConfig{{GroupName: "sources", Settings: source}}
			}
			var loads atomic.Int32
			f.api.r.SetConfigPath("runtime-operation-owner")
			f.api.r.SetReloadFunc(func(string) ([]*PeerSettings, Globals, []*DynamicGroupConfig, error) {
				loads.Add(1)
				return peers, Globals{}, groups, nil
			})
			op := &rpc.ConfigOperation{ID: "bgp-modify-peer-destination", Root: "bgp", Owner: "bgp",
				Type: configop.ModifyPeer, Target: rpc.ResourceRef{Kind: rpc.ResourcePeer, Peer: "destination"},
				Params: rpc.ConfigOperationParams{Peer: "destination"}}
			const tx = "runtime-owner-final-only"
			resolved, _, err := f.api.resolveOperationCandidate(tx, op)
			require.NoError(t, err)
			require.Equal(t, live.owner, resolved.DisseminationOwner)
			require.False(t, destination.DisseminationOwner.IsSet(), "resolution mutated the cached pure candidate")
			require.False(t, source.DisseminationOwner.IsSet(), "resolution mutated the source template")
			require.NoError(t, f.api.r.VerifyConfigOperation(tx, op))
			session, owner := f.peer.currentSession(), f.peer.sourceOwner
			j := &rfc8212TxJournal{Journal: sdk.NewJournal(), txID: tx, deadline: time.Now().Add(time.Minute)}
			_, err = f.api.r.ApplyConfigOperation(op, j)
			require.NoError(t, err)
			require.Same(t, f.peer, f.api.r.peers[f.peer.settings.PeerKey()])
			require.Same(t, session, f.peer.currentSession())
			require.Equal(t, owner, f.peer.sourceOwner)
			require.Equal(t, routepolicy.FinalReject, f.peer.ExportPolicy().Final())
			require.Equal(t, int32(1), loads.Load(), "verify and apply must share only the pure parse")
			require.NoError(t, f.api.r.CompleteSourcePolicyTx(context.Background(), tx, true))
		})
	}
}

// TestReconcilePeersVerifyRuntimeOwnerPreflight invokes the residual handler
// directly. Another modify-peer may also verify a whole transaction; this
// control claims only that the residual handler itself refuses before mutation.
func TestReconcilePeersVerifyRuntimeOwnerPreflight(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		refresh               bool
		badDynamicDestination bool
		wantError             string
	}{
		{name: "missing route refresh", wantError: "session did not negotiate Route Refresh"},
		{name: "negotiated route refresh", refresh: true},
		{name: "incomplete dynamic destination", refresh: true, badDynamicDestination: true, wantError: "incomplete destination grants"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newSelectedWriterFixture(t, false)
			live := f.peer.selectedOwnerSnapshot()
			require.NotNil(t, live)
			require.NoError(t, live.err)
			require.True(t, live.owner.IsSet())
			f.peer.mu.Lock()
			f.peer.settings.Name = "destination"
			f.peer.settings.DisseminationOwner = live.owner
			f.peer.settings.ImportPolicy = filterapi.Policy{}
			f.peer.mu.Unlock()
			f.peer.negotiated.Store(relaxNegotiated(tc.refresh))
			before, session := f.peer.settingsSnapshot(), f.peer.currentSession()
			destination := *before
			destination.DisseminationOwner = pluginserver.DisseminationOwner{}
			if !tc.badDynamicDestination {
				destination.ImportPolicy = mustPolicy(t, nil, routepolicy.FinalAccept)
			}
			source := &PeerSettings{Name: "source", Address: netip.MustParseAddr("192.0.2.79"),
				ProcessBindings: []ProcessBinding{{PluginName: selectedWriterProcess, Explicit: true,
					Receive: map[string]events.Direction{"update": events.DirReceived, "state": events.DirBoth}}}}
			var groups []*DynamicGroupConfig
			if tc.badDynamicDestination {
				groups = []*DynamicGroupConfig{{GroupName: "incomplete-destination", Settings: &PeerSettings{
					ProcessBindings: []ProcessBinding{{PluginName: selectedWriterProcess, Explicit: true,
						Receive: map[string]events.Direction{"state": events.DirBoth, "update": events.DirSent},
						Send:    map[string]bool{"update": true}}}}}}
			}
			f.api.r.SetConfigPath("residual-runtime-preflight")
			f.api.r.SetReloadFunc(func(string) ([]*PeerSettings, Globals, []*DynamicGroupConfig, error) {
				return []*PeerSettings{source, &destination}, Globals{}, groups, nil
			})
			op := &rpc.ConfigOperation{ID: "bgp-reconcile-peers", Root: "bgp", Owner: "bgp", Type: configop.ReconcilePeers,
				Params: rpc.ConfigOperationParams{Config: json.RawMessage(`{}`)}}
			err := f.api.r.VerifyConfigOperation("residual-runtime-preflight", op)
			if tc.wantError == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.wantError)
			}
			require.Equal(t, *before, *f.peer.settingsSnapshot(), "verify mutated running settings")
			require.Same(t, session, f.peer.currentSession(), "verify replaced the established connection")
			require.Same(t, live, f.peer.selectedOwnerSnapshot(), "verify changed live output authority")
		})
	}
}
