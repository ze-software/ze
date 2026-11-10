// Fixture: the registrations that make the answering handlers boundaries.
package answers

import (
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-answers:typed", Handler: answerTyped},
		pluginserver.RPCRegistration{WireMethod: "ze-answers:plain", Handler: answerPlain},
		pluginserver.RPCRegistration{WireMethod: "ze-answers:pass", Handler: answerPassThrough},
		pluginserver.RPCRegistration{WireMethod: "ze-answers:nested", Handler: answerNested},
	)
}
