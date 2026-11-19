// Design: docs/architecture/fleet-config.md -- the hub's resolution of a managed client that holds

package fixture

// The two-daemon drivers of adopt on the hub, and of the hub's copy across a
// hub restart. Their bodies are misc_fixture_managed_fleet_divergence_adopt.go.
func init() {
	Register("managed/fleet-divergence-adopt", managedFleetDivergenceAdoptDriver)
	Register("managed/fleet-divergence-hub-restart", managedFleetDivergenceHubRestartDriver)
}
