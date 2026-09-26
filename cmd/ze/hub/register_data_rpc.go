// Design: docs/architecture/hub-architecture.md -- request data RPCs beside request reload.
// Related: data_rpc.go -- the handlers registered here.

package hub

import (
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-data:backup", Handler: handleDataBackup},
		pluginserver.RPCRegistration{WireMethod: "ze-data:restore", Handler: handleDataRestore},
	)
}
