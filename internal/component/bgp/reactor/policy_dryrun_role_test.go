// Design: docs/guide/bgp-policy.md -- read-only export simulation at an explicit role.
package reactor

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestPolicyDryRunReportsRoleDefault resolves an explicit role through its real
// grant graph, then requires the read-only result to name that exemption.
func TestPolicyDryRunReportsRoleDefault(t *testing.T) {
	r := New(&Config{})
	settings := NewPeerSettings(mustParseAddr("192.0.2.232"), 65000, 65000, 0x01010101)
	settings.DisseminationOwner = dryRunRoleOwner(t)
	peer := NewPeer(settings)
	peer.state.Store(int32(PeerStateEstablished))
	r.peers[settings.PeerKey()] = peer

	result, err := (&reactorAPIAdapter{r: r}).PolicyDryRun(peer.sourceOwner, directionExport, "", dryRunUpdateBody(t), true, "")
	require.NoError(t, err)
	require.Equal(t, dryRunActionAccept, result.Action)
	encoded, err := json.Marshal(result)
	require.NoError(t, err)
	var fields map[string]any
	require.NoError(t, json.Unmarshal(encoded, &fields))
	require.Equal(t, true, fields["role-default"])
	require.False(t, settings.ExportPolicy.Configured())
	require.Empty(t, peer.adjOut.routes)
}

// TestPolicyDryRunKeepsSameAddressOwners gives two peer objects the same
// address and different roles; each prediction must use the requested owner.
func TestPolicyDryRunKeepsSameAddressOwners(t *testing.T) {
	r, ordinary, role := dryRunAliasPeers(t)
	api := &reactorAPIAdapter{r: r}
	for _, test := range []struct {
		peer   *Peer
		action string
		role   bool
	}{
		{ordinary, dryRunActionReject, false},
		{role, dryRunActionAccept, true},
	} {
		result, err := api.PolicyDryRun(test.peer.sourceOwner, directionExport, "", dryRunUpdateBody(t), true, "learned")
		require.NoError(t, err)
		require.Equal(t, test.action, result.Action)
		require.Equal(t, test.role, result.RoleDefault)
	}
}

// TestPolicyDryRunRemovedOwnerNeverSelectsAlias retains a same-address peer
// after removal and requires a refusal instead of that survivor's prediction.
func TestPolicyDryRunRemovedOwnerNeverSelectsAlias(t *testing.T) {
	r, ordinary, role := dryRunAliasPeers(t)
	delete(r.peers, role.settings.PeerKey())
	api := &reactorAPIAdapter{r: r}
	for _, owner := range []uint64{0, role.sourceOwner} {
		result, err := api.PolicyDryRun(owner, directionExport, "", dryRunUpdateBody(t), true, "local")
		require.ErrorIs(t, err, errPeerNotFound)
		require.Nil(t, result)
		require.Contains(t, err.Error(), "fix: ze show bgp")
	}
	result, err := api.PolicyDryRun(ordinary.sourceOwner, directionExport, "", dryRunUpdateBody(t), true, "local")
	require.NoError(t, err)
	require.Equal(t, dryRunActionAccept, result.Action)
}

// TestPolicyDryRunRoleDefaultIgnoresSimulation preserves the explicit role's
// default for either class while keeping external RFC 8212 rejection intact.
func TestPolicyDryRunRoleDefaultIgnoresSimulation(t *testing.T) {
	for _, remoteAS := range []uint32{65000, 65001} {
		for _, kind := range []string{"learned", "local"} {
			r := New(&Config{})
			settings := NewPeerSettings(mustParseAddr("192.0.2.237"), 65000, remoteAS, 1)
			settings.DisseminationOwner = dryRunRoleOwner(t)
			peer := NewPeer(settings)
			r.peers[settings.PeerKey()] = peer
			result, err := (&reactorAPIAdapter{r: r}).PolicyDryRun(peer.sourceOwner, directionExport, "", dryRunUpdateBody(t), true, kind)
			require.NoError(t, err)
			want := dryRunActionReject
			if remoteAS == 65000 {
				want = dryRunActionAccept
			}
			require.Equal(t, want, result.Action)
			require.Equal(t, kind, result.SimulatedRouteKind)
			require.True(t, result.RoleDefault)
			require.Contains(t, result.Reason, "explicit role")
		}
	}
}

func dryRunAliasPeers(t *testing.T) (*Reactor, *Peer, *Peer) {
	t.Helper()
	r := New(&Config{})
	ordinarySettings := NewPeerSettings(mustParseAddr("192.0.2.236"), 65000, 65000, 1)
	roleSettings := NewPeerSettings(ordinarySettings.Address, 65000, 65000, 1)
	roleSettings.Port++
	roleSettings.DisseminationOwner = dryRunRoleOwner(t)
	ordinary, role := NewPeer(ordinarySettings), NewPeer(roleSettings)
	r.peers[ordinarySettings.PeerKey()] = ordinary
	r.peers[roleSettings.PeerKey()] = role
	return r, ordinary, role
}

// dryRunRoleOwner takes the same declared grants and claim boundary as a
// configured role; a role name or route fact alone grants no exemption.
func dryRunRoleOwner(t *testing.T) pluginserver.DisseminationOwner {
	t.Helper()
	const process = "dry-run-role"
	graph := pluginserver.NewDisseminationGraph("bgp", []pluginserver.DeliveryPeer{
		{Addr: "source", Bindings: []plugin.PeerProcessBinding{{PluginName: process, Explicit: true,
			Receive: map[string]events.Direction{"update": events.DirReceived, "state": events.DirBoth}}}},
		{Addr: "destination", Bindings: []plugin.PeerProcessBinding{{PluginName: process, Explicit: true,
			Receive: map[string]events.Direction{"state": events.DirBoth, "refresh": events.DirBoth, "update": events.DirSent},
			Send:    map[string]bool{"update": true}}}},
	})
	owner, err := graph.ResolveDisseminationOwner("destination", SelectedDisseminationRole, []pluginserver.DisseminationProcess{{
		Name: process, Claims: []string{SelectedDisseminationRole}, Declarations: []rpc.DisseminationDecl{{
			Namespace: "bgp", Role: SelectedDisseminationRole,
			SourceReceive:      []string{"update-received", "state"},
			DestinationReceive: []string{"state", "refresh", "update-sent"}, DestinationSend: []string{"update"},
		}},
	}})
	require.NoError(t, err)
	require.True(t, owner.Explicit())
	return owner
}
