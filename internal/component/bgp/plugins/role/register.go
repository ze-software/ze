package role

import (
	"os"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	roleyang "github.com/ze-software/ze/internal/component/bgp/plugins/role/yang"
	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/slogutil"
)

func init() {
	// RFC 9234: Register OTC attribute (type 35) as a known attribute.
	attribute.RegisterName(attribute.AttributeCode(otcAttrCode), "OTC")

	// Register attr mod handler for OTC egress stamping (progressive build path).
	// Called by buildModifiedPayload in the reactor forward path after egress filters accept.
	filterapi.RegisterAttrModHandler(otcAttrCode, otcAttrModHandler)

	// Route filter pipeline contribution (BGP-owned seam, not the generic registry).
	if err := filterapi.Register(filterapi.Filter{
		Name:    "bgp-role",
		Stage:   filterapi.FilterStageAnnotation,
		Ingress: OTCIngressFilter,
		Egress:  OTCEgressFilter,
	}); err != nil {
		logger().Error("bgp-role: filter registration failed", "error", err)
		os.Exit(1)
	}

	reg := registry.Registration{
		Name:            "bgp-role",
		Description:     "RFC 9234 BGP Role capability",
		RFCs:            []string{"9234"},
		SupportsCapa:    true,
		Features:        "capa yang",
		ConfigRoots:     []string{"bgp"},
		Dependencies:    []string{"bgp"},
		YANG:            roleyang.ZeRoleYANG,
		CapabilityCodes: []uint8{roleCapCode},
		RunEngine:       RunRolePlugin,
		ConfigureEngineLogger: func(loggerName string) {
			ConfigureLogger(slogutil.Logger(loggerName))
		},
	}
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.GetYANG = GetYANG
		cfg.ConfigLogger = func(level string) {
			ConfigureLogger(slogutil.PluginLogger(reg.Name, level))
		}
		cfg.RunCLIDecode = RunCLIDecode
		cfg.RunEngine = RunRolePlugin
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		logger().Error("bgp-role: registration failed", "error", err)
		os.Exit(1)
	}
}
