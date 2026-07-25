// VALIDATES: spec-ospfv3-2-wire AC-1/AC-14 -- hardcoded wire vectors lock the OSPFv3
// common-header and IPv6-prefix byte layouts so a field-offset or width change is
// caught even when encode/decode round-trips agree with each other.
// PREVENTS: a self-consistent but RFC-wrong encoding passing the round-trip tests
// (the failure mode that hid the AS-External flag/offset bug until review).

package packet

import (
	"bytes"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

func TestOSPFv3CommonHeaderGolden(t *testing.T) {
	p := Packet{
		Header: Header{Type: PacketTypeHello, RouterID: mustRouterID(t, "10.0.0.1"), AreaID: mustAreaID(t, "0"), InstanceID: types.InstanceID(7)},
		Hello:  &Hello{InterfaceID: types.InterfaceID(0x01020304), Priority: 1, HelloInterval: 10, RouterDeadInterval: 40, DR: mustRouterID(t, "10.0.0.1"), BDR: mustRouterID(t, "0.0.0.0")},
	}
	buf := encodePacket(t, p)
	// 36-octet packet: 16-octet common header + 20-octet Hello fixed prefix (no neighbors).
	want := []byte{
		3,          // Version 3
		1,          // Type Hello
		0x00, 0x24, // Packet Length 36
		0x0a, 0x00, 0x00, 0x01, // Router ID 10.0.0.1
		0x00, 0x00, 0x00, 0x00, // Area ID 0.0.0.0
		0x00, 0x00, // Checksum left zero (finalized by transport with the IPv6 src/dst)
		0x07, // Instance ID 7
		0x00, // Reserved
	}
	if !bytes.Equal(buf[:CommonHeaderLen], want) {
		t.Fatalf("common header golden:\n got % x\nwant % x", buf[:CommonHeaderLen], want)
	}
}

func TestOSPFv3PrefixGolden(t *testing.T) {
	// Repeating-entry carriage form: PrefixLength(1) + PrefixOptions(1) + 16-bit field(2)
	// + AddressPrefix(ByteLen). RFC 5340 §A.4.1.
	p := makePrefix(t, 64, types.OptPrefixNU, 0x0064)
	buf := make([]byte, p.encodedLen())
	n := p.writeTo(buf, 0)
	want := []byte{
		0x40,       // PrefixLength 64
		0x01,       // PrefixOptions NU
		0x00, 0x64, // 16-bit field (metric 100 in an Intra-Area-Prefix entry)
		0x20, 0x01, 0x0d, 0xb8, 0x12, 0x34, 0x56, 0x78, // /64 AddressPrefix, no padding
	}
	if n != len(want) || !bytes.Equal(buf[:n], want) {
		t.Fatalf("prefix golden:\n got % x\nwant % x", buf[:n], want)
	}
}
