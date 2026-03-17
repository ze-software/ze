// Design: docs/architecture/config/yang-config-design.md — editor mode RPCs

package cli

import (
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-editor:mode-command", Help: "Switch to operational command mode (or run <cmd> to execute)", ReadOnly: true},
		pluginserver.RPCRegistration{WireMethod: "ze-editor:mode-edit", Help: "Switch back to config edit mode", ReadOnly: true},
	)
}
