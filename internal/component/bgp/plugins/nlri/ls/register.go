package ls

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/slogutil"
)

func init() {
	// RFC 7752: Register BGP-LS attribute (type 29).
	attribute.RegisterName(29, "BGP_LS")

	reg := registry.Registration{
		Name:         "bgp-nlri-ls",
		Description:  "BGP-LS family plugin",
		RFCs:         []string{"7752", "9085", "9514"},
		SupportsNLRI: true,
		Features:     "nlri",
		Families:     []string{familyBGPLS, familyBGPLSVPN},
		RunEngine:    runBGPLSPlugin,
		ConfigureEngineLogger: func(loggerName string) {
			setBGPLSLogger(slogutil.Logger(loggerName))
		},
		InProcessDecoder: func(input, output *bytes.Buffer) int {
			return runBGPLSDecode(input, output)
		},
	}
	reg.CLIHandler = func(args []string) int {
		var family *string
		cfg := cli.BaseConfig(&reg)
		cfg.GetYANG = getBGPLSYANG
		cfg.ConfigLogger = func(level string) {
			setBGPLSLogger(slogutil.PluginLogger(reg.Name, level))
		}
		cfg.ExtraFlags = func(fs *flag.FlagSet) {
			family = fs.String("family", familyBGPLS, "Address family (bgp-ls/bgp-ls, bgp-ls/bgp-ls-vpn)")
		}
		cfg.RunCLIWithCtx = func(hex string, text bool, out, errOut io.Writer, fs *flag.FlagSet) int {
			return runBGPLSCLIDecode(hex, *family, text, out, errOut)
		}
		cfg.RunDecode = runBGPLSDecode
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "bgpls: registration failed: %v\n", err)
		os.Exit(1)
	}

	// RFC 9552 Section 8.2.2: "A BGP-LS Propagator ... should not perform semantic
	// validation of the Link-State NLRI or the BGP-LS Attribute to determine if it is
	// malformed or invalid." So this registration wires no per-route decode of the BGP-LS
	// Attribute into the receive path, and a received attribute is judged by the syntactic
	// walk alone (validateBGPLSAttr, internal/component/bgp/message/rfc7606_bgpls.go)
	// before being relayed with its TLVs untouched.
	//
	// The call below is deliberately unreachable. It names the check that is NOT run and
	// the point it would have been wired in, which an uncalled function cannot do: the
	// reader asking "does ze semantically validate a propagated BGP-LS Attribute, and where
	// would it have" gets both answers here. Go eliminates the branch, so it costs nothing
	// at runtime.
	//
	// RFC requirement: RFC9552-8.2.2-3 positive -- the semantic decode is absent from the
	// propagation path, marked at the point it would have been wired in.
	if false { //nolint:staticcheck // deliberately unreachable: it marks the semantic validation RFC 9552 Section 8.2.2 says a Propagator does not perform
		var received []byte // the BGP-LS Attribute bytes a wired decode would be handed
		if _, err := decodeAllAttrTLVs(received); err != nil {
			fmt.Fprintf(os.Stderr, "bgpls: attribute decode failed: %v\n", err)
		}
	}
}
