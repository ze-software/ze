// Design: docs/architecture/fleet-config.md -- managed inventory and health reporting
// Related: misc_fixture_managed_fleet_reports.go -- inventory and health checks through the hub CLI

package fixture

func init() {
	Register("managed/fleet-inventory-report", managedFleetInventoryReportDriver)
	Register("managed/fleet-health-report", managedFleetHealthReportDriver)
}
