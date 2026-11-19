// Design: ai/rules/repo-maintenance.md -- managed hub reachability check registration
// Design: docs/architecture/api/commands.md -- show fleet status, request fleet disable and enable registration
// Related: doctor.go -- the hub-unreachable reachability check registered here
// Related: cmd_fleet.go -- the three fleet command handlers registered here

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
	for _, check := range []diagnostic.DoctorCheck{hubDoctorCheck, hubClientDoctorCheck, managedDivergedDoctorCheck} {
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
		"daemon-answered", "client", "connection", "holding",
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
}
