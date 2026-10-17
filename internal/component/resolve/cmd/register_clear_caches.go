// Design: docs/architecture/resolve.md -- Clearing a resolver cache
// Related: clear_caches.go -- the handlers registered here

package cmd

import (
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-resolve:clear-peeringdb-cache-all", Handler: handleClearPeeringDBCacheAll},
		pluginserver.RPCRegistration{WireMethod: "ze-resolve:clear-peeringdb-cache-asn", Handler: handleClearPeeringDBCacheASN},
		pluginserver.RPCRegistration{WireMethod: "ze-resolve:clear-irr-cache-all", Handler: handleClearIRRCacheAll},
		pluginserver.RPCRegistration{WireMethod: "ze-resolve:clear-irr-cache-as-set", Handler: handleClearIRRCacheASSet},
		pluginserver.RPCRegistration{WireMethod: "ze-resolve:clear-cymru-cache-all", Handler: handleClearCymruCacheAll},
		pluginserver.RPCRegistration{WireMethod: "ze-resolve:clear-cymru-cache-address", Handler: handleClearCymruCacheAddress},
	)
}
