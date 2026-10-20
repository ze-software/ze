package srpolicy

import (
	"bytes"
	"encoding/hex"
	"testing"
)

// TestSRPolicyEncodeRouteAddPathWritesPathID checks the exact NLRI field the
// SR Policy route encoder emits with and without ADD-PATH, for both AFIs.
//
// Method: encode one route through EncodeRoute (the `ze bgp encode` entry) and
// read the NLRI out of the MP_REACH_NLRI value it built.
//
// VALIDATES: RFC 7911 Section 3: with ADD-PATH the 4-octet Path Identifier
// precedes the 12- or 24-octet SR Policy NLRI; without it the NLRI stands alone.
// PREVENTS: a Ze receiver with ADD-PATH receive reading the Distinguisher as the
// Path Identifier and resetting the session (srPolicyNLRIBounds).
func TestSRPolicyEncodeRouteAddPathWritesPathID(t *testing.T) {
	const ipv4NLRI = "60" + "00000000" + "00000064" + "0a000001"
	const ipv6NLRI = "c0" + "00000000" + "00000064" + "20010db8000000000000000000000001"
	tests := []struct {
		name     string
		family   string
		endpoint string
		addPath  bool
		wantNLRI string
	}{
		{"ipv4 without add-path", "ipv4/sr-policy", "10.0.0.1", false, ipv4NLRI},
		{"ipv4 with add-path", "ipv4/sr-policy", "10.0.0.1", true, "00000000" + ipv4NLRI},
		{"ipv6 without add-path", "ipv6/sr-policy", "2001:db8::1", false, ipv6NLRI},
		{"ipv6 with add-path", "ipv6/sr-policy", "2001:db8::1", true, "00000000" + ipv6NLRI},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := "distinguisher 0 color 100 endpoint " + tt.endpoint +
				" next-hop 192.0.2.1 preference 100 segment-list weight 1 segment type-a mpls 16001"
			body, nlri, err := EncodeRoute(cmd, tt.family, 65000, true, true, tt.addPath)
			if err != nil {
				t.Fatalf("EncodeRoute: %v", err)
			}
			want, err := hex.DecodeString(tt.wantNLRI)
			if err != nil {
				t.Fatalf("bad want hex: %v", err)
			}
			got := mpReachNLRIField(t, body)
			if !bytes.Equal(got, want) {
				t.Errorf("MP_REACH NLRI = %x, want %x", got, want)
			}
			if !bytes.Equal(nlri, want) {
				t.Errorf("returned NLRI = %x, want %x", nlri, want)
			}
		})
	}
}

// mpReachNLRIField returns the NLRI field of the MP_REACH_NLRI attribute in an
// UPDATE message EncodeRoute returns (19-octet header, withdrawn length,
// attribute length, attributes).
func mpReachNLRIField(t *testing.T, message []byte) []byte {
	t.Helper()
	if len(message) < 23 {
		t.Fatalf("UPDATE too short: %x", message)
	}
	body := message[19:]
	withdrawnOctets := int(body[0])<<8 | int(body[1])
	off := 2 + withdrawnOctets
	attrOctets := int(body[off])<<8 | int(body[off+1])
	attrs := body[off+2 : off+2+attrOctets]
	for len(attrs) >= 3 {
		flags, code := attrs[0], attrs[1]
		hdr, valueOctets := 3, int(attrs[2])
		if flags&0x10 != 0 {
			hdr, valueOctets = 4, int(attrs[2])<<8|int(attrs[3])
		}
		value := attrs[hdr : hdr+valueOctets]
		attrs = attrs[hdr+valueOctets:]
		if code != 14 {
			continue
		}
		nextHopOctets := int(value[3])
		return value[4+nextHopOctets+1:]
	}
	t.Fatalf("no MP_REACH_NLRI in %x", body)
	return nil
}
