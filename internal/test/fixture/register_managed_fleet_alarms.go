// Design: docs/architecture/fleet-alarms.md -- hub-derived fleet alarm functional fixtures
// Related: misc_fixture_managed_fleet_alarms.go -- alarm lifecycle and history restart drivers

package fixture

func init() {
	Register("managed/fleet-alarm-lifecycle", managedFleetAlarmLifecycleDriver)
	Register("managed/fleet-alarm-history", managedFleetAlarmHistoryDriver)
}
