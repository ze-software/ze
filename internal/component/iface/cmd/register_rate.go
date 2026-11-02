// Design: docs/features/interfaces.md -- interface rate show + monitor handlers
// Related: interface_rate.go -- the handlers registered here

package cmd

import (
	"github.com/ze-software/ze/internal/component/iface"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func init() {
	// The rates live in the iface plugin's tracker, so the stream and its
	// one-shot sample forward to the command that plugin serves.
	pluginserver.RegisterPluginStreamingHandler("monitor interface rate", pluginserver.OneShotSample,
		iface.RateCommand, streamInterfaceRate)
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod: "ze-iface:monitor-interface-rate",
			Handler:    handleMonitorInterfaceRate,
		},
	)
}
