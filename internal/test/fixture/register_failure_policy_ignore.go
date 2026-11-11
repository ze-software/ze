// Design: docs/architecture/doctor-and-health-checks.md -- plugin state a runtime check reads
// Related: plugin_fixture_failure_policy_ignore.go -- the driver registered here

package fixture

func init() {
	Register("plugin/failure-policy-ignore", failurePolicyIgnoreDriver)
}
