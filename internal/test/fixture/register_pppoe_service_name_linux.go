//go:build linux

// Design: docs/labs/pppoe-interop.md -- the compiled ze-test personality that
// dials a matching and a mismatched PPPoE service against a real Ze AC.
// Related: tunnel_fixture_pppoe_linux.go -- tunnelPPPoEServiceName, the raw
// AF_PACKET client this registers.

package fixture

func init() {
	Register("pppoe/service-name", tunnelPPPoEServiceName)
}
