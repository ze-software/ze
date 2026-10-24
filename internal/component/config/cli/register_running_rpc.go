// Design: docs/architecture/api/commands.md -- show config running, the configuration the daemon applied
// Related: running_rpc.go -- the handler registered here.

package cli

import (
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// `show config running` is the one `show config` command the DAEMON answers.
// Every other one reads a file or the store in the caller's own process
// (register.go), so none of them can say what a running daemon applied: after a
// refused reload the file already holds the refused text.
func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-config:show-config-running", Handler: handleShowConfigRunning},
	)
}
