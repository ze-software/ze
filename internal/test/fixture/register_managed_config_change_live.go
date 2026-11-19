// Design: docs/architecture/fleet-config.md -- the hub's config-changed push to a connected client

package fixture

// The two-daemon driver of a configuration change imported on a running hub.
// Its body is misc_fixture_managed_config_change_live.go.
func init() {
	Register("managed/fleet-config-change-live", managedFleetConfigChangeLiveDriver)
}
