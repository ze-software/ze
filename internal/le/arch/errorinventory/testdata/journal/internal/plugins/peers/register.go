// Fixture: the registrations that make the answering handlers boundaries.
package peers

import (
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-peers:before", Handler: answerBefore},
		pluginserver.RPCRegistration{WireMethod: "ze-peers:after", Handler: answerAfter},
	)
}
