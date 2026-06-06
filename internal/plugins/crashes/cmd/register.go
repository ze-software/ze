// Design: docs/architecture/core-design.md — plugin self-containment carve-out

package cmd

import (
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:crashes",
			Handler: func(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
				return HandleShowCrashes(args)
			},
		},
	)
}
