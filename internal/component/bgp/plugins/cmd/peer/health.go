// Design: docs/architecture/api/commands.md — show bgp health overview handler
// Detail: summary.go — the per-peer BGP summary this overview complements

package peer

import (
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-show:bgp-health", Handler: handleShowBGPHealth},
	)
}

func handleShowBGPHealth(ctx *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	if ctx == nil || ctx.Reactor() == nil {
		return &plugin.Response{Status: plugin.StatusError, Error: "reactor not available"}, nil
	}
	peers := ctx.Reactor().Peers()
	rows := make([]map[string]any, 0, len(peers))
	notEstablished := 0
	for i := range peers {
		p := &peers[i]
		state := p.State.String()
		if p.State != plugin.PeerStateEstablished {
			notEstablished++
		}
		row := map[string]any{
			"peer":   p.Address.String(),
			"state":  state,
			"as":     p.PeerAS,
			"uptime": p.Uptime.Truncate(time.Second).String(),
		}
		rows = append(rows, row)
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{"peers": rows, "count": len(rows), "not-established": notEstablished},
	}, nil
}
