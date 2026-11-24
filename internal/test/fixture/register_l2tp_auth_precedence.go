// Design: docs/architecture/l2tp/subscriber-session-model.md -- prioritized subscriber auth handler selection.
// Related: tunnel_fixture_l2tp_auth_precedence.go -- tunnelL2TPAuthPrecedence peer driver.

package fixture

func init() {
	Register("l2tp/auth-precedence", tunnelL2TPAuthPrecedence)
}
