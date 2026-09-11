// Design: docs/research/l2tpv2-ze-integration.md -- the compiled ze-test
// personality that proves a persistently failing listener socket keeps CPU
// low and counts every swallowed read
// (spec-subscriber-reader-loops-retry-a-failing-socket-without-backoff, AC-1).
// Related: tunnel_fixture_l2tp_failing_socket.go -- tunnelL2TPFailingSocket,
// the driver this registers, and the header that says why no gate reaches it.
//
// The name registered here is reachable only from
// test/draft/l2tp/subscriber-reader-failing-socket.ci and from an operator
// typing `ze-test fixture l2tp/failing-socket <metrics-port>`. No gated .ci
// names it, deliberately: the driver's own header says why, and what would
// have to change for it to be gated.

package fixture

func init() {
	Register("l2tp/failing-socket", tunnelL2TPFailingSocket)
}
