// Design: docs/architecture/config/yang-config-design.md — editor mode RPCs

package cli

import (
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-editor:mode-command"},
		pluginserver.RPCRegistration{WireMethod: "ze-editor:mode-edit"},
	)
}
