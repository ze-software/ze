package flowspec

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
	"github.com/ze-software/ze/internal/core/bgp/nlri/flowspec"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisyntax"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// Family registrations for FlowSpec. The wire codec in
// internal/core/bgp/nlri/flowspec builds the same families from the
// internal/core/family constants and registers nothing.
var (
	IPv4FlowSpec    = family.MustRegister(flowspec.AFIIPv4, flowspec.SAFIFlowSpec, "ipv4", "flow")
	IPv6FlowSpec    = family.MustRegister(flowspec.AFIIPv6, flowspec.SAFIFlowSpec, "ipv6", "flow")
	IPv4FlowSpecVPN = family.MustRegister(flowspec.AFIIPv4, flowspec.SAFIFlowSpecVPN, "ipv4", "flow-vpn")
	IPv6FlowSpecVPN = family.MustRegister(flowspec.AFIIPv6, flowspec.SAFIFlowSpecVPN, "ipv6", "flow-vpn")
)

func init() {
	// RFC 8955 Section 4.2: the receive path judges every FlowSpec NLRI section
	// through this rule (message.validateMPNLRISyntax), so a malformed NLRI
	// resets the session rather than reaching a plugin (rfc8955_syntax.go).
	for _, fam := range flowSpecFamilySet {
		if err := nlrisyntax.Register(fam, flowSpecSyntaxRule(fam)); err != nil {
			slog.Error("flowspec: NLRI syntax rule registration failed", "family", fam, "error", err)
			os.Exit(1)
		}
	}

	reg := registry.Registration{
		Name:         "bgp-nlri-flowspec",
		Description:  "FlowSpec NLRI encoding/decoding",
		RFCs:         []string{"8955", "8956"},
		SupportsNLRI: true,
		Features:     "nlri",
		Families:     flowSpecFamilies(),
		RunEngine:    runFlowSpecPlugin,
		ConfigureEngineLogger: func(loggerName string) {
			setFlowSpecLogger(slogutil.Logger(loggerName))
		},
		InProcessDecoder: func(input, output *bytes.Buffer) int {
			return RunFlowSpecDecode(input, output)
		},
		InProcessNLRIDecoder:       DecodeNLRIHex,
		InProcessNLRIEncoder:       EncodeNLRIHex,
		InProcessRouteEncoder:      EncodeRoute,
		InProcessConfigRouteParser: parseConfigRoute,
	}
	reg.CLIHandler = func(args []string) int {
		var family *string
		cfg := cli.BaseConfig(&reg)
		cfg.GetYANG = getFlowSpecYANG
		cfg.ConfigLogger = func(level string) {
			setFlowSpecLogger(slogutil.PluginLogger(reg.Name, level))
		}
		cfg.ExtraFlags = func(fs *flag.FlagSet) {
			names := flowSpecFamilies()
			family = fs.String("family", names[0], "Address family ("+strings.Join(names, ", ")+")")
		}
		cfg.RunCLIWithCtx = func(hex string, text bool, out, errOut io.Writer, fs *flag.FlagSet) int {
			return RunCLIDecode(hex, *family, text, out, errOut)
		}
		cfg.RunDecode = RunFlowSpecDecode
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "flowspec: registration failed: %v\n", err)
		os.Exit(1)
	}
}
