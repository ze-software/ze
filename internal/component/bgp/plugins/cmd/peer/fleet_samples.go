// Design: docs/architecture/fleet-config.md -- BGP peer fleet metric sample producer
// Related: summary.go -- peer state counting over reactor snapshots

package peer

import (
	"encoding/json"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/pkg/fleet"
)

const (
	cmdFleetSourceBGPPeer  = "show fleet source bgp-peer"
	wireFleetSourceBGPPeer = "ze-bgp:fleet-source-bgp-peer"
)

// registerFleetSourceBGPPeer registers the hidden BGP peer fleet sample source
// RPC and its document answer shape.
func registerFleetSourceBGPPeer() {
	command.RegisterShape([]string{cmdFleetSourceBGPPeer}, command.ShapeDoc)
	command.RegisterAddressFields([]string{cmdFleetSourceBGPPeer})

	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: wireFleetSourceBGPPeer, Handler: handleFleetSourceBGPPeer},
	)
}

// handleFleetSourceBGPPeer returns the current established and configured BGP
// peer counts as a bounded fleet.SourceSnapshot. When BGP is not attached to
// the daemon's coordinator (or no reactor is present), it emits no readings so
// the collector records the figures as absent rather than zero.
func handleFleetSourceBGPPeer(ctx *pluginserver.CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
	snap := buildFleetBGPPeerSnapshot(ctx)
	raw, err := json.Marshal(snap)
	if err != nil {
		return nil, err
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.RawJSON(raw)}, nil
}

// buildFleetBGPPeerSnapshot projects the live BGP reactor's peer slice into a
// fleet.SourceSnapshot. A bare coordinator with no attached BGP reactor yields
// an empty snapshot (absent), while an attached BGP reactor with zero peers
// reports a real zero for both figures.
func buildFleetBGPPeerSnapshot(ctx *pluginserver.CommandContext) fleet.SourceSnapshot {
	reactor, live := liveBGPReactor(ctx)
	if !live {
		return fleet.SourceSnapshot{Owner: fleet.SampleOwnerBGPPeer}
	}

	peers := reactor.Peers()
	established := 0
	for i := range peers {
		if peers[i].State == plugin.PeerStateEstablished {
			established++
		}
	}

	return fleet.SourceSnapshot{
		Owner: fleet.SampleOwnerBGPPeer,
		Readings: []fleet.SourceReading{
			{
				Figure: fleet.FigureBGPPeersEstablished,
				Unit:   fleet.SampleUnitCount,
				Value:  float64(established),
			},
			{
				Figure: fleet.FigureBGPPeersConfigured,
				Unit:   fleet.SampleUnitCount,
				Value:  float64(len(peers)),
			},
		},
	}
}

// liveBGPReactor returns the command context's reactor when an actual BGP
// reactor is attached. A coordinator with no BGP block configured satisfies
// plugin.ReactorLifecycle but not bgptypes.BGPReactor, so checking the BGP
// interface prevents reporting zero peers on a non-BGP daemon.
func liveBGPReactor(ctx *pluginserver.CommandContext) (plugin.ReactorLifecycle, bool) {
	if ctx == nil {
		return nil, false
	}
	reactor := ctx.Reactor()
	if reactor == nil {
		return nil, false
	}
	if _, ok := reactor.(bgptypes.BGPReactor); !ok {
		return nil, false
	}
	return reactor, true
}
