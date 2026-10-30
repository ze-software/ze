// Design: docs/architecture/bgp/filter-irr.md -- IRR filter config at boot and on reload
// Related: filter_irr.go -- handleConfigure, the one place a config becomes the filter's state

package filter_irr

import (
	"github.com/ze-software/ze/internal/component/policy"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// irrDelivery is the two roots the filter reads: the policy root, whose irr
// container holds the IRR and PeeringDB settings, and the bgp root, whose
// peers and chains decide which ASNs are resolved.
type irrDelivery struct {
	policy map[string]any
	bgp    map[string]any
}

// newIRRConfigTx is the filter's side of the config transaction: the two
// roots are its config, handleConfigure installs them, and the SDK keeps the
// verified candidate and the replaced config for rollback. Before any install
// the filter runs no config, which handleConfigure(irrDelivery{}) restores.
// The registration reads policy as ConfigReads, so a reload delivers both
// roots whole whenever either changes (policy.SectionsParser).
func newIRRConfigTx(plug *irrPlugin) *sdk.ConfigTx[irrDelivery] {
	parse := policy.SectionsParser(pluginName, errFilterIrrInvalidBgpConfigJson, configRootBGP, parseIRRDelivery)
	return sdk.NewConfigTx(irrDelivery{}, parse, func(cfg irrDelivery) error {
		plug.handleConfigure(cfg)
		return nil
	})
}

// parseIRRDelivery keeps the two roots for handleConfigure. It refuses
// nothing: handleConfigure reads every value with its default.
func parseIRRDelivery(policyCfg, bgpCfg map[string]any) (irrDelivery, error) {
	return irrDelivery{policy: policyCfg, bgp: bgpCfg}, nil
}
