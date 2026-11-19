// Design: docs/architecture/fleet-config.md -- request fleet disable and enable: a managed client leaves its hub and rejoins it

package fixture

// The two-daemon drivers for a managed device that leaves its hub and rejoins
// it. Their bodies are misc_fixture_managed_fleet_leave.go.
func init() {
	Register("managed/fleet-disable-severs", managedFleetDisableSeversDriver)
	Register("managed/fleet-enable-rejoins", managedFleetEnableRejoinsDriver)
}
