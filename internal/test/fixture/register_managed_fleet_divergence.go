// Design: docs/architecture/fleet-config.md -- the hub's copy of the configuration a managed client holds

package fixture

// The two-daemon driver for a managed device that holds and sends its
// configuration to its hub. Its body is misc_fixture_managed_fleet_divergence.go.
func init() {
	Register("managed/fleet-divergence-upload", managedFleetDivergenceUploadDriver)
}
