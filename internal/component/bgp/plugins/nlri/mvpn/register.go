package mvpn

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
		Name:         "bgp-mvpn",
		Description:  "Multicast VPN family plugin (RFC 6514)",
		RFCs:         []string{"6514"},
		SupportsNLRI: true,
		Features:     "nlri",
		Families:     []string{"ipv4/mvpn", "ipv6/mvpn"},
		RunEngine:    RunMVPNPlugin,
		InProcessDecoder: func(input, output *bytes.Buffer) int {
			return RunDecode(input, output)
		},
		InProcessNLRIDecoder: DecodeNLRIHex,
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
			family = fs.String("family", "ipv4/mvpn", "Address family (ipv4/mvpn or ipv6/mvpn)")
		}
		cfg.RunCLIWithCtx = func(hex string, text bool, out, errOut io.Writer, _ *flag.FlagSet) int {
			return RunCLIDecode(hex, *family, text, out, errOut)
		}
		cfg.RunDecode = RunDecode
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		slog.Error("mvpn: registration failed", "error", err)
		os.Exit(1)
	}
}
