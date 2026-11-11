// Design: docs/guide/bmp.md -- runtime admission for Loc-RIB monitoring.
// Related: bmp.go -- initial configuration and reload verification.
package bmp

import (
	"errors"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

var errLocRIBEventBusUnavailable = errors.New("bgp-bmp Loc-RIB monitoring cannot run because the BGP RIB event bus is unavailable in this process; replace the external instance with plugin { internal bmp { use bgp-bmp; } }")

// validateLocRIBRuntimeConfig checks the live feed before runtime configuration
// can start sessions. The offline sender parser remains independent of it.
// RFC 9069 Section 5.4: "Route Monitoring messages are used for initial
// synchronization of the Loc-RIB." A process with no RIB bus cannot supply it.
func validateLocRIBRuntimeConfig(sections []sdk.ConfigSection) error {
	for _, section := range sections {
		if section.Root != configRootBGP {
			continue
		}
		cfg, err := parseSenderConfig(section.Data)
		if err != nil {
			return err
		}
		if cfg.LocRIB == yangTrue && getEventBus() == nil {
			return errLocRIBEventBusUnavailable
		}
	}
	return nil
}
