package gr

import (
	"bytes"
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	gryang "github.com/ze-software/ze/internal/component/bgp/plugins/gr/yang"
	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// grPluginName is the registration name of this plugin, and the name an
// operator writes in `use bgp-gr` or `run "ze plugin bgp-gr"`.
const grPluginName = "bgp-gr"

// configRootBGP is the configuration root this plugin reads its peers from.
const configRootBGP = "bgp"

func init() {
	// Register LLGR well-known community names (RFC 9494).
	for _, c := range []struct {
		value attribute.Community
		name  string
	}{
		{attribute.CommunityLLGRStale, "llgr-stale"},
		{attribute.CommunityNoLLGR, "no-llgr"},
	} {
		if err := attribute.RegisterCommunityName(c.value, c.name); err != nil {
			logger().Error("community registration failed", "error", err)
		}
	}

	// The LLGR egress filter answers from state only RunGRPlugin stores, so an
	// out-of-process engine leaves it blind. doctor.go reports that arrangement.
	for _, m := range grDiagnosticCodes {
		_ = diagnostic.Register(m)
	}
	if err := diagnostic.RegisterDoctorCheck(grDoctorCheck); err != nil {
		fmt.Fprintf(os.Stderr, "gr: doctor check registration failed: %v\n", err)
		os.Exit(1)
	}

	// Route filter pipeline contribution (BGP-owned seam, not the generic registry).
	if err := filterapi.Register(filterapi.Filter{
		Name:  grPluginName,
		Stage: filterapi.FilterStageAnnotation,
		// RFC 9494: the LLGR egress decision (keep+mark / depreference / withdraw)
		// must run per destination peer on the RIB stale-readvertise rail, not just
		// on ForwardUpdate. Readvertise opts it into AnnounceNLRIBatch for stale batches.
		Egress:      LLGREgressFilter,
		Readvertise: true,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "gr: filter registration failed: %v\n", err)
		os.Exit(1)
	}

	reg := registry.Registration{
		Name:            grPluginName,
		Description:     "Graceful Restart capability and mechanism plugin",
		RFCs:            []string{"4724", "9494"},
		SupportsCapa:    true,
		Features:        "capa yang",
		ConfigRoots:     []string{configRootBGP},
		YANG:            gryang.ZeGracefulRestartYANG,
		CapabilityCodes: []uint8{64, 71},
		Dependencies:    []string{configRootBGP, "bgp-rib"},
		RunEngine:       RunGRPlugin,
		InProcessDecoder: func(input, output *bytes.Buffer) int {
			return RunDecodeMode(input, output)
		},
		ConfigureEngineLogger: func(loggerName string) {
			SetLogger(slogutil.Logger(loggerName))
		},
		ConfigureMetrics: func(reg metrics.Registry) {
			SetMetricsRegistry(reg)
		},
	}
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.GetYANG = GetYANG
		cfg.ConfigLogger = func(level string) {
			SetLogger(slogutil.PluginLogger(reg.Name, level))
		}
		cfg.RunCLIDecode = RunCLIDecode
		cfg.RunDecode = RunDecodeMode
		cfg.RunEngine = RunGRPlugin
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "gr: registration failed: %v\n", err)
		os.Exit(1)
	}
}
