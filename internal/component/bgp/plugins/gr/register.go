package bgp_gr

import (
	"bytes"
	"fmt"
	"os"

	grschema "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/bgp-gr/schema"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

func init() {
	reg := registry.Registration{
		Name:            "bgp-gr",
		Description:     "Graceful Restart capability and mechanism plugin",
		RFCs:            []string{"4724"},
		SupportsCapa:    true,
		Features:        "capa yang",
		ConfigRoots:     []string{"bgp"},
		YANG:            grschema.ZeGracefulRestartYANG,
		CapabilityCodes: []uint8{64},
		Dependencies:    []string{"bgp-rib"},
		RunEngine:       RunGRPlugin,
		InProcessDecoder: func(input, output *bytes.Buffer) int {
			return RunDecodeMode(input, output)
		},
		ConfigureEngineLogger: func(loggerName string) {
			SetLogger(slogutil.Logger(loggerName))
		},
	}
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.GetYANG = GetYANG
		cfg.ConfigLogger = func(level string) {
			SetLogger(slogutil.PluginLogger(reg.Name, level))
		}
		cfg.RunCLIDecode = RunCLIDecode
		cfg.RunDecode = RunDecodeMode
		cfg.RunEngine = RunGRPlugin
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "gr: registration failed: %v\n", err)
		os.Exit(1)
	}
}
