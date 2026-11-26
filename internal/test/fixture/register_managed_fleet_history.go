// Design: docs/architecture/fleet-config.md -- hub-side fleet metric history functional fixtures
// Related: misc_fixture_managed_fleet_history.go -- reconnect, gap, restart, cap eviction and deletion drivers

package fixture

func init() {
	Register("managed/fleet-history-reconnect", managedFleetHistoryReconnectDriver)
	Register("managed/fleet-history-cap", managedFleetHistoryCapDriver)
}
