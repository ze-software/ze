// Design: docs/architecture/api/process-protocol.md -- fatal failure policy
// Overview: plugin_fixture_failure_policy_peer.go -- live-peer fatal owner

package fixture

func init() {
	Register("plugin/failure-policy-fatal-peer", failurePolicyFatalPeerDriver)
}
