// Fixture: a registration built in a loop, its wire method computed.
package fixture

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func registerLoop(names []string) {
	regs := make([]pluginserver.RPCRegistration, 0, len(names))
	for _, name := range names {
		section := name
		regs = append(regs, pluginserver.RPCRegistration{
			WireMethod: "ze-fixture:loop-" + section,
			Handler: func(_ *pluginserver.CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
				return dispatchSection(section)
			},
		})
	}
	pluginserver.RegisterRPCs(regs...)
}

func dispatchSection(_ string) (*plugin.Response, error) {
	return nil, nil
}
