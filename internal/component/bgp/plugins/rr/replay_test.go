package rr

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"slices"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

type replayDispatchCall struct {
	command string
	args    []string
	peer    string
}

// TestReplayForPeerRelaysTheStoredRoutesItself pins the owner-relayed peer-up
// replay: bgp-rr reads the store's routes page by page and relays them under
// its OWN name, because the destination grants `send [ update ]` to bgp-rr and
// gives the receive store nothing.
//
// Method: answer the replay-routes reads on a DirectBridge, the first with a
// two-page walk (More set) and the delta with an empty page, and record every
// RelayStoredRoute the plugin makes.
//
// VALIDATES: replayForPeer dispatches command=ribevents.ReplayRoutesCommand with
// peer and cursor in args, follows More to the next page from last-index, relays
// each page to the peer itself, and resumes the delta from the last relayed
// sequence.
// PREVENTS: the replay going out under the store's name, which the engine judges
// on the store's grant and refuses, so a late destination never received the
// routes already held (silent-fall-through journal, 2026-10-21).
func TestReplayForPeerRelaysTheStoredRoutesItself(t *testing.T) {
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

	const route = `{"source-peer":"10.0.0.9","family":"ipv4/unicast","nlri-hex":"180a0000"}`
	pages := map[string]string{
		"0": `{"routes":[` + route + `,` + route + `],"last-index":4,"more":true}`,
		"4": `{"routes":[` + route + `],"last-index":5,"more":false}`,
		"5": `{"routes":[],"last-index":0,"more":false}`,
	}

	var mu sync.Mutex
	var calls []replayDispatchCall
	var relayed []int
	bridge.SetDispatchCommandArgs(func(_ context.Context, command string, args []string, peer string) (*rpc.DispatchCommandOutput, error) {
		mu.Lock()
		calls = append(calls, replayDispatchCall{command: command, args: slices.Clone(args), peer: peer})
		mu.Unlock()
		page, ok := pages[args[1]]
		if !ok {
			return nil, errors.New("unexpected cursor " + args[1])
		}
		return &rpc.DispatchCommandOutput{Status: statusDone, Data: json.RawMessage(page)}, nil
	})
	bridge.SetRelayStoredRoute(func(_ context.Context, destination string, routes []rpc.StoredRoute) error {
		assert.Equal(t, "10.0.0.1", destination)
		mu.Lock()
		relayed = append(relayed, len(routes))
		mu.Unlock()
		return nil
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

	rr.replayForPeer("10.0.0.1", 1, 17)

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, calls, 3)
	for _, call := range calls {
		assert.Equal(t, ribevents.ReplayRoutesCommand, call.command)
		assert.Empty(t, call.peer)
	}
	assert.Equal(t, []string{"10.0.0.1", "0"}, calls[0].args)
	assert.Equal(t, []string{"10.0.0.1", "4"}, calls[1].args, "More must resume from the page's last-index")
	assert.Equal(t, []string{"10.0.0.1", "5"}, calls[2].args, "the delta must resume after the last relayed route")
	assert.Equal(t, []int{2, 1}, relayed, "each page must be relayed by bgp-rr itself, in order")
}
