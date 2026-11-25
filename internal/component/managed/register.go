// Design: ai/rules/repo-maintenance.md -- managed hub reachability check registration
// Design: docs/architecture/api/commands.md -- show fleet status, request fleet disable and enable registration
// Design: docs/architecture/fleet-config.md -- the hub's fleet divergence commands
// Related: doctor.go -- the hub-unreachable reachability check registered here
// Related: cmd_fleet.go -- the three fleet command handlers registered here
// Related: cmd_divergence.go -- the four fleet divergence command handlers registered here
// Related: cmd_devices.go -- the hub device registry commands

package managed

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/audit"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// commandModeOffline is the registry mode of the fleet commands: each one
// answers in the caller's own process when no daemon does.
const commandModeOffline = "offline"

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-managed:fleet-devices", Handler: handleFleetDevices},
		pluginserver.RPCRegistration{WireMethod: "ze-managed:fleet-device", Handler: handleFleetDevice, Complete: completeFleetDevices},
	)
	command.RegisterShape([]string{"show fleet devices"}, command.ShapeTab)
	command.RegisterShape([]string{"show fleet device name"}, command.ShapeDoc)
	rowColumns := command.ColumnOrder{
		"name", "connected", "config-state", "labels", "hub-version", "device-version", "last-fetch", "last-fetch-result",
		"connected-since", "last-seen", "source-address", "ever-connected", "diverged", "removed",
	}
	command.RegisterColumns([]string{"show fleet devices"}, rowColumns)
	command.RegisterColumns([]string{"show fleet device name"}, append(rowColumns,
		"first-connected", "last-ack", "ack-version", "ack-result", "ack-error", "last-fetch-error", "persisted", "unreadable", "removed-at"))
	command.RegisterPipeFilters([]string{"show fleet devices"}, command.PipeFilter{
		Name: "label", Description: "Select devices with this key=value label", Argument: command.ArgText,
	})

	for _, check := range []diagnostic.DoctorCheck{hubDoctorCheck, hubClientDoctorCheck, managedDivergedDoctorCheck, hubDivergedDoctorCheck, hubSessionDoctorCheck} {
		if err := diagnostic.RegisterDoctorCheck(check); err != nil {
			panic("BUG: managed doctor check registration refused: " + err.Error())
		}
	}

	// `show fleet status` answers in the caller's own process from the store,
	// and the daemon serves the same handler under the same path, so it reads
	// the store on every surface. Its node is in yang/ze-managed-cmd.yang.
	command.MustRegisterLocalData("show fleet status", answerFleetStatus, registry.Meta{
		ShortHelp: "Whether this device follows a hub, and how its configuration compares with the hub's last one.",
		Mode:      commandModeOffline,
	}, command.RenderLocalAnswer)
	// The answer is ONE document, so the row operators are refused over it by name.
	command.RegisterShape([]string{"show fleet status"}, command.ShapeDoc)
	command.RegisterColumns([]string{"show fleet status"}, command.ColumnOrder{
		"managed", "frozen", "hub-declared", "hub-name", "hub-address",
		"baseline-recorded", "baseline-hash", "active-recorded", "active-hash", "diverged",
		"daemon-answered", "client", "connection", "holding", "held-by-hub", "revert-offered",
	})

	// `request fleet disable` and `request fleet enable` change the managed
	// flag and the daemon's hub client, so the daemon answers them as builtin
	// RPCs, each recording a change of a store key. They are plain local
	// commands beside that, not local-data ones: `ze cli` is the router's CLI
	// and asks the daemon, while `ze request fleet ...` at a shell writes the
	// flag itself when no daemon holds the store. Their nodes are in
	// yang/ze-managed-cmd.yang.
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-managed:fleet-disable", Handler: handleFleetDisable, AuditAction: audit.ActionDataKeyChange},
		pluginserver.RPCRegistration{WireMethod: "ze-managed:fleet-enable", Handler: handleFleetEnable, AuditAction: audit.ActionDataKeyChange},
	)
	command.MustRegisterLocalMeta("request fleet disable", requestFleetDisableLocal, registry.Meta{
		ShortHelp: "Leave the fleet: end the hub connection and stop following the hub.",
		Mode:      commandModeOffline,
	})
	command.MustRegisterLocalMeta("request fleet enable", requestFleetEnableLocal, registry.Meta{
		ShortHelp: "Rejoin the fleet: follow the hub again.",
		Mode:      commandModeOffline,
	})

	// The hub's two divergence views read the held copies from the store, so
	// they answer in the caller's own process and the daemon serves the same
	// handlers under the same paths, as `show fleet status` does. The paths
	// are literals: the command-contract gate reads them from this file.
	command.MustRegisterLocalData("show fleet divergence list", answerDivergenceList, registry.Meta{
		ShortHelp: "The managed clients that hold, and what the hub keeps for each.",
		Mode:      commandModeOffline,
	}, command.RenderLocalAnswer)
	command.MustRegisterLocalData("show fleet divergence name", answerDivergenceShow, registry.Meta{
		ShortHelp: "What differs for a managed client that holds.",
		Mode:      commandModeOffline,
	}, command.RenderLocalAnswer)
	command.RegisterShape([]string{"show fleet divergence list"}, command.ShapeTab)
	command.RegisterShape([]string{"show fleet divergence name"}, command.ShapeDoc)
	command.RegisterColumns([]string{"show fleet divergence list"}, command.ColumnOrder{
		"client", "declared", "device-version", "hub-version", "edited-at", "received-at", "revert", "unreadable",
	})
	// The daemon serves the show of one client as a builtin over the same
	// handler, as `show doctor` serves `ze doctor`, so that the show declares
	// the completer of its client name: the clients whose held copy the
	// daemon's store keeps. A local-data path declares no completer.
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-managed:fleet-divergence-show", Handler: handleDivergenceShow, Complete: completeDivergenceClients},
	)

	// Adopt changes what the hub serves a client and revert what it offers
	// one, so the daemon answers both as builtin RPCs, each recorded under its
	// own audit action, and the local route sends the line to the daemon that
	// owns the store. Their nodes are in yang/ze-managed-cmd.yang.
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-managed:fleet-divergence-adopt", Handler: handleDivergenceAdopt, AuditAction: audit.ActionFleetDivergenceAdopt},
		pluginserver.RPCRegistration{WireMethod: "ze-managed:fleet-divergence-revert", Handler: handleDivergenceRevert, AuditAction: audit.ActionFleetDivergenceRevert},
	)
	command.MustRegisterLocalMeta("update fleet divergence name adopt", updateDivergenceAdoptLocal, registry.Meta{
		ShortHelp: "Make the configuration a client holds the hub's configuration for it.",
		Mode:      commandModeOffline,
	})
	command.MustRegisterLocalMeta("request fleet divergence name revert", requestDivergenceRevertLocal, registry.Meta{
		ShortHelp: "Make a client that holds take the hub's configuration.",
		Mode:      commandModeOffline,
	})
}
