package evpn

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlritype"
	"github.com/ze-software/ze/internal/core/slogutil"
)

func init() {
	// RFC 7606 Section 5.4: EVPN is a typed address family and RFC 7432 states no
	// deviation, so a route whose type ze does not implement must be discarded.
	// Registered here, beside the family, so removing this plugin removes both the
	// advertisement and the obligation.
	if err := nlritype.Register(L2VPNEVPN, RecognizeNLRI); err != nil {
		fmt.Fprintf(os.Stderr, "evpn: RFC 7606 Section 5.4 recognizer registration failed: %v\n", err)
		os.Exit(1)
	}

	reg := registry.Registration{
		Name:         "bgp-nlri-evpn",
		Description:  "EVPN family plugin",
		RFCs:         []string{"7432", "9136"},
		SupportsNLRI: true,
		Features:     "nlri",
		Families:     []string{familyNameEVPN},
		RunEngine:    runEVPNPlugin,
		ConfigureEngineLogger: func(loggerName string) {
			setEVPNLogger(slogutil.Logger(loggerName))
		},
		InProcessDecoder: func(input, output *bytes.Buffer) int {
			return runEVPNDecode(input, output)
		},
		InProcessNLRIDecoder:  DecodeNLRIHex,
		InProcessNLRIEncoder:  EncodeNLRIHex,
		InProcessRouteEncoder: EncodeRoute,
	}
	reg.CLIHandler = func(args []string) int {
		var family *string
		cfg := cli.BaseConfig(&reg)
		cfg.GetYANG = getEVPNYANG
		cfg.ConfigLogger = func(level string) {
			setEVPNLogger(slogutil.PluginLogger(reg.Name, level))
		}
		cfg.ExtraFlags = func(fs *flag.FlagSet) {
			family = fs.String("family", familyNameEVPN, "Address family (l2vpn/evpn)")
		}
		cfg.RunCLIWithCtx = func(hex string, text bool, out, errOut io.Writer, fs *flag.FlagSet) int {
			return RunCLIDecode(hex, *family, text, out, errOut)
		}
		cfg.RunDecode = runEVPNDecode
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "evpn: registration failed: %v\n", err)
		os.Exit(1)
	}
}
