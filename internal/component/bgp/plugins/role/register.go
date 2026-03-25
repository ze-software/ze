package role

import (
	"os"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
	roleschema "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/role/schema"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

func init() {
	// RFC 9234: Register OTC attribute (type 35) as a known attribute.
	attribute.RegisterName(attribute.AttributeCode(otcAttrCode), "OTC")

	// Register mod handler for OTC egress stamping.
	// Called by applyMods in the reactor forward path after egress filters accept.
	registry.RegisterModHandler("set:attr:otc", otcModHandler)

	reg := registry.Registration{
		Name:            "role",
		Description:     "RFC 9234 BGP Role capability",
		RFCs:            []string{"9234"},
		SupportsCapa:    true,
		Features:        "capa yang",
		ConfigRoots:     []string{"bgp"},
		YANG:            roleschema.ZeRoleYANG,
		CapabilityCodes: []uint8{roleCapCode},
		RunEngine:       RunRolePlugin,
		IngressFilter:   OTCIngressFilter,
		EgressFilter:    OTCEgressFilter,
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
		logger().Error("role: registration failed", "error", err)
		os.Exit(1)
	}
}
