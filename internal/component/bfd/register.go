package bfd

import (
	"fmt"
	"os"

	bfdyang "codeberg.org/thomas-mangin/ze/internal/component/bfd/yang"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

func init() {
	reg := registry.Registration{
		Name:                    "bfd",
		Description:             "Bidirectional Forwarding Detection (RFC 5880, 5881, 5883)",
		Features:                "yang",
		RFCs:                    []string{"5880", "5881", "5882", "5883"},
		ConfigRoots:             []string{"bfd"},
		YANG:                    bfdyang.ZeBFDConfYANG,
		InProcessConfigVerifier: verifyBFDConfig,
		RunEngine:               RunBFDPlugin,
		ConfigureEngineLogger: func(loggerName string) {
			UseLogger(slogutil.Logger(loggerName))
		},
		ConfigureMetrics: func(r metrics.Registry) {
			bindMetricsRegistry(r)
		},
	}
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.ConfigLogger = func(level string) {
			UseLogger(slogutil.PluginLogger(reg.Name, level))
		}
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "bfd: registration failed: %v\n", err)
		os.Exit(1)
	}
}
