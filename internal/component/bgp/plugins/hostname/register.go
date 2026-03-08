package hostname

import (
	"bytes"
	"fmt"
	"os"

	hostnameschema "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/hostname/schema"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

func init() {
	reg := registry.Registration{
		Name:            "bgp-hostname",
		Description:     "FQDN capability decoding",
		RFCs:            []string{"draft-walton-bgp-hostname-capability"},
		SupportsCapa:    true,
		Features:        "capa yang",
		ConfigRoots:     []string{"bgp"},
		YANG:            hostnameschema.ZeHostnameYANG,
		CapabilityCodes: []uint8{73},
		RunEngine:       RunHostnamePlugin,
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
		fmt.Fprintf(os.Stderr, "hostname: registration failed: %v\n", err)
		os.Exit(1)
	}
}
