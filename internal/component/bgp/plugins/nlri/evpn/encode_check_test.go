package evpn_test

import (
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/nlri/evpn"
)

func TestEVPNEncodeRouteLength(t *testing.T) {
	hex1, err := evpn.EncodeNLRIHex("l2vpn/evpn", []string{
		"type2", "rd", "1:1", "esi", "00:00:00:00:00:00:00:00:00:00",
		"ethernet-tag", "0", "mac", "00:11:22:33:44:55", "label", "100",
	})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := hex.DecodeString(strings.ToLower(hex1))
	fmt.Printf("No IP: hex=%s type=%d routeLen=%d(0x%02x) totalBytes=%d\n", hex1, b[0], b[1], b[1], len(b))

	// Route type = 2, route length should be 33 (no IP)
	// RD(8) + ESI(10) + Etag(4) + MACLen(1) + MAC(6) + IPLen(1) + Label(3) = 33
	if b[1] != 33 {
		t.Errorf("Route length without IP: got %d (0x%02x), want 33 (0x21)", b[1], b[1])
	}
}
