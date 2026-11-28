package mup

import (
	"bytes"
	"flag"
	"io"
	"strings"

	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlritype"
	"github.com/ze-software/ze/internal/core/slogutil"
)

func init() {
	registerMUPDiagnosticCodes()

	// RFC 7606 Section 5.4: BGP-MUP is a typed address family and
	// draft-ietf-bess-mup-safi states no deviation, so a route whose architecture and
	// route type ze does not implement must be discarded. draft-ietf-bess-mup-safi
	// Section 3.1.3.1: a malformed route is treated as withdrawn (recognizerFor), so
	// the recognizer judges the body and is registered as one that does. Registered
	// here, beside the families, so removing this plugin removes both the
	// advertisement and the obligation.
	for _, fam := range []Family{IPv4MUP, IPv6MUP} {
		if err := nlritype.RegisterBodyRecognizer(fam, recognizerFor(fam.AFI)); err != nil {
			panic("BUG: mup: RFC 7606 Section 5.4 recognizer registration failed: " + err.Error())
		}
		// draft-ietf-bess-mup-safi Sections 3.3.3 and 3.3.6: ISD and DSD routes
		// without a Prefix-SID attribute, or whose Address/next hop mismatches
		// the known locator originator, are treated as withdrawn.
		if err := nlritype.RegisterUpdateCheck(fam, updateCheckFor(fam.AFI)); err != nil {
			panic("BUG: mup: draft-ietf-bess-mup-safi segment discovery check registration failed: " + err.Error())
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
		panic("BUG: mup: registration failed: " + err.Error())
	}
}
