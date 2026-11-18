// Design: docs/architecture/fleet-config.md -- a managed client starts from its cached config when its hub is unreachable

package fixture

// The real-daemon driver for the startup matrix row "hub unreachable, cached
// config exists". Its body is misc_fixture_managed_cached_boot.go.
func init() {
	Register("managed/client-cached-boot-hub-down", managedCachedBootHubDownDriver)
}
