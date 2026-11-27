// Design: docs/architecture/fleet-alarms.md -- hub-derived fleet alarm cause functional fixture
// Related: misc_fixture_managed_fleet_alarm_causes.go -- alarm-hidden-behind-offline driver

package fixture

func init() {
	Register(fleetAlarmCausesFixtureName, managedFleetAlarmCausesDriver)
}
