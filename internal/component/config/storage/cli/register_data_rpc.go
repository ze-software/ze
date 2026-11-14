// Design: docs/architecture/hub-architecture.md -- request data RPCs beside request reload.
// Related: data_rpc.go -- the handlers registered here.

package cli

import (
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/audit"
)

// The key removal carries no RequiresSelector. That guard is the peer
// selector's: it waits for a value bound to the leaf named `selector`, which
// this command never declares. The mandatory `key` leaf in ze-data-cmd.yang
// is its guard: command.ValidateArgs binds the lone value typed after `key`
// and refuses a line without one before the handler runs, as it does for
// `delete interface name <name>`.
func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-config:data-backup", Handler: handleDataBackup},
		pluginserver.RPCRegistration{WireMethod: "ze-config:data-restore", Handler: handleDataRestore},
		pluginserver.RPCRegistration{WireMethod: "ze-config:data-key-delete", Handler: handleDataKeyDelete, AuditAction: audit.ActionDataKeyChange},
		// A salvage writes a new store on the daemon's host and changes nothing
		// in the live one, as a backup writes a file, so like the backup it
		// declares no audit action.
		pluginserver.RPCRegistration{WireMethod: "ze-config:data-repair", Handler: handleDataRepair},
	)
}
