package gr

import (
	"bytes"
	"fmt"
	"os"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
	grschema "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/gr/schema"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

func init() {
	// Register LLGR well-known community names (RFC 9494).
	for _, c := range []struct {
		value attribute.Community
		name  string
	}{
		{attribute.CommunityLLGRStale, "LLGR_STALE"},
		{attribute.CommunityNoLLGR, "NO_LLGR"},
	} {
		if err := attribute.RegisterCommunityName(c.value, c.name); err != nil {
			logger().Error("community registration failed", "error", err)
		}
	}

	reg := registry.Registration{
		Name:            "bgp-gr",
		Description:     "Graceful Restart capability and mechanism plugin",
		RFCs:            []string{"4724", "9494"},
		SupportsCapa:    true,
		Features:        "capa yang",
		ConfigRoots:     []string{"bgp"},
		YANG:            grschema.ZeGracefulRestartYANG,
		CapabilityCodes: []uint8{64, 71},
		Dependencies:    []string{"bgp-rib"},
		RunEngine:       RunGRPlugin,
		InProcessDecoder: func(input, output *bytes.Buffer) int {
			return RunDecodeMode(input, output)
		},
		ConfigureEngineLogger: func(loggerName string) {
			SetLogger(slogutil.Logger(loggerName))
		},
		ConfigureMetrics: func(reg any) {
			if r, ok := reg.(metrics.Registry); ok {
				SetMetricsRegistry(r)
			}
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
