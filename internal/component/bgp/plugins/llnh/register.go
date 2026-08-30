package llnh

import (
	"fmt"
	"os"

	llnhyang "github.com/ze-software/ze/internal/component/bgp/plugins/llnh/yang"
	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// The BGP daemon appears twice under one spelling: as the config subtree this
// plugin reads, and as the plugin it depends on. Each meaning gets its name.
const (
	configRootBGP = "bgp"
	pluginNameBGP = "bgp"
)

func init() {
	reg := registry.Registration{
		Name:            "bgp-llnh",
		Description:     "Link-Local Next-Hop capability plugin",
		SupportsCapa:    true,
		Features:        "capa yang",
		ConfigRoots:     []string{configRootBGP},
		Dependencies:    []string{pluginNameBGP},
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
