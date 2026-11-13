// Design: docs/architecture/l2tp/subscriber-session-model.md -- the order of
// one L2TP session's notifications, proved end to end by an operator clear
// (spec-single-owner-lifecycle-5-session-end-locals AC-8, AC-9).
// Related: tunnel_fixture_l2tp_session_clear.go -- tunnelL2TPSessionClear,
// the LAC peer this registers.
// Related: tunnel_fixture_l2tp_radius.go -- tunnelRadiusAccounting, the
// RADIUS mock registered here under this test's name.

package fixture

func init() {
	// The mock is the radius-acct-wire one: the same function, so the two
	// tests read one decoder of what the server received.
	Register("l2tp/session-clear-leaves-no-subscriber/radius", tunnelRadiusAccounting)
	Register("l2tp/session-clear-leaves-no-subscriber", tunnelL2TPSessionClear)
}
