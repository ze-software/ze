// Design: docs/architecture/isis/isis-3-l2-transport.md -- RFC 1195 section 4.1
// receive encapsulation.
//
// Goal: prove that this IP-only router receives IS-IS PDUs carried in the normal
// OSI encapsulation (IEEE 802.3 length field plus LLC 0xFE/0xFE/0x03) and refuses
// a frame that does not use it. Method: build one frame byte for byte, parse it,
// and compare the PDU view; then corrupt only the encapsulation and parse again.

package transport

import (
	"bytes"
	"errors"
	"testing"
)

// osiFrame returns a captured 802.3 plus LLC frame carrying pdu, built by hand so
// the test does not depend on BuildFrame agreeing with ParseFrame.
func osiFrame(pdu []byte) []byte {
	frame := make([]byte, FrameHeaderLen+len(pdu))
	copy(frame[0:MACLen], []byte{0x01, 0x80, 0xC2, 0x00, 0x00, 0x14})
	copy(frame[MACLen:2*MACLen], []byte{0x02, 0x00, 0x00, 0x00, 0x00, 0x01})
	llcAndPDU := LLCHeaderLen + len(pdu)
	frame[2*MACLen] = byte(llcAndPDU >> 8)
	frame[2*MACLen+1] = byte(llcAndPDU)
	frame[2*MACLen+2] = LLCSAP
	frame[2*MACLen+3] = LLCSAP
	frame[2*MACLen+4] = LLCControl
	copy(frame[FrameHeaderLen:], pdu)
	return frame
}

// RFC requirement: RFC1195-4.1-1 positive -- a frame using the normal OSI
// encapsulation (802.3 length field, LLC DSAP 0xFE, SSAP 0xFE, control 0x03) is
// accepted and its IS-IS PDU is returned intact.
func TestRFC1195ParseFrameOSIEncapsulation(t *testing.T) {
	pdu := []byte{0x83, 0x1B, 0x01, 0x00, 0x0F, 0x01, 0x00, 0x00}
	f, err := ParseFrame(osiFrame(pdu))
	if err != nil {
		t.Fatalf("ParseFrame refused the OSI encapsulation: %v", err)
	}
	if !bytes.Equal(f.PDU, pdu) {
		t.Fatalf("PDU = % x, want % x", f.PDU, pdu)
	}
	if f.SrcMAC != [MACLen]byte{0x02, 0x00, 0x00, 0x00, 0x00, 0x01} {
		t.Fatalf("source MAC = % x, want 02:00:00:00:00:01", f.SrcMAC)
	}
}

// RFC requirement: RFC1195-4.1-1 negative -- a frame that is not the normal OSI
// encapsulation is refused: an Ethernet II ethertype in place of the 802.3 length
// is ErrNotISO, and an LLC header other than 0xFE/0xFE/0x03 is ErrBadLLC. No PDU
// is returned for either.
func TestRFC1195ParseFrameRefusesNonOSIEncapsulation(t *testing.T) {
	pdu := []byte{0x83, 0x1B, 0x01, 0x00, 0x0F, 0x01, 0x00, 0x00}

	ethernetII := osiFrame(pdu)
	ethernetII[2*MACLen] = 0x08 // ethertype 0x0800 (IPv4), not a length
	ethernetII[2*MACLen+1] = 0x00
	f, err := ParseFrame(ethernetII)
	if !errors.Is(err, ErrNotISO) {
		t.Fatalf("Ethernet II frame: err = %v, want ErrNotISO", err)
	}
	if f.PDU != nil {
		t.Fatalf("Ethernet II frame returned a PDU: % x", f.PDU)
	}

	snap := osiFrame(pdu)
	snap[2*MACLen+2] = 0xAA // SNAP DSAP, not ISO CLNS
	snap[2*MACLen+3] = 0xAA
	f, err = ParseFrame(snap)
	if !errors.Is(err, ErrBadLLC) {
		t.Fatalf("SNAP LLC frame: err = %v, want ErrBadLLC", err)
	}
	if f.PDU != nil {
		t.Fatalf("SNAP LLC frame returned a PDU: % x", f.PDU)
	}
}
