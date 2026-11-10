// Design: docs/architecture/plugin/plugin-system.md -- the registry seals after start-up
// Related: lateregister.go -- the engine that attempts the late registration

package lateregister

import (
	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// init registers the plugin during package initialization, the one time a
// registration is allowed. Its success is half of what
// test/plugin/registry-sealed-after-start.ci proves: start-up registration
// still works while the seal refuses the engine's later call.
func init() {
	reg := registry.Registration{
		Name:        Name,
		Description: "Test-only plugin whose engine registers after start-up, to prove the seal refuses it (use ze.lateregister)",
		RunEngine:   runPlugin,
	}
	reg.CLIHandler = func(args []string) int {
		return cli.RunPlugin(cli.BaseConfig(&reg), args)
	}
	if err := registry.Register(reg); err != nil {
		panic("BUG: " + Name + " registration failed: " + err.Error())
	}
}
