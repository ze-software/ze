// Design: docs/architecture/l2tp/subscriber-session-model.md -- a readable
// session selector resolves once to one captured original, proved through
// the operator's clear over SSH.
// Related: tunnel_fixture_l2tp_session_selector.go -- tunnelL2TPSessionSelectorScope,
// the LAC peer this registers.
// Related: tunnel_fixture_l2tp_radius.go -- tunnelRadiusAccounting, the
// RADIUS mock registered here under this test's name.

package fixture

func init() {
	// The mock is the radius-acct-wire one, so the peer reads the one
	// decoder of what the server received.
	Register("l2tp/session-selector-scope/radius", tunnelRadiusAccounting)
	Register("l2tp/session-selector-scope", tunnelL2TPSessionSelectorScope)
}
