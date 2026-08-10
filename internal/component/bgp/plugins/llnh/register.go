package llnh

import (
	"fmt"
	"os"

	llnhyang "github.com/ze-software/ze/internal/component/bgp/plugins/llnh/yang"
	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/slogutil"
)

func init() {
	reg := registry.Registration{
		Name:            "bgp-llnh",
		Description:     "Link-Local Next-Hop capability plugin",
		SupportsCapa:    true,
		Features:        "capa yang",
		ConfigRoots:     []string{"bgp"},
		Dependencies:    []string{"bgp"},
		YANG:            llnhyang.ZeLinkLocalNexthopYANG,
		CapabilityCodes: []uint8{77},
		RunEngine:       runLLNHPlugin,
		ConfigureEngineLogger: func(loggerName string) {
			setLLNHLogger(slogutil.Logger(loggerName))
		},
	}
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.GetYANG = getLLNHYANG
		cfg.ConfigLogger = func(level string) {
			setLLNHLogger(slogutil.PluginLogger(reg.Name, level))
		}
		cfg.RunCLIDecode = runLLNHCLIDecode
		cfg.RunDecode = runLLNHDecodeMode
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "llnh: registration failed: %v\n", err)
		os.Exit(1)
	}
}
