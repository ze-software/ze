// Design: docs/architecture/wire/isis.md -- ISO 9542 ISH and RFC 1195 NET addressing.
// Goal: preserve the configured router identity on the ISH wire path.
// Method: encode complete ISHs, compare their NET bytes, and decode valid and corrupt PDUs.

package packet

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// RFC requirement: RFC1195-3.3-1 positive -- the production ISH encoder emits
// a configured GOSIP NET with both reserved address octets zero.
func TestRFC1195ISHGOSIPReservedZero(t *testing.T) {
	net, err := types.ParseNET("47.0005.8000.0000.0000.fc00.0001.001b.2143.6587.00")
	if err != nil {
		t.Fatal(err)
	}
	ish := ISH{NET: net, HoldingTime: 30, TLVs: []TLV{{Type: TLVProtocolsSupported, Value: []byte{0xcc}}}}
	var buf [254]byte
	end := ish.WriteTo(buf[:], 0)
	want := []byte{0x47, 0, 5, 0x80, 0, 0, 0, 0, 0, 0xfc, 0, 0, 1, 0, 0x1b, 0x21, 0x43, 0x65, 0x87, 0}
	if !bytes.Equal(buf[10:30], want) {
		t.Fatalf("ISH NET = %x, want %x", buf[10:30], want)
	}
	decoded, err := DecodeISH(buf[:end])
	if err != nil {
		t.Fatal(err)
	}
	defer ReleaseTLVs(decoded.TLVs)
	if !decoded.NET.Equal(net) {
		t.Fatalf("received NET = %s, want %s", decoded.NET, net)
	}
}

// RFC requirement: RFC1195-3.3-1 negative -- either nonzero GOSIP reserved
// octet is rejected by the NET constructor used by config and ISH reception.
func TestRFC1195ISHGOSIPReservedRefused(t *testing.T) {
	for _, configured := range []string{
		"47.0005.8000.0000.0100.fc00.0001.001b.2143.6587.00",
		"47.0005.8000.0000.0001.fc00.0001.001b.2143.6587.00",
	} {
		if _, err := types.ParseNET(configured); !errors.Is(err, types.ErrNETReserved) {
			t.Fatalf("configured reserved NET error = %v, want ErrNETReserved", err)
		}
	}
	for _, reserved := range []int{17, 18} {
		pdu := []byte{0x82, 30, 1, 0, 4, 0, 30, 0, 0, 20,
			0x47, 0, 5, 0x80, 0, 0, 0, 0, 0, 0xfc, 0, 0, 1, 0, 0x1b, 0x21, 0x43, 0x65, 0x87, 0}
		pdu[reserved] = 1
		if _, err := DecodeISH(pdu); !errors.Is(err, types.ErrNETReserved) {
			t.Fatalf("reserved byte %d error = %v, want ErrNETReserved", reserved, err)
		}
	}
}

// RFC requirement: RFC1195-3.3-2 positive -- an IEEE station identifier supplied
// in canonical form is emitted unchanged in the production ISH NET field.
func TestRFC1195ISHCanonicalIEEEIdentifier(t *testing.T) {
	net, err := types.ParseNET("49.0001.001b.2143.6587.00")
	if err != nil {
		t.Fatal(err)
	}
	ish := ISH{NET: net, HoldingTime: 30}
	var buf [64]byte
	end := ish.WriteTo(buf[:], 0)
	want := []byte{0, 0x1b, 0x21, 0x43, 0x65, 0x87}
	if !bytes.Equal(buf[13:19], want) {
		t.Fatalf("IEEE ID = %x, want canonical %x", buf[13:19], want)
	}
	decoded, err := DecodeISH(buf[:end])
	if err != nil {
		t.Fatal(err)
	}
	defer ReleaseTLVs(decoded.TLVs)
	if decoded.NET.SystemID() != (types.SystemID{0, 0x1b, 0x21, 0x43, 0x65, 0x87}) {
		t.Fatalf("received IEEE ID = %s", decoded.NET.SystemID())
	}
}

