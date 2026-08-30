package bfd

import (
	"fmt"
	"os"

	bfdyang "github.com/ze-software/ze/internal/component/bfd/yang"
	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// pluginName is the name this plugin registers under, and configRoot is the
// top-level YANG container it owns. The two hold the same text and name two
// different things: a rename of one does not rename the other.
const (
	pluginName = "bfd"
	configRoot = "bfd"
)

func init() {
	reg := registry.Registration{
		Name:                    pluginName,
		Description:             "Bidirectional Forwarding Detection (RFC 5880, 5881, 5883)",
		Features:                "yang",
		RFCs:                    []string{"5880", "5881", "5882", "5883"},
		ConfigRoots:             []string{configRoot},
		YANG:                    bfdyang.ZeBFDConfYANG,
		InProcessConfigVerifier: verifyBFDConfig,
		RunEngine:               RunBFDPlugin,
		ConfigureEngineLogger: func(loggerName string) {
			useLogger(slogutil.Logger(loggerName))
		},
		ConfigureMetrics: func(r metrics.Registry) {
			bindMetricsRegistry(r)
		},
	}
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.ConfigLogger = func(level string) {
			useLogger(slogutil.PluginLogger(reg.Name, level))
		}
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "bfd: registration failed: %v\n", err)
		os.Exit(1)
	}
}
