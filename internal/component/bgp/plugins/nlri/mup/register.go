package mup

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlritype"
	"github.com/ze-software/ze/internal/core/slogutil"
)

func init() {
	// RFC 7606 Section 5.4: BGP-MUP is a typed address family and
	// draft-ietf-bess-mup-safi states no deviation, so a route whose architecture and
	// route type ze does not implement must be discarded. Registered here, beside the
	// families, so removing this plugin removes both the advertisement and the
	// obligation.
	for _, fam := range []Family{IPv4MUP, IPv6MUP} {
		if err := nlritype.Register(fam, RecognizeNLRI); err != nil {
			fmt.Fprintf(os.Stderr, "mup: RFC 7606 Section 5.4 recognizer registration failed: %v\n", err)
			os.Exit(1)
		}
	}

	reg := registry.Registration{
		Name:         "bgp-nlri-mup",
		Description:  "Mobile User Plane family plugin (draft-ietf-bess-mup-safi)",
		SupportsNLRI: true,
		Features:     "nlri",
		Families:     []string{"ipv4/mup", "ipv6/mup"},
		RunEngine:    runMUPPlugin,
		InProcessDecoder: func(input, output *bytes.Buffer) int {
			return RunDecode(input, output)
		},
		InProcessNLRIDecoder:       DecodeNLRIHex,
		InProcessNLRIEncoder:       EncodeNLRIHex,
		InProcessRouteEncoder:      EncodeRoute,
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
			family = fs.String("family", "ipv4/mup", "Address family (ipv4/mup or ipv6/mup)")
		}
		cfg.RunCLIWithCtx = func(hex string, text bool, out, errOut io.Writer, _ *flag.FlagSet) int {
			return RunCLIDecode(hex, *family, text, out, errOut)
		}
		cfg.RunDecode = RunDecode
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		slog.Error("mup: registration failed", "error", err)
		os.Exit(1)
	}
}
