package llnh

import (
	"fmt"
	"os"

	"codeberg.org/thomas-mangin/ze/internal/plugin/cli"
	llnhschema "codeberg.org/thomas-mangin/ze/internal/plugin/llnh/schema"
	"codeberg.org/thomas-mangin/ze/internal/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/slogutil"
)

func init() {
	reg := registry.Registration{
		Name:            "llnh",
		Description:     "Link-Local Next-Hop capability plugin",
		SupportsCapa:    true,
		Features:        "capa yang",
		ConfigRoots:     []string{"bgp"},
		YANG:            llnhschema.ZeLinkLocalNexthopYANG,
		CapabilityCodes: []uint8{77},
		RunEngine:       RunLLNHPlugin,
		ConfigureEngineLogger: func(loggerName string) {
			SetLLNHLogger(slogutil.Logger(loggerName))
		},
	}
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.GetYANG = GetLLNHYANG
		cfg.ConfigLogger = func(level string) {
			SetLLNHLogger(slogutil.PluginLogger(reg.Name, level))
		}
		cfg.RunCLIDecode = RunLLNHCLIDecode
		cfg.RunDecode = RunLLNHDecodeMode
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "llnh: registration failed: %v\n", err)
		os.Exit(1)
	}
}
