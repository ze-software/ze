// Design: docs/architecture/fleet-config.md -- audit the hub's resolution of a managed client that holds.

package audit

// ActionFleetDivergenceAdopt records an operator making the configuration a
// managed client holds the hub's configuration for that client.
const ActionFleetDivergenceAdopt = "fleet-divergence-adopt"

// ActionFleetDivergenceRevert records an operator deciding that a managed
// client which holds drops its configuration and takes the hub's.
const ActionFleetDivergenceRevert = "fleet-divergence-revert"
