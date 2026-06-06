// Design: docs/architecture/api/commands.md — log command registration

package cmd

import (
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(RPCs()...)
}
