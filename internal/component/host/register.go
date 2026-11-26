// Design: docs/features/ai-first.md -- platform name registration for doctor check validation
// Design: docs/architecture/fleet-config.md -- host-owned fleet inventory report producer registration

package host

import (
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/fleet"
)

func init() {
	names := make([]string, 0, len(platformTypeNames))
	for _, name := range platformTypeNames {
		names = append(names, name)
	}
	registry.RegisterDoctorPlatforms(names)

	fleet.MustRegisterReportProducer(fleet.ReportProducer{
		Name:     fleetInventoryProducerName,
		Method:   fleet.VerbInventoryReport,
		Interval: FleetInventoryInterval,
		Collect:  collectFleetInventory,
	})
}
