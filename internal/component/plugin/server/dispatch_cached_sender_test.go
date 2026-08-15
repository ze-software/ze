// Related: dispatch_cached.go — forwardCached, opRelayStoredRoute, procSender
// Related: bgp/reactor/send_permission_rails_test.go — the guard these two rails
//   hand their sender to, driven over real peers

package server

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestCachedRailsNameTheProcessAsTheSender pins what these two entry points owe
// the send permission: the identity of the process that called them.
//
// Neither rail can be judged here, because the judgement is the reactor's and
// the reactor imports this package. What this side owns is the NAME the reactor
// looks the attach block up by. An op that dropped it would reach the rail as
// the zero Sender and be refused for the wrong reason; an op that named the
// operator would bypass the attach block entirely, which is the hole round 1 of
// the Review Gate found on four rails.
//
// VALIDATES: forward-cached and relay-stored-route carry plugin.ProcessSender of
// the calling process, on the transport a forked plugin uses (JSON params) and
// on the in-process one alike.
// PREVENTS: a plugin RPC arriving at the send permission with an identity no
// config can grant or refuse.
func TestCachedRailsNameTheProcessAsTheSender(t *testing.T) {
	t.Parallel()

	reactor := &mockReactor{}
	s := &Server{reactor: reactor}
	proc := process.NewProcess(plugin.PluginConfig{Name: "adj-rib-in"})

	require.NoError(t, s.forwardCached(proc, []uint64{7}, []string{"192.0.2.7"}))
	require.Len(t, reactor.forwardCalls, 1)
	forwardSender := reactor.forwardCalls[0].sender
	assert.False(t, forwardSender.IsOperator(), "a plugin RPC must not carry operator authority")
	name, ok := forwardSender.Process()
	require.True(t, ok, "forward-cached must name the calling process")
	assert.Equal(t, "adj-rib-in", name)

	params, err := json.Marshal(rpc.RelayStoredRouteInput{
		Destination: "192.0.2.7",
		Routes:      []rpc.StoredRoute{{SourcePeer: "10.0.0.1", Family: "ipv4/unicast", NLRIHex: "180a0000"}},
	})
	require.NoError(t, err)

	_, err = s.opRelayStoredRoute(proc, params)
	require.NoError(t, err)
	require.Len(t, reactor.relayCalls, 1)
	relaySender := reactor.relayCalls[0].sender
	assert.False(t, relaySender.IsOperator(), "a plugin RPC must not carry operator authority")
	name, ok = relaySender.Process()
	require.True(t, ok, "relay-stored-route must name the calling process")
	assert.Equal(t, "adj-rib-in", name)
}

// TestRelayStoredRouteFromAnUnnamedCallerStaysUnnamed is the fail-closed half of
// procSender.
//
// A nil process means the caller could not say who it was. Reading that as the
// operator would hand operator authority to every dispatch path that forgot to
// name itself, on the one guard whose purpose is to stop a process reaching a
// peer that never attached it. The rail refuses an unset sender before it looks
// the peer up ((*reactorAPIAdapter).RelayStoredRoute), so what this side must
// guarantee is that the unset value arrives unchanged.
//
// VALIDATES: procSender(nil) is the zero Sender, and the op passes it on.
// PREVENTS: an unnamed caller being upgraded at the boundary, which would make
// the refusal unreachable.
func TestRelayStoredRouteFromAnUnnamedCallerStaysUnnamed(t *testing.T) {
	t.Parallel()

	reactor := &mockReactor{}
	s := &Server{reactor: reactor}

	params, err := json.Marshal(rpc.RelayStoredRouteInput{
		Destination: "192.0.2.7",
		Routes:      []rpc.StoredRoute{{SourcePeer: "10.0.0.1", Family: "ipv4/unicast", NLRIHex: "180a0000"}},
	})
	require.NoError(t, err)

	_, err = s.opRelayStoredRoute(nil, params)
	require.NoError(t, err)

	require.Len(t, reactor.relayCalls, 1)
	sender := reactor.relayCalls[0].sender
	assert.False(t, sender.IsSet(), "an unnamed caller must stay unnamed")
	assert.False(t, sender.IsOperator(), "an unnamed caller must not become the operator")
}
