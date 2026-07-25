package fakeenrich

import (
	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/show"
)

func init() {
	show.MustRegister(Command, "fakeenrich", show.Enricher{
		Detail: func(base map[string]any) { base["fakeenrich"] = "present" },
		Brief:  func(base map[string]any) { base["fakeenrich"] = "present" },
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
