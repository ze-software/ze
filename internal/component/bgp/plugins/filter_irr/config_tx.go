// Design: docs/architecture/bgp/filter-irr.md -- IRR filter config at boot and on reload
// Related: filter_irr.go -- handleConfigure, the one place a config becomes the filter's state

package filter_irr

import (
	"github.com/ze-software/ze/internal/component/bgp/configjson"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// irrDelivery is the one root the filter reads: the bgp root, whose peers and
// chains decide which ASNs are enforced. The IRR server, the PeeringDB URL and
// the refresh period are the irr plugin's config (system irr), not the
// filter's.
type irrDelivery struct {
	bgp map[string]any
}

// newIRRConfigTx is the filter's side of the config transaction: the bgp root
// is its config, handleConfigure installs it, and the SDK keeps the verified
// candidate and the replaced config for rollback. Before any install the
// filter runs no config, which handleConfigure(irrDelivery{}) restores.
func newIRRConfigTx(plug *irrPlugin) *sdk.ConfigTx[irrDelivery] {
	parse := configjson.BGPSectionParser(pluginName, errFilterIrrInvalidBgpConfigJson, parseIRRDelivery)
	return sdk.NewConfigTx(irrDelivery{}, parse, func(cfg irrDelivery) error {
		plug.handleConfigure(cfg)
		return nil
	})
}

// parseIRRDelivery keeps the bgp root for handleConfigure. It refuses
// nothing: handleConfigure reads every value with its default.
func parseIRRDelivery(bgpCfg map[string]any) (irrDelivery, error) {
	return irrDelivery{bgp: bgpCfg}, nil
}
