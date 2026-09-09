//go:build linux

// Design: docs/architecture/l2tp/bng-5-pppoe.md -- the compiled ze-test personality
// that proves a PADR replay flood from one MAC leaves the AC's descriptor use flat.
// Related: tunnel_fixture_pppoe_padr_flood_linux.go -- tunnelPPPoEFloodReplay, the raw
// AF_PACKET client this registers.

package fixture

func init() {
	Register("pppoe/padr-flood", tunnelPPPoEFloodReplay)
}
