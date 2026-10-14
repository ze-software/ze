package fakedrain

import (
	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

func init() {
	reg := registry.Registration{
		Name:        Name,
		Description: "Test-only drain responder for the stop drain functional tests (use ze.fakedrain)",
		RunEngine:   runPlugin,
	}
	reg.CLIHandler = func(args []string) int {
		return cli.RunPlugin(cli.BaseConfig(&reg), args)
	}
	if err := registry.Register(reg); err != nil {
		panic("BUG: " + Name + " registration failed: " + err.Error())
	}
}
