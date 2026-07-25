package cmd

import (
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:system-update",
			Handler:    handleShowSystemUpdate,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:system-update-history",
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
	)
}
