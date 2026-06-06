// Design: docs/architecture/api/commands.md — doctor command registration

package cmd

import (
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:doctor",
			Handler:    HandleShowDoctor,
		},
	)
}
