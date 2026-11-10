// Fixture: the registrations that make the handlers operator boundaries.
package fixture

import (
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-fixture:peer", Handler: handlePeer},
	)
	registry.Register(registry.Registration{
		Name:                    "fixture",
		InProcessConfigVerifier: verifyFixture,
	})
}
