package mvpn

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlritype"
	"github.com/ze-software/ze/internal/core/slogutil"
)

func init() {
	// RFC 6514: Register PMSI Tunnel attribute (type 22).
	attribute.RegisterName(22, "PMSI_TUNNEL")

	// RFC 7606 Section 5.4: MCAST-VPN is one of the section's own examples of a typed
	// address family, and RFC 6514 states no deviation, so a route whose type ze does
	// not implement must be discarded. Registered here, beside the families, so
	// removing this plugin removes both the advertisement and the obligation.
	for _, fam := range []Family{IPv4MVPN, IPv6MVPN} {
		if err := nlritype.Register(fam, RecognizeNLRI); err != nil {
			fmt.Fprintf(os.Stderr, "mvpn: RFC 7606 Section 5.4 recognizer registration failed: %v\n", err)
			os.Exit(1)
		}
	}

	reg := registry.Registration{
		Name:         "bgp-nlri-mvpn",
		Description:  "Multicast VPN family plugin (RFC 6514)",
		RFCs:         []string{"6514"},
		SupportsNLRI: true,
		Features:     "nlri",
		Families:     []string{"ipv4/mvpn", "ipv6/mvpn"},
		RunEngine:    runMVPNPlugin,
		InProcessDecoder: func(input, output *bytes.Buffer) int {
			return RunDecode(input, output)
		},
		InProcessNLRIDecoder:       DecodeNLRIHex,
		InProcessConfigRouteParser: parseConfigRoute,
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
