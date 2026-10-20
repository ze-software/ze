package flowspec

import (
	"bytes"
	"testing"
)

// TestFlowSpecEncodeRouteAddPathWritesPathID checks the NLRI field the FlowSpec
// route encoder (`ze bgp encode -i`) emits with and without ADD-PATH.
//
// Method: encode one route through EncodeRoute twice, addPath false then true,
// and read the NLRI out of the MP_REACH_NLRI value each UPDATE carries.
//
// VALIDATES: RFC 7911 Section 3: with ADD-PATH the 4-octet Path Identifier,
// 0 because the route command names no path, precedes the NLRI; without it the
// NLRI stands alone.
// PREVENTS: the encoder dropping the message.PluginNLRI framing, so a receiver
// with ADD-PATH reads the first NLRI octets as the Path Identifier.
func TestFlowSpecEncodeRouteAddPathWritesPathID(t *testing.T) {
	const routeCmd = "match destination-ipv4 10.0.0.0/8 then discard"
	bareMessage, bareNLRI, err := EncodeRoute(routeCmd, "ipv4/flow", 65000, true, true, false)
	if err != nil {
		t.Fatalf("EncodeRoute without ADD-PATH: %v", err)
	}
	if len(bareNLRI) == 0 {
		t.Fatal("EncodeRoute without ADD-PATH returned no NLRI")
	}
	framedMessage, framedNLRI, err := EncodeRoute(routeCmd, "ipv4/flow", 65000, true, true, true)
	if err != nil {
		t.Fatalf("EncodeRoute with ADD-PATH: %v", err)
	}

	want := append([]byte{0, 0, 0, 0}, bareNLRI...)
	if got := flowSpecMPReachNLRI(t, bareMessage); !bytes.Equal(got, bareNLRI) {
		t.Errorf("without ADD-PATH: MP_REACH NLRI = %x, want %x", got, bareNLRI)
	}
	if !bytes.Equal(framedNLRI, want) {
		t.Errorf("with ADD-PATH: returned NLRI = %x, want %x", framedNLRI, want)
	}
	if got := flowSpecMPReachNLRI(t, framedMessage); !bytes.Equal(got, want) {
		t.Errorf("with ADD-PATH: MP_REACH NLRI = %x, want %x", got, want)
	}
}

// flowSpecMPReachNLRI returns the NLRI field of the MP_REACH_NLRI attribute in
// the UPDATE message EncodeRoute returns: past the 19-octet header, the
// withdrawn routes, the AFI, SAFI, next hop and the reserved octet
// (RFC 4760 Section 3).
func flowSpecMPReachNLRI(t *testing.T, message []byte) []byte {
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
	t.Fatalf("no MP_REACH_NLRI in %x", message)
	return nil
}
