// Design: docs/guide/l2tp.md -- PPP negotiation: each NCP opens and closes on
// its own, and a session with no NCP open is bounded by `ncp timeout`.
// Related: tunnel_fixture_l2tp_ncp_lifetime.go -- the two LAC peers this registers.
// Related: tunnel_fixture_l2tp_radius.go -- tunnelRadiusAccounting, the
// RADIUS mock registered here under each test's name.

package fixture

func init() {
	// The mock is the radius-acct-wire one. It accepts the CHAP credential
	// the proxy call carries, which is all these tests ask of RADIUS.
	Register("l2tp/ppp-ncp-terminate-after-up/radius", tunnelRadiusAccounting)
	Register("l2tp/ppp-ncp-terminate-after-up", tunnelL2TPNCPTerminateAfterUp)
	Register("l2tp/ppp-ncp-protocol-reject-ipcp/radius", tunnelRadiusAccounting)
	Register("l2tp/ppp-ncp-protocol-reject-ipcp", tunnelL2TPNCPProtocolRejectIPCP)
}
