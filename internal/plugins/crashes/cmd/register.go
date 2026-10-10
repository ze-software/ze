// Design: docs/architecture/core-design.md — plugin self-containment carve-out

package cmd

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod: "ze-crashes:show-crashes",
			Handler: func(_ *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
				args := validated.Tokens()
				return handleShowCrashes(args)
			},
		},
	)
}
