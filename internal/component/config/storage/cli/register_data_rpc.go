// Design: docs/architecture/hub-architecture.md -- request data RPCs beside request reload.
// Related: data_rpc.go -- the handlers registered here.

package cli

import (
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/audit"
)

// The two key commands carry no RequiresSelector. That guard is the peer
// selector's: it waits for a value bound to the leaf named `selector`, which
// neither command declares. The mandatory `key` leaf in ze-data-cmd.yang is
// their guard: command.ValidateArgs binds the value typed after `key` and
// refuses a line without one before the handler runs, as it does for
// `delete interface name <name>`. Both change store state, so both declare the
// audit action; the content of a write reaches the entry as the placeholder.
func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-config:data-backup", Handler: handleDataBackup},
		pluginserver.RPCRegistration{WireMethod: "ze-config:data-restore", Handler: handleDataRestore},
		pluginserver.RPCRegistration{WireMethod: "ze-config:data-key-delete", Handler: handleDataKeyDelete, AuditAction: audit.ActionDataKeyChange},
		pluginserver.RPCRegistration{WireMethod: "ze-config:data-key-update", Handler: handleDataKeyUpdate, AuditAction: audit.ActionDataKeyChange},
		// A config import commits a new version, and for the daemon's own
		// config runs the reload, so it declares its own audit action; the
		// content reaches the entry as the placeholder. Its mandatory `name`
		// leaf is its guard, as `key` is for the key commands.
		pluginserver.RPCRegistration{WireMethod: "ze-config:data-config-update", Handler: handleDataConfigUpdate, AuditAction: audit.ActionDataConfigImport},
		// A salvage writes a new store on the daemon's host and changes nothing
		// in the live one, as a backup writes a file, so like the backup it
		// declares no audit action.
		pluginserver.RPCRegistration{WireMethod: "ze-config:data-repair", Handler: handleDataRepair},
	)
}
