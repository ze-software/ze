// Design: docs/plugin-development/protocol.md -- test-only plugin registration.
// Related: filter.go -- one evaluator for both plugin transports.
package filterwirecases

import (
	"net"

	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

func init() {
	// A filter reference names the registration, not its config block. The
	// two transports MUST have distinct registrations to run in one daemon.
	for _, name := range []string{filterWireCasesName, filterWireCasesName + "-forked"} {
		reg := registry.Registration{
			Name: name, Description: "Test-only field and set evaluator on both plugin transports",
			Filters:   filterWireCasesDecls(),
			RunEngine: func(conn net.Conn) int { return filterWireCasesRun(name, conn) },
		}
		reg.CLIHandler = func(args []string) int { return cli.RunPlugin(cli.BaseConfig(&reg), args) }
		if err := registry.Register(reg); err != nil {
			panic("BUG: filter-wire-cases registration failed: " + err.Error())
		}
	}
}
