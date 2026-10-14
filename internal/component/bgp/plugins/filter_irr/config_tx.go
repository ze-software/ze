// Design: docs/architecture/bgp/filter-irr.md -- IRR filter config at boot and on reload
// Related: filter_irr.go -- handleConfigure, the one place a config becomes the filter's state

package filter_irr

import sdk "github.com/ze-software/ze/pkg/plugin/sdk"

// newIRRConfigTx is the filter's side of the config transaction: the bgp
// subtree is its config, handleConfigure installs it, and the SDK keeps the
// verified candidate and the replaced config for rollback. Before any install
// the filter runs no subtree, which handleConfigure(nil) restores.
func newIRRConfigTx(plug *irrPlugin) *sdk.ConfigTx[map[string]any] {
	return sdk.NewConfigTx(map[string]any(nil), parseIRRSections, func(bgpCfg map[string]any) error {
		plug.handleConfigure(bgpCfg)
		return nil
	})
}

// parseIRRSections reads the bgp subtree of a delivery. A delivery with no bgp
// section carries nothing for the filter.
func parseIRRSections(sections []sdk.ConfigSection) (map[string]any, bool, error) {
	bgpCfg, err := bgpSubtreeFromSections(sections)
	if err != nil {
		return nil, false, err
	}
	return bgpCfg, bgpCfg != nil, nil
}
