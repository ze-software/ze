// Design: docs/architecture/fleet-config.md -- PPPoE fleet metric sample producer
// Related: pppoe.go -- PPPoE access-concentrator service locator and summary handler

package cmd

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/pkg/fleet"
)

const (
	cmdFleetSourcePPPoE  = "show fleet source pppoe"
	wireFleetSourcePPPoE = "ze-l2tp:fleet-source-pppoe"
)

// registerFleetSourcePPPoE registers the hidden PPPoE fleet sample source RPC
// and its document answer shape.
func registerFleetSourcePPPoE() {
	command.RegisterShape([]string{cmdFleetSourcePPPoE}, command.ShapeDoc)
	command.RegisterAddressFields([]string{cmdFleetSourcePPPoE})

	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: wireFleetSourcePPPoE, Handler: handleFleetSourcePPPoE},
	)
}

// handleFleetSourcePPPoE returns the active PPPoE access-concentrator
// subscriber session count as a bounded fleet.SourceSnapshot. Outbound PPPoE
// client interfaces live in the separate pppoeclient component and are excluded
// by design. When the PPPoE access concentrator is not running, it returns
// empty readings so the collector treats PPPoE as absent rather than zero.
func handleFleetSourcePPPoE(_ *pluginserver.CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
	svc := pppoe.LookupService()
	if svc == nil {
		return jsonResponse("fleet source pppoe", fleet.SourceSnapshot{Owner: fleet.SampleOwnerPPPoE})
	}
	return jsonResponse("fleet source pppoe", fleetPPPoESnapshot(svc.Snapshot()))
}

// fleetPPPoESnapshot projects an access-concentrator Snapshot into a bounded
// fleet.SourceSnapshot carrying the subscriber-sessions count.
func fleetPPPoESnapshot(snap pppoe.Snapshot) fleet.SourceSnapshot {
	return fleet.SourceSnapshot{
		Owner: fleet.SampleOwnerPPPoE,
		Readings: []fleet.SourceReading{
			{
				Figure: fleet.FigureSubscriberSessions,
				Unit:   fleet.SampleUnitCount,
				Value:  float64(snap.SessionCount),
			},
		},
	}
}
