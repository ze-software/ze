// Overview: doc.go -- package doc + schema import
//
// register.go wires the ping module into ze's two registries from init():
//   - the plugin server RPC registry, for the daemon-side show/monitor/resolve
//     ping handlers, and
//   - the command registry, for the offline `ze ping` root command.
//
// The module is reached by the daemon through scripts/codegen/plugin_imports.go
// rpcDirs (internal/component/ping/cmd) and by the `ze` binary through plugin/all.

package cmd

import (
	"codeberg.org/thomas-mangin/ze/internal/component/command/registry"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-show:ping", Handler: handleShowPing},
		pluginserver.RPCRegistration{WireMethod: "ze-monitor:ping", Handler: handleMonitorPing},
		pluginserver.RPCRegistration{WireMethod: "ze-resolve:ping", Handler: handleResolvePing},
	)

	// Owner-backed offline root: `ze ping <target>` shells out to the OS ping
	// tool. RunPing has the LocalHandler shape (no RuntimeContext needed), so it
	// is wrapped to satisfy the RootHandler signature.
	registry.MustRegisterRootHandler("ping", func(_ *registry.RuntimeContext, args []string) int {
		return RunPing(args)
	}, registry.Meta{
		Description: "Ping a target from this box (offline, internal ICMP). Use --count N and --source IP to control the test.",
		Mode:        "offline",
		Section:     registry.SectionSystem,
		Subs:        "--count N, --source IP",
	})
}
