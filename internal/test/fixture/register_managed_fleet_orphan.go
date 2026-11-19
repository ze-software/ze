// Design: docs/architecture/fleet-config.md -- the hub's copy of the configuration a client no serving block declares

package fixture

// The two-daemon driver for the hub's copy of a client that no serving block
// declares any more. Its body is misc_fixture_managed_fleet_orphan.go.
func init() {
	Register("managed/fleet-divergence-orphan", managedFleetDivergenceOrphanDriver)
}
