package message

import (
	"encoding/hex"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// TestBuildVPN_AddPathPathIDZero verifies that a VPN UPDATE built for a family
// with ADD-PATH negotiated carries the four-octet Path Identifier even when the
// identifier is zero.
//
// VALIDATES: With AddPath true, BuildVPN writes the Path Identifier before the
// Length octet for identifier 0 and identifier 7 alike, and the MP_REACH_NLRI
// attribute length counts those four octets. With AddPath false, no Path
// Identifier is written whatever the identifier holds.
//
// PREVENTS: Zero read as "no path identifier". RFC 7911 Section 3 gives the
// field four octets and reserves no value, so a VPN NLRI written without it
// for identifier 0 is misframed: the receiver reads the Length octet, the label
// and the first RD octets as the Path Identifier.
//
// Method: build the UPDATE, extract the MP_REACH_NLRI value through its own
// attribute header, and compare the whole value as hex: AFI 1, SAFI 128, next
// hop length 12, zero RD, next hop 192.168.1.1, reserved octet, then the NLRI.
func TestBuildVPN_AddPathPathIDZero(t *testing.T) {
	// Fixed MP_REACH_NLRI prefix: AFI(2) SAFI(1) NH-len(1) RD(8) NH(4) Reserved(1).
	const mpReachHead = "0001" + "80" + "0c" + "0000000000000000" + "c0a80101" + "00"
	// VPN NLRI after any Path Identifier: Length 112 bits (label 24 + RD 64 +
	// prefix 24), label 100 with bottom-of-stack set, RD 1:100:100, 10.0.0.0/24.
	const vpnNLRI = "70" + "000641" + "0001000000640064" + "0a0000"

	tests := []struct {
		name    string
		addPath bool
		pathID  uint32
		want    string
	}{
		{name: "add-path, path id 0", addPath: true, pathID: 0, want: mpReachHead + "00000000" + vpnNLRI},
		{name: "add-path, path id 7", addPath: true, pathID: 7, want: mpReachHead + "00000007" + vpnNLRI},
		{name: "no add-path, path id 7", addPath: false, pathID: 7, want: mpReachHead + vpnNLRI},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ub := NewUpdateBuilder(65001, false, true, tt.addPath)
			params := VPNParams{
				Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
				PathID:  tt.pathID,
				NextHop: netip.MustParseAddr("192.168.1.1"),
				Origin:  attribute.OriginIGP,
				Labels:  []uint32{100},
				RDBytes: [8]byte{0, 1, 0, 0, 0, 100, 0, 100},
			}

			nlriBytes := ub.buildVPNNLRIBytes(&params)
			nlriWant := len(tt.want)/2 - len(mpReachHead)/2
			if len(nlriBytes) != nlriWant {
				t.Fatalf("buildVPNNLRIBytes length = %d, want %d (%x)", len(nlriBytes), nlriWant, nlriBytes)
			}

			update := ub.BuildVPN(&params)
			if update == nil {
				t.Fatal("BuildVPN returned nil")
				return
			}
			got := hex.EncodeToString(extractMPReachValue(t, update.PathAttributes))
			if got != tt.want {
				t.Fatalf("MP_REACH_NLRI value\n got  %s\n want %s", got, tt.want)
			}
		})
	}
}
