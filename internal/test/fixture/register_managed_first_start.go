// Design: docs/architecture/fleet-config.md -- the first start of a managed client that ze init --managed provisioned

package fixture

// The two-daemon driver for the first start of a managed client provisioned by
// `ze init --managed` alone. Its body is misc_fixture_managed_first_start.go.
func init() {
	Register("managed/client-first-start-from-init", managedFirstStartDriver)
}
