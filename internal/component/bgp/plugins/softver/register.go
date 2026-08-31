package softver

import (
	"bytes"
	"fmt"
	"os"

	softveryang "github.com/ze-software/ze/internal/component/bgp/plugins/softver/yang"
	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/slogutil"
)

func init() {
	reg := registry.Registration{
		Name:            "bgp-softver",
		Description:     "Software Version capability (code 75)",
		RFCs:            []string{"draft-abraitis-bgp-version-capability"},
		SupportsCapa:    true,
		Features:        "capa yang",
		ConfigRoots:     []string{configRootBGP},
		Dependencies:    []string{configRootBGP},
		YANG:            softveryang.ZeSoftverYANG,
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
