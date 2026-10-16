// Design: docs/architecture/pool-architecture.md -- pool-stats, the operator view of the attribute pools

package poolcmd

import (
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:pool-stats", Handler: handlePoolStats},
	)
}
