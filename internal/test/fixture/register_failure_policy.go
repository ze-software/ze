// Design: docs/architecture/api/process-protocol.md -- the Stage-1 failure-policy declaration
// Overview: plugin_fixture_failure_policy.go -- the drivers this file names

package fixture

func init() {
	Register("plugin/failure-policy-restart", failurePolicyRestartDriver)
	Register("plugin/failure-policy-fatal", failurePolicyFatalDriver)
	Register("plugin/failure-policy-disagree", failurePolicyDisagreeDriver)
}
