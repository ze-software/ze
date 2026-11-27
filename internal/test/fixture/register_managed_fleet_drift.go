// Design: docs/architecture/fleet-config.md -- fleet SHA-1 hash chain drift and resync-failed alarm functional fixtures
// Related: misc_fixture_managed_fleet_drift.go -- fleet-drift-hash-chain and fleet-resync-failed-alarm drivers

package fixture

func init() {
	Register(fleetDriftHashChainFixtureName, managedFleetDriftHashChainDriver)
	Register(fleetResyncFailedAlarmFixture, managedFleetResyncFailedAlarmDriver)
}
