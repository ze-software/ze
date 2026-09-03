package filter_path_asn

import (
	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	fpayang "github.com/ze-software/ze/internal/component/bgp/plugins/filter_path_asn/yang"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/metrics"
)

func init() {
	_ = registry.Register(registry.Registration{
		Name:         "bgp-filter-path-asn",
		Description:  "Named reject-asn list: rejects a route whose AS_PATH carries a listed ASN at a listed position",
		ConfigRoots:  []string{configRootBGP},
		Dependencies: []string{configRootBGP},
		YANG:         fpayang.ZeFilterPathAsnYANG,
		FilterTypes:  []string{"reject-asn"},
		// A reject-asn list is what discharges the transit-leak obligation: it
		// refuses a route whose AS_PATH carries a listed ASN. The config rule
		// that makes the check mandatory for a peer with a declared RFC 9234
		// role reads this declaration, so the rule holds no filter type name.
		FilterObligations: []string{filterapi.ObligationTransitLeak},
		RunEngine:         runFilterPathASN,
		CLIHandler:        func(_ []string) int { return 0 },
		ConfigureMetrics: func(reg metrics.Registry) {
			setMetricsRegistry(reg)
		},
	})
}
