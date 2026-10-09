//go:build linux

// Design: docs/functional-tests.md -- native namespace BGP proof.
// Related: plugin_fixture_rfc2545_joint_subnet_linux.go -- topology and lifecycle.
package fixture

func init() {
	Register("plugin/rfc2545-joint-subnet", jointSubnetDriver)
	Register("plugin/rfc2545-joint-subnet-frr", jointSubnetFRRDriver)
}
