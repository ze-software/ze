package labeled

import (
	"bytes"
	"flag"
	"io"
	"log/slog"
	"os"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

func init() {
	reg := registry.Registration{
		Name:                  "bgp-nlri-labeled",
		Description:           "Labeled Unicast family plugin (RFC 8277)",
		RFCs:                  []string{"8277"},
		Features:              "nlri",
		SupportsNLRI:          true,
		Families:              []string{"ipv4/mpls-label", "ipv6/mpls-label"},
		RunEngine:             RunLabeledPlugin,
		InProcessNLRIDecoder:  DecodeNLRIHex,
		InProcessNLRIEncoder:  EncodeNLRIHex,
		InProcessRouteEncoder: EncodeRoute,
		InProcessDecoder: func(input, output *bytes.Buffer) int {
			return RunDecode(input, output)
		},
		ConfigureEngineLogger: func(loggerName string) {
			SetLogger(slogutil.Logger(loggerName))
		},
	}
	reg.CLIHandler = func(args []string) int {
		var family *string
		cfg := cli.BaseConfig(&reg)
		cfg.ConfigLogger = func(level string) {
			SetLogger(slogutil.PluginLogger(reg.Name, level))
		}
		cfg.ExtraFlags = func(fs *flag.FlagSet) {
			family = fs.String("family", "ipv4/mpls-label", "Address family (ipv4/mpls-label, ipv6/mpls-label)")
		}
		cfg.RunCLIWithCtx = func(hex string, text bool, out, errOut io.Writer, fs *flag.FlagSet) int {
			return RunCLIDecode(hex, *family, text, out, errOut)
		}
		cfg.RunDecode = RunDecode
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		slog.Error("labeled: registration failed", "error", err)
		os.Exit(1)
	}
}
