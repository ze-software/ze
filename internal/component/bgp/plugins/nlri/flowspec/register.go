package flowspec

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/slogutil"
)

func init() {
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
