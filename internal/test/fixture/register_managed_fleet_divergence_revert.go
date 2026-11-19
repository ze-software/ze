// Design: docs/architecture/fleet-config.md -- the hub's resolution of a managed client that holds

package fixture

// The two-daemon drivers of revert on the hub, and of a revert decision that
// went stale. Their bodies are misc_fixture_managed_fleet_divergence_revert.go.
func init() {
	Register("managed/fleet-divergence-revert", managedFleetDivergenceRevertDriver)
	Register("managed/fleet-divergence-revert-void", managedFleetDivergenceRevertVoidDriver)
}
