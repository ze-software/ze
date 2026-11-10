// Fixture: the registrations the operator-error scan derives its boundaries from.
package fixture

import (
	"encoding/json"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

const wireTyped = "ze-fixture:typed"

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-fixture:plain", Handler: handlePlain},
		pluginserver.RPCRegistration{WireMethod: wireTyped, Handler: handleTyped},
		pluginserver.RPCRegistration{WireMethod: "ze-fixture:pass", Handler: handlePassThrough},
		pluginserver.RPCRegistration{WireMethod: "ze-fixture:factory", Handler: forward("show fixture")},
		pluginserver.RPCRegistration{WireMethod: "ze-fixture:opaque", Handler: handlers[0]},
	)
	registry.Register(registry.Registration{
		Name:                    "fixture",
		InProcessConfigVerifier: verifyFixture,
		RPCHandlers: map[string]func(json.RawMessage) (any, error){
			"ze-fixture:rpc": rpcFixture,
		},
	})
}
