// Design: docs/architecture/fleet-config.md -- L2TP fleet metric sample producer
// Related: l2tp.go -- L2TP service locator and summary snapshot handler

package cmd

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/l2tp"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/pkg/fleet"
)

const (
	cmdFleetSourceL2TP  = "show fleet source l2tp"
	wireFleetSourceL2TP = "ze-l2tp:fleet-source-l2tp"
)

// registerFleetSourceL2TP registers the hidden L2TP fleet sample source RPC
// and its document answer shape.
func registerFleetSourceL2TP() {
	command.RegisterShape([]string{cmdFleetSourceL2TP}, command.ShapeDoc)
	command.RegisterAddressFields([]string{cmdFleetSourceL2TP})

	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: wireFleetSourceL2TP, Handler: handleFleetSourceL2TP},
	)
}

// handleFleetSourceL2TP returns the active L2TP subscriber session count as a
// bounded fleet.SourceSnapshot. When the L2TP subsystem is not running, it
// returns empty readings so the collector treats L2TP as absent rather than
// reporting a synthetic zero.
func handleFleetSourceL2TP(_ *pluginserver.CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
	svc := l2tp.LookupService()
	if svc == nil {
		return jsonResponse("fleet source l2tp", fleet.SourceSnapshot{Owner: fleet.SampleOwnerL2TP})
	}
	snap := svc.Snapshot()
	out := fleet.SourceSnapshot{
		Owner: fleet.SampleOwnerL2TP,
		Readings: []fleet.SourceReading{
			{
				Figure: fleet.FigureSubscriberSessions,
				Unit:   fleet.SampleUnitCount,
				Value:  float64(snap.SessionCount),
			},
		},
	}
	return jsonResponse("fleet source l2tp", out)
}
