// Design: docs/architecture/api/commands.md — BGP policy command registration

package policy

import (
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:policy-chain",
			Handler:    handleShowPolicyChain,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:policy-test",
			Handler:    handleShowPolicyTest,
		},
	)
}
