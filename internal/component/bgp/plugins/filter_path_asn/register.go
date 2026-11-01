package filter_path_asn

import (
	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	fpayang "github.com/ze-software/ze/internal/component/bgp/plugins/filter_path_asn/yang"
	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/metrics"
)

// filterTypeName is the ze:filter list this plugin owns under the policy root.
const filterTypeName = "reject-asn"

func init() {
	// The route fields the filter type reads and sets, against which a chain
	// at an attachment point is checked (spec "Fields each filter type reads
	// and sets").
	policy.MustRegisterFilterType(filterTypeName,
		[]policy.Field{policy.FieldASPath},
		nil)

	// The reject-asn condition of a match instance (condition.go), keyed like
	// the list it names and judged at a BGP chain. The Loc-RIB route carries
	// no AS path, so it has no check there.
	rejectASNCondition := policy.MustRegisterMatchCondition(filterTypeName,
		[]policy.Field{policy.FieldASPath}, false)
	policychain.MatchConditions.MustAdd(rejectASNCondition, compileBGPCondition)
	// RFC 7454 Section 9: a match naming the condition discharges the
	// transit-leak obligation a declared RFC 9234 role carries, as a
	// reject-asn chain member does (FilterObligations below).
	policy.MustDeclareMatchConditionObligation(rejectASNCondition, filterapi.ObligationTransitLeak)
	// The match leaf naming a list completes and is checked at commit
	// through this source (reject_asn_reference.go).
	yang.RegisterReferenceSource(rejectASNReferenceSource, rejectASNReference)

	_ = registry.Register(registry.Registration{
		Name:         "bgp-filter-path-asn",
		Description:  "Named reject-asn list: rejects a route whose AS_PATH carries a listed ASN at a listed position",
		ConfigRoots:  []string{configRootBGP},
		ConfigReads:  []string{policy.RootName},
		Dependencies: []string{configRootBGP, policy.PluginName},
		YANG:         fpayang.ZeFilterPathAsnYANG,
		FilterTypes:  []string{filterTypeName},
		// A reject-asn list is what discharges the transit-leak obligation: it
		// refuses a route whose AS_PATH carries a listed ASN. The config rule
		// that makes the check mandatory for a peer with a declared RFC 9234
		// role reads this declaration, so the rule holds no filter type name.
		FilterObligations: []string{filterapi.ObligationTransitLeak},
		RunEngine:         runFilterPathASN,
		Commands:          commandDecls(),
		CLIHandler:        func(_ []string) int { return 0 },
		ConfigureMetrics: func(reg metrics.Registry) {
			setMetricsRegistry(reg)
		},
	})
}
