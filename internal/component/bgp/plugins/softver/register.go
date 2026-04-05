package softver

import (
	"bytes"
	"fmt"
	"os"

	softverschema "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/softver/schema"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

func init() {
	reg := registry.Registration{
		Name:            "bgp-softver",
		Description:     "Software Version capability (code 75)",
		RFCs:            []string{"draft-ietf-idr-software-version"},
		SupportsCapa:    true,
		Features:        "capa yang",
		ConfigRoots:     []string{"bgp"},
		Dependencies:    []string{"bgp"},
		YANG:            softverschema.ZeSoftverYANG,
		CapabilityCodes: []uint8{75},
		RunEngine:       RunSoftverPlugin,
		InProcessDecoder: func(input, output *bytes.Buffer) int {
			return RunDecodeMode(input, output)
		},
		ConfigureEngineLogger: func(loggerName string) {
			ConfigureLogger(slogutil.Logger(loggerName))
		},
	}
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.GetYANG = GetYANG
		cfg.ConfigLogger = func(level string) {
			ConfigureLogger(slogutil.PluginLogger(reg.Name, level))
		}
		cfg.RunCLIDecode = RunCLIDecode
		cfg.RunDecode = RunDecodeMode
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "softver: registration failed: %v\n", err)
		os.Exit(1)
	}
}
