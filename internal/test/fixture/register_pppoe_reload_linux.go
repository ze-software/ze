//go:build linux

// Design: docs/guide/pppoe.md -- the compiled le test personality that proves a
// reload reaches a running Ze AC.
// Related: tunnel_fixture_pppoe_reload_linux.go -- tunnelPPPoEReload, the
// driver this registers.

package fixture

func init() {
	Register("pppoe/reload", tunnelPPPoEReload)
}
