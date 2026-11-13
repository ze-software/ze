// Design: docs/architecture/core-design.md -- system RIB configuration.
// Related: register.go -- startup and transaction callback bindings.

package sysrib

import (
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// routeViewConfiguration is an owned, complete declaration from the RIB schema.
// Callers MUST keep its maps immutable after successful parsing.
type routeViewConfiguration struct {
	distances map[string]int
	permitted map[string]bool
}

// parseRouteViewConfiguration reads a complete candidate, including removal of
// the owned root. Absence MUST return present=true: leaving the previous values
// in place would turn root removal into an accepted change that never ran.
// ConfigTx MUST retain the returned maps unchanged through apply and rollback.
func parseRouteViewConfiguration(sections []sdk.ConfigSection) (routeViewConfiguration, bool, error) {
	data := "{}"
	for _, section := range sections {
		if section.Root != configRootRIB {
			continue
		}
		data = section.Data
		break
	}
	distances, err := parseAdminDistanceConfig(data)
	if err != nil {
		return routeViewConfiguration{}, false, opererr.Wrap(err,
			"The system RIB distance configuration could not be read",
			"No routing declaration was changed",
			"ze config validate -")
	}
	permitted, err := parseFIBImportConfig(data)
	if err != nil {
		return routeViewConfiguration{}, false, opererr.Wrap(err,
			"The system RIB forwarding configuration could not be read",
			"No forwarding permission was changed",
			"ze config validate -")
	}
	return routeViewConfiguration{distances: distances, permitted: permitted}, true, nil
}
