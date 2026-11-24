package route_refresh

import (
	"bytes"
	"flag"
	"fmt"
	"io"

	_ "github.com/ze-software/ze/internal/component/bgp/plugins/route_refresh/yang" // init() registers the refresh api and command modules
	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/slogutil"
)

func init() {
	if err := events.RegisterMessage(bgpevents.Namespace, bgpevents.SendEnhancedRefresh); err != nil {
		panic("BUG: route-refresh: register message enhanced-refresh: " + err.Error())
	}

	reg := registry.Registration{
		Name:            "bgp-route-refresh",
		Description:     "Route Refresh capability decoding",
		RFCs:            []string{"2918", "7313"},
		SupportsCapa:    true,
		Features:        "capa",
		ConfigRoots:     []string{configRootBGP},
		Dependencies:    []string{configRootBGP},
		CapabilityCodes: []uint8{2, 70},
		RunEngine:       RunRouteRefreshPlugin,
		InProcessDecoder: func(input, output *bytes.Buffer) int {
			return RunDecodeMode(input, output)
		},
		ConfigureEngineLogger: func(loggerName string) {
			SetLogger(slogutil.Logger(loggerName))
		},
	}
	reg.CLIHandler = func(args []string) int {
		var capCode uint
		cfg := cli.BaseConfig(&reg)
		cfg.ConfigLogger = func(level string) {
			SetLogger(slogutil.PluginLogger(reg.Name, level))
		}
		cfg.RunCLIDecode = RunCLIDecode
		cfg.ExtraFlags = func(fs *flag.FlagSet) {
			fs.UintVar(&capCode, "code", 2, "Capability code to decode (2 route-refresh, 70 enhanced-route-refresh)")
		}
		cfg.RunCLIWithCtx = func(hex string, text bool, out, errW io.Writer, _ *flag.FlagSet) int {
			if capCode > 255 {
				if _, err := fmt.Fprintf(errW, "error: capability code out of range: %d\n", capCode); err != nil { //nolint:errcheck // output
					logger().Debug("write failed", "err", err)
				}
				return 1
			}
			return RunCLIDecodeWithCode(uint8(capCode), hex, text, out, errW)
		}
		cfg.RunDecode = RunDecodeMode
		cfg.RunEngine = RunRouteRefreshPlugin
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		panic("BUG: route-refresh: registration failed: " + err.Error())
	}
}