// RFC requirement: RFC1195-3.3-2 negative -- ISH decode and encode do not turn
// canonical local bit0x02 into noncanonical bit0x40 in an IP-derived identifier.
func TestRFC1195ISHDoesNotReverseIdentifierBits(t *testing.T) {
	pdu := []byte{0x82, 20, 1, 0, 4, 0, 30, 0, 0, 10,
		0x49, 0, 1, 2, 0, 192, 0, 2, 129, 0}
	decoded, err := DecodeISH(pdu)
	if err != nil {
		t.Fatal(err)
	}
	defer ReleaseTLVs(decoded.TLVs)
	want := types.SystemID{2, 0, 192, 0, 2, 129}
	if decoded.NET.SystemID() != want {
		t.Fatalf("received ID = %s, want %s", decoded.NET.SystemID(), want)
	}
	var buf [64]byte
	end := decoded.WriteTo(buf[:], 0)
	if !bytes.Equal(buf[13:end-1], want[:]) {
		t.Fatalf("re-encoded ID = %x, want canonical %x", buf[13:end-1], want)
	}
}

// ISH framing must honor the PDU length rather than Ethernet padding, and an
// enabled checksum must catch corruption before the NET or options are accepted.
func TestISHLengthAndChecksum(t *testing.T) {
	net, err := types.ParseNET("49.0001.0000.0000.0001.00")
	if err != nil {
		t.Fatal(err)
	}
	ish := ISH{NET: net, HoldingTime: 45, TLVs: []TLV{{Type: TLVProtocolsSupported, Value: []byte{0xcc, 0x8e}}}}
	var buf [64]byte
	end := ish.WriteTo(buf[:], 2)
	decoded, err := DecodeISH(buf[2:])
	if err != nil {
		t.Fatal(err)
	}
	defer ReleaseTLVs(decoded.TLVs)
	if decoded.HoldingTime != 45 {
		t.Fatalf("holding time = %d, want 45", decoded.HoldingTime)
	}
	if len(decoded.TLVs) != 1 {
		t.Fatalf("TLVs = %d, want 1", len(decoded.TLVs))
	}
	if !bytes.Equal(decoded.TLVs[0].Value, []byte{0xcc, 0x8e}) {
		t.Fatalf("protocols = %x", decoded.TLVs[0].Value)
	}
	buf[end-1] ^= 1
	if _, err := DecodeISH(buf[2:]); !errors.Is(err, ErrISHChecksum) {
		t.Fatalf("corrupted ISH error = %v, want ErrISHChecksum", err)
	}
}

// All declared lengths are checked before slicing peer-controlled ISH bytes.
func TestISHTruncatedFields(t *testing.T) {
	for _, pdu := range [][]byte{
		{0x82},
		{0x82, 20, 1, 0, 4, 0, 30, 0, 0, 10, 0x49},
		{0x82, 11, 1, 0, 4, 0, 30, 0, 0, 20, 0x49},
		{0x82, 21, 1, 0, 4, 0, 30, 0, 0, 10, 0x49, 0, 1, 0, 0, 0, 0, 0, 1, 0, 129},
	} {
		if _, err := DecodeISH(pdu); err == nil {
			t.Fatalf("accepted truncated ISH %x", pdu)
		}
	}
}

// FuzzISHDecode exercises every length and field boundary through the production decoder.
func FuzzISHDecode(f *testing.F) {
	f.Add([]byte{0x82, 20, 1, 0, 4, 0, 30, 0, 0, 10, 0x49, 0, 1, 0, 0, 0, 0, 0, 1, 0})
	f.Fuzz(func(_ *testing.T, pdu []byte) {
		ish, err := DecodeISH(pdu)
		if err == nil {
			ReleaseTLVs(ish.TLVs)
		}
	})
}
