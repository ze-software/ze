// Design: docs/architecture/api/commands.md -- show fleet status answers with no daemon and through a running one

package fixture

// The real-daemon driver for `show fleet status` with the daemon stopped and
// running, on a managed device and on one that is not. Its body is
// misc_fixture_managed_fleet_status.go.
func init() {
	Register("managed/fleet-status", managedFleetStatusDriver)
}
