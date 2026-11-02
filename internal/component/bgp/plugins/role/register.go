package role

import (
	"os"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	roleyang "github.com/ze-software/ze/internal/component/bgp/plugins/role/yang"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// The route metadata keys this plugin's ingress step writes (otc.go), as
// fields of the BGP route view: a filter declares a read of them, and a
// filter at bgp import may set them (spec-filter-wire-6-stage-filters D-8).
var (
	// fieldSrcRole is Ze's local role toward the peer a route came from.
	fieldSrcRole = policy.MustRegisterField("src-role", policy.RouteView{})
	// fieldSrcPeerRole is what that peer is to Ze.
	fieldSrcPeerRole = policy.MustRegisterField("src-peer-role", policy.RouteView{})
	// fieldOTC is the route's OTC attribute, a field of the BGP route view
	// only this plugin's filters may set.
	fieldOTC = policy.MustRegisterField("otc", policy.RouteView{})
)

// rolePluginName is this plugin's registration name: the owner of the otc
// field, and so the one plugin whose filters may set it.
const rolePluginName = "bgp-role"

func init() {
	policyview.MustRegisterMetadata(fieldSrcRole, policyview.MetadataText)
	policyview.MustRegisterMetadata(fieldSrcPeerRole, policyview.MetadataText)
	// RFC 9234 Section 5: "The OTC Attribute is an optional transitive Path Attribute of the
	// UPDATE message with Attribute Type Code 35 and a length of 4 octets."
	// The field reads the four octets as a number, and a value of another
	// length fails the member that reads it: "The OTC Attribute is
	// considered malformed if the length value is not 4."
	policyview.MustRegisterAttributeNumber(fieldOTC, attribute.AttributeCode(otcAttrCode), rolePluginName)

	// RFC 9234 Section 5: "The OTC Attribute is an optional transitive Path Attribute of
	// the UPDATE message with Attribute Type Code 35 and a length of 4 octets." That
	// sentence fixes both bits, so it travels with the name: recognition and the RFC 7606
	// Section 3.c judgement of a received flags octet are one registration, and dropping
	// this plugin takes both away together.
	attribute.RegisterName(attribute.AttributeCode(otcAttrCode), "OTC", attribute.OptionalTransitiveFlags())

	// The writer of the OTC attribute: a set of the otc field becomes an
	// AttrModSet the reactor's rebuild hands to this handler, which keeps an
	// OTC the route already carries (RFC 9234 Section 5: "Once the OTC
	// Attribute has been set, it MUST be preserved unchanged").
	filterapi.RegisterAttrModHandler(otcAttrCode, otcAttrModHandler)

	// The OTC filters read the session facts and src-role through these.
	resolveOTCReaders()

	// role transit-asns completes and is checked at commit against the
	// asn-sets the policy root defines (upstream.go).
	yang.RegisterReferenceSource(asnSetReferenceSource, asnSetReference)

	reg := registry.Registration{
		Name:         rolePluginName,
		Description:  "RFC 9234 BGP Role capability",
		RFCs:         []string{"9234"},
		SupportsCapa: true,
		Features:     "capa yang",
		ConfigRoots:  []string{configRootBGP},
		// The policy root holds the asn-sets role transit-asns names; a
		// read root never auto-loads the plugin.
		ConfigReads:  []string{policy.RootName},
		Dependencies: []string{configRootBGP, policy.PluginName},
		// The RFC 9234 Section 5 OTC procedures (otc_filter.go) and the
		// RFC 7454 Section 9 upstream check (upstream.go): the filters, and
		// the role peers each is inserted on.
		Filters:          roleFilterDecls(),
		FilterInsertions: roleInsertions(),
		YANG:             roleyang.ZeRoleYANG,
		CapabilityCodes:  []uint8{roleCapCode},
		RunEngine:        RunRolePlugin,
		ConfigureEngineLogger: func(loggerName string) {
			ConfigureLogger(slogutil.Logger(loggerName))
		},
		ConfigureMetrics: func(reg metrics.Registry) {
			setMetricsRegistry(reg)
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
