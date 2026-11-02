// Design: docs/features/interfaces.md -- interface rate show + monitor handlers
// Related: interface_rate.go -- the handlers registered here

package cmd

import (
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterStreamingHandler("monitor interface rate", pluginserver.OneShotSample, streamInterfaceRate)
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod: "ze-iface:monitor-interface-rate",
			Handler:    handleMonitorInterfaceRate,
		},
	)
}
