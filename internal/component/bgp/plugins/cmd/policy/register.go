// Design: docs/architecture/api/commands.md — BGP policy command registration

package policy

import (
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
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
