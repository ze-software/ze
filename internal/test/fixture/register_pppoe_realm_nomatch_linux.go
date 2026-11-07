//go:build linux

// Design: docs/guide/l2tp.md -- relay by realm: a username whose realm names
// no binding is authenticated by the local auth handler.
// Related: tunnel_fixture_pppoe_realm_linux.go -- tunnelPPPoERealmNoMatch, the
// PPPoE subscriber this registers.

package fixture

func init() {
	Register("pppoe/realm-nomatch", tunnelPPPoERealmNoMatch)
}
