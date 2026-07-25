// Design: docs/architecture/api/commands.md — log command registration

package cmd

import (
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(RPCs()...)
}
