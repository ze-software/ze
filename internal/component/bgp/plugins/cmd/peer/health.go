// Design: docs/architecture/api/commands.md — show bgp health overview handler
// Detail: summary.go — the per-peer BGP summary this overview complements

package peer

import (
	"time"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// cmdBgpHealth is the command path this file answers, as an operator types it.
const cmdBgpHealth = "show bgp health"

func init() {
	registerHealthShape()

	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-show:bgp-health", Handler: handleShowBGPHealth},
	)
}

// registerHealthShape declares what `show bgp health` answers, so the operators
// its answer cannot support are refused before it runs and the ones it can are
// published.
//
// The answer carries one row for each peer under "peers", beside the two
// aggregate counts (handleShowBGPHealth), so it is `tab` and the row operators
// apply: `| count` answers the peer count and `| match Idle | count` answers
// how many are not established.
//
// The declaration sits on `show bgp health`, which is LONGER than the empty
// declaration registerShapes writes for the same path as a child of `show bgp`
// (peer.go). An empty declaration is a floor and never overrides a value, so
// the two agree whatever order the file initializers run in
// (declarationRegistry.declare in internal/component/command/column_order.go).
func registerHealthShape() {
	command.RegisterShape([]string{cmdBgpHealth}, command.ShapeTab)

	// A row reads as the answer to one question: whose session, is it up, whose
	// AS, and for how long. The two aggregate keys beside the rows share no
	// name with the row, so they order alphabetically and no second declaration
	// is owed (tableStyle.declaredKeys, internal/component/command/pipe_table.go).
	command.RegisterColumns([]string{cmdBgpHealth},
		command.ColumnOrder{"peer", "state", "as", "uptime"},
	)

	// "peer" holds the peer address as a bare string, which is the one form
	// `| resolve` and `| origin` decorate.
	command.RegisterAddressFields([]string{cmdBgpHealth}, "peer")
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
