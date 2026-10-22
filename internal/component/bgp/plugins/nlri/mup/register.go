package mup

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlritype"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
)

func init() {
	// RFC 7606 Section 5.4: BGP-MUP is a typed address family and
	// draft-ietf-bess-mup-safi states no deviation, so a route whose architecture and
	// route type ze does not implement must be discarded. draft-ietf-bess-mup-safi
	// Section 3.1.3.1: a malformed route is treated as withdrawn (recognizerFor), so
	// the recognizer judges the body and is registered as one that does. Registered
	// here, beside the families, so removing this plugin removes both the
	// advertisement and the obligation.
	for _, fam := range []Family{IPv4MUP, IPv6MUP} {
		if err := nlritype.RegisterBodyRecognizer(fam, recognizerFor(fam.AFI)); err != nil {
			fmt.Fprintf(os.Stderr, "mup: RFC 7606 Section 5.4 recognizer registration failed: %v\n", err)
			os.Exit(1)
		}
		// draft-ietf-bess-mup-safi Section 3.3.6: a DSD route announced with no
		// Prefix-SID attribute is treated as withdrawn (CheckDSDPrefixSID).
		if err := nlritype.RegisterUpdateCheck(fam, CheckDSDPrefixSID); err != nil {
			fmt.Fprintf(os.Stderr, "mup: draft-ietf-bess-mup-safi Section 3.3.6 check registration failed: %v\n", err)
			os.Exit(1)
		}
		// draft-ietf-bess-mup-safi Section 3.1.3.1: a -02 Type 1 ST route, read
		// without its Source Address Length, is held and relayed with the octet.
		canonical := nlritype.Canonical{Grow: canonicalT1STGrow, WriteTo: writeCanonicalT1ST}
		if err := nlritype.RegisterCanonical(fam, canonical); err != nil {
			var tb textbuf.Buffer
			tb.Str("mup: draft-ietf-bess-mup-safi Section 3.1.3.1 canonical form registration failed: ").Str(err.Error()).Byte('\n')
			_ = tb.StdErr() //nolint:errcheck // the process exits on the next line either way
			os.Exit(1)
		}
	}

	reg := registry.Registration{
		Name:         "bgp-nlri-mup",
		Description:  "Mobile User Plane family plugin (draft-ietf-bess-mup-safi)",
		SupportsNLRI: true,
		Features:     "nlri",
		Families:     mupFamilies(),
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
			names := mupFamilies()
			family = fs.String("family", names[0], "Address family ("+strings.Join(names, " or ")+")")
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
