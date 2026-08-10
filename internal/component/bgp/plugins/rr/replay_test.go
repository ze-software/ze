package rr

import (
	"encoding/json"
	"net"
	"slices"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

type replayDispatchCall struct {
	command string
	args    []string
	peer    string
}

// TestReplayForPeerDispatchesTypedReplayCommand verifies RR keeps the exact
// registered command name separate from its replay args.
//
// VALIDATES: replayForPeer dispatches command="request bgp adj-rib-in replay" with peer/index in args.
// PREVENTS: peer or cursor being folded into the exact command name, which would miss registry lookup.
func TestReplayForPeerDispatchesTypedReplayCommand(t *testing.T) {
	t.Parallel()

	bridge := rpc.NewDirectBridge()
	pluginEnd, engineEnd := net.Pipe()
	t.Cleanup(func() {
		_ = engineEnd.Close()
	})

	pluginConn := rpc.NewBridgedConn(pluginEnd, bridge)
	p := sdk.NewWithConn("rr-test", pluginConn)
	t.Cleanup(func() {
		_ = p.Close()
	})

	var mu sync.Mutex
	var calls []replayDispatchCall
	bridge.SetDispatchCommandArgs(func(command string, args []string, peer string) (*rpc.DispatchCommandOutput, error) {
		mu.Lock()
		calls = append(calls, replayDispatchCall{command: command, args: slices.Clone(args), peer: peer})
		callNum := len(calls)
		mu.Unlock()

		data := json.RawMessage(`{"last-index":5,"replayed":0}`)
		if callNum == 1 {
			data = json.RawMessage(`{"last-index":5,"replayed":3}`)
		}
		return &rpc.DispatchCommandOutput{Status: statusDone, Data: data}, nil
	})
	bridge.SetReady()

	rr := &routeReflector{
		plugin: p,
		peers: map[string]*peerState{
			"10.0.0.1": {
				Address:   "10.0.0.1",
				Up:        true,
				ReplayGen: 1,
				Families:  map[family.Family]bool{},
			},
		},
		withdrawals: make(map[string]map[string]withdrawalInfo),
	}

	rr.replayForPeer("10.0.0.1", 1)

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, calls, 2)
	assert.Equal(t, "request bgp adj-rib-in replay", calls[0].command)
	assert.Equal(t, []string{"10.0.0.1", "0"}, calls[0].args)
	assert.Empty(t, calls[0].peer)
	assert.Equal(t, "request bgp adj-rib-in replay", calls[1].command)
	assert.Equal(t, []string{"10.0.0.1", "5"}, calls[1].args)
	assert.Empty(t, calls[1].peer)
}
