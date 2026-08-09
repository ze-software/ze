// Design: docs/architecture/traffic/traffic-analysis-layers.md -- show traffic feature CLI handler
//
// The show traffic feature view is trafficfeature's Spec-1 consumer: it surfaces
// the neutral per-source feature signals for operators, and satisfies wiring
// completeness before the anomaly detector (Spec 2) becomes a second consumer.

package cmd

import (
	"math"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/component/trafficfeature"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:traffic-feature",
			Handler:    handleShowTrafficFeature,
		},
	)
}

func handleShowTrafficFeature(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	svc := trafficfeature.EnsureGlobal()
	if svc == nil {
		return &plugin.Response{Status: plugin.StatusError, Error: "trafficfeature service not available"}, nil
	}

	id := svc.Attach()
	defer svc.Detach(id)

	snap := svc.Snapshot()

	var filter string
	if len(args) > 0 {
		filter = args[0]
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   snapshotToMap(snap, filter),
	}, nil
}

func snapshotToMap(snap *trafficfeature.Snapshot, filter string) plugin.Map {
	sources := make([]plugin.Map, 0, len(snap.Sources))
	for _, fe := range snap.Sources {
		if filter != "" && fe.Addr.String() != filter {
			continue
		}
		entry := plugin.Map{
			"address":      fe.Addr.String(),
			"fan-out":      fe.FanOut,
			"port-entropy": fe.PortEntropy,
			"new-peer":     fe.NewPeer,
			"rare-port":    fe.RarePort,
			"beaconing":    fe.Beaconing,
		}
		// A ratio of +Inf (no inbound bytes) is not valid JSON; surface it as a
		// string sentinel so the exfil signal is still visible.
		if math.IsInf(fe.OutInRatio, 1) {
			entry["out-in-ratio"] = "inf"
		} else {
			entry["out-in-ratio"] = fe.OutInRatio
		}
		sources = append(sources, entry)
	}
	return plugin.Map{
		"degraded":       snap.Degraded,
		"top-source-ips": sources,
	}
}
