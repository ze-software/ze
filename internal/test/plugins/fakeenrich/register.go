package fakeenrich

import (
	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/show"
)

// marker is the value both enrichers write, and what the tests assert on.
// One spelling so a typo in either arm cannot make one enricher silently
// disagree with the other.
const marker = "present"

func init() {
	show.MustRegister(Command, "fakeenrich", show.Enricher{
		Detail: func(base map[string]any) { base["fakeenrich"] = marker },
		Brief:  func(base map[string]any) { base["fakeenrich"] = marker },
	})

	reg := registry.Registration{
		Name:        Name,
		Description: "Test-only in-process enricher (harmless when not invoked)",
		RunEngine:   runPlugin,
	}
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		panic("BUG: fakeenrich registration failed")
	}
}
