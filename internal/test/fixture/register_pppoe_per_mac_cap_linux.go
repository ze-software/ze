//go:build linux

// Design: docs/architecture/l2tp/bng-5-pppoe.md -- the compiled ze-test personality that
// exercises the per-MAC session cap against a real Ze AC.
// Related: tunnel_fixture_pppoe_per_mac_cap_linux.go -- tunnelPPPoEPerMACCap, the raw
// AF_PACKET client this registers.

package fixture

func init() {
	Register("pppoe/per-mac-cap", tunnelPPPoEPerMACCap)
}
