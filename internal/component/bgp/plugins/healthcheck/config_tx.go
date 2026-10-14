// Design: docs/architecture/bgp/healthcheck-plugin.md -- disable is a config reload
// Related: healthcheck.go -- applyConfig, which an install runs
// Related: config.go -- parseConfig, which a verify runs

package healthcheck

import (
	"fmt"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// newHealthcheckConfigTx is the plugin's side of the config transaction: the
// probe set is its config. Verify parses and validates the candidate, so a
// probe the plugin mode cannot run is refused before any participant applies.
// Apply makes the candidate the running probe set without waiting for a
// stopped probe's exit (applyConfig), and rollback reinstalls the previous
// set, whose probes restart with a fresh FSM. Before any install no probe runs.
func newHealthcheckConfigTx(mgr *probeManager) *sdk.ConfigTx[[]ProbeConfig] {
	parse := func(sections []sdk.ConfigSection) ([]ProbeConfig, bool, error) {
		for _, section := range sections {
			if section.Root != configRootBGP {
				continue
			}
			probes, err := parseConfig(section.Data)
			if err != nil {
				return nil, false, fmt.Errorf("bgp-healthcheck: %w", err)
			}
			if err := mgr.validateConfig(probes); err != nil {
				return nil, false, fmt.Errorf("bgp-healthcheck: %w", err)
			}
			return probes, true, nil
		}
		return nil, false, nil
	}
	install := func(probes []ProbeConfig) error {
		mgr.applyConfig(probes)
		return nil
	}
	return sdk.NewConfigTx([]ProbeConfig(nil), parse, install)
}
