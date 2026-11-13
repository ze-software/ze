// Design: docs/architecture/testing/interop.md -- concurrent state validation.
// Related: check_ldp_backoff_clear.go -- the scenario's checker, registered in check_special.go.
// Related: prepare.go -- scenarioHostAddresses, addHostAddresses.
package bgp

// ze's LDP transport address (host ldpTransportHost, .250) is on eth0 before
// ze starts. ze dials from it on FRR's first Hello, so an address the checker
// added once the containers ran lost that race on some runs: the bind failed,
// the failed setup imposed a 15 s backoff, and each further loss doubled it.
func init() {
	scenarioHostAddresses[ldpBackoffClearScenario] = []hostAddress{{peer: "ze", host: ldpTransportHost}}
}
