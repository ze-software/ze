package fixture

import (
	"errors"

	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(pluginserver.RPCRegistration{WireMethod: "ze-fixture:test", Handler: handleTest})
}

func handleTest() error { return errors.New("a test handler") }
