// Design: docs/architecture/diagnostics/debug-filtering.md -- debug RPC command registration

package cmd

import (
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(RPCs()...)
}
