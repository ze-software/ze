// Design: docs/architecture/fleet-config.md — host command handler for fleet metric sampling
// Related: register.go — host-cmd RPC registrations

package cmd

import (
	"encoding/json"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/host"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/fleet"
)

// sampleHostFleetMetrics is a package variable so unit tests can inject a
// deterministic host snapshot without depending on host procfs timing.
var sampleHostFleetMetrics = host.SampleFleetMetrics

func registerFleetSamples() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod: "ze-host:show-fleet-source-host",
			Handler:    handleShowFleetSourceHost,
		},
	)
}

func handleShowFleetSourceHost(_ *pluginserver.CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
	snap := sampleHostFleetMetrics()
	if err := fleet.ValidateSourceSnapshot(fleet.SampleOwnerHost, &snap); err != nil {
		return plugin.OperatorErrorResponse(opererr.Wrap(err,
			"host fleet metric sample is invalid",
			"the local host sampler produced a reading outside the fleet sample schema",
			"show doctor")), nil
	}
	encoded, err := json.Marshal(snap)
	if err != nil {
		return nil, err
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.RawJSON(encoded)}, nil
}
