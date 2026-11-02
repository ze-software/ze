package cmd

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod: "ze-update:show-system-update",
			Handler:    handleShowSystemUpdate,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-update:show-system-update-history",
			Handler:    handleShowSystemUpdateHistory,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-update:system-firmware-check",
			Handler:    handleFirmwareCheck,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-update:system-firmware-download",
			Handler:    handleFirmwareDownload,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-update:system-firmware-apply",
			Handler:    handleFirmwareApply,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-update:system-firmware-restart",
			Handler:    handleFirmwareRestart,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-update:system-firmware-rollback",
			Handler:    handleFirmwareRollback,
		},
		// The daemon form of `update serve`: a server with a lifecycle under the
		// daemon (serve_daemon.go). They are daemon commands, never local-data:
		// `ze cli -c` answers a local-data command in its own process, and these
		// act on the daemon's server.
		pluginserver.RPCRegistration{
			WireMethod: "ze-update:update-serve-start",
			Handler:    handleUpdateServeStart,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-update:update-serve-stop",
			Handler:    handleUpdateServeStop,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-update:update-serve-pause",
			Handler:    handleUpdateServePause,
		},
	)

	// `update` is a YANG verb, so a root handler named `update` would be
	// unreachable behind the isYANGVerb branch in zeDispatch. RunCommand consults
	// the local-handler registry before the YANG tree, so the foreground
	// `ze update serve` is a local meta. A deeper declared path (`update serve
	// start`) still reaches the daemon: command.LookupLocal yields to it.
	command.MustRegisterLocalMeta("update serve", runUpdateServe, registry.Meta{
		ShortHelp: "Serve this binary and its version manifest for update checks.",
		Description: "The server answers a version manifest, the running binary and its SHA-256 digest. " +
			"It is meant for build infrastructure rather than for a router in production.",
		Mode: "offline",
	})
}
