// Overview: doc.go -- package doc + schema import
//
// register.go wires the traceroute module into the plugin server RPC registry
// from init(): the daemon-side show/probe-round/monitor/resolve traceroute
// handlers. Unlike ping, traceroute has no offline `ze traceroute` root (the
// router-side trace is reached through `show traceroute`), so no command
// registry root is registered here.
//
// The module is reached by the daemon through scripts/codegen/plugin_imports.go
// rpcDirs (internal/component/traceroute/cmd) and by the `ze` binary through
// plugin/all.

package cmd

import (
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-show:traceroute", Handler: handleTraceroute},
		pluginserver.RPCRegistration{WireMethod: "ze-show:probe-round", Handler: HandleProbeRound},
		pluginserver.RPCRegistration{WireMethod: "ze-monitor:traceroute", Handler: handleMonitorTraceroute},
		pluginserver.RPCRegistration{WireMethod: "ze-resolve:traceroute", Handler: handleResolveTraceroute},
	)
}
