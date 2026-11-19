// Design: docs/architecture/fleet-config.md -- the baseline and the hold of a managed client that runs a configuration its hub did not send

package fixture

// The two-daemon drivers for a managed device that holds: one that keeps a
// configuration its hub did not send, and one whose hold ends when the hub is
// given that configuration. Their bodies are misc_fixture_managed_fleet_hold.go.
func init() {
	Register("managed/fleet-hold-keeps-local-edit", managedFleetHoldKeepsLocalEditDriver)
	Register("managed/fleet-hold-clears-on-match", managedFleetHoldClearsOnMatchDriver)
}
