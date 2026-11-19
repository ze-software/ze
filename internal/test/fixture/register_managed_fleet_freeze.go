// Design: docs/architecture/fleet-config.md -- the freeze of a managed device's own configuration

package fixture

// The two-daemon driver for a managed device that refuses a local change to
// its own configuration. Its body is misc_fixture_managed_fleet_freeze.go.
func init() {
	Register("managed/fleet-freeze-refuses-commit", managedFleetFreezeRefusesCommitDriver)
}
