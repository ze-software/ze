// Fixture: development tooling registers nothing an operator reads.
package tool

import (
	"errors"

	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(pluginserver.RPCRegistration{WireMethod: "ze-tool:x", Handler: handleTool})
}

func handleTool() error { return errors.New("a tool handler") }
