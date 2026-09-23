// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- IntServ Path MTU signaling.
package rsvpte

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"testing"
)

// TestAdspecWireFormat checks the independently specified RFC 2210 Section 3.3
// layout, the unchanged sender maximum, and the MTU returned in a RESV.
func TestAdspecWireFormat(t *testing.T) {
	want := []byte{
		0, 48, 13, 2, 0, 0, 0, 10,
		1, 0, 0, 8, 4, 0, 0, 1, 0, 0, 0, 1,
		6, 0, 0, 1, 0, 0, 0, 0,
		8, 0, 0, 1, 255, 255, 255, 255,
		10, 0, 0, 1, 0, 0, 5, 220,
		5, 0, 0, 0,
	}
	var adspec [48]byte
	if n := encodeAdspec(adspec[:], 1500, serviceControlledLoad); n != len(want) {
		t.Fatalf("encoded length = %d, want %d", n, len(want))
	}
	if !bytes.Equal(adspec[:], want) {
		t.Fatalf("ADSPEC = %x, want %x", adspec, want)
	}
	if err := updateAdspec(adspec[:], 1400); err != nil {
		t.Fatal(err)
	}
	psb := samplePSB()
	psb.Adspec = adspec[:]
	msg, err := DecodeMessage(buildPath(psb, netip.MustParseAddr("10.0.0.1"), 64))
	if err != nil {
		t.Fatal(err)
	}
	if msg.PathMTU != 1400 || msg.SenderTSpec.MaxPacketSize != 1500 {
		t.Fatalf("path MTU = %d, sender maximum = %d", msg.PathMTU, msg.SenderTSpec.MaxPacketSize)
	}
	fs := msg.SenderTSpec
	fs.MaxPacketSize = msg.PathMTU
	rsb := &resvStateBlock{Session: psb.Session, FlowSpec: fs, Label: labelObject{Label: 16000}}
	resv, err := DecodeMessage(buildResv(rsb, psb.SenderTemplate, DefaultRefreshPeriod, psb.Session.TunnelEndpoint))
	if err != nil {
		t.Fatal(err)
	}
	if len(resv.FlowDescriptors) != 1 {
		t.Fatalf("reservation descriptors = %d, want 1", len(resv.FlowDescriptors))
	}
	if got := receivedPathMTU(resv.FlowDescriptors[0].FlowSpec, true); got != 1400 {
		t.Fatalf("returned MTU = %d, want 1400", got)
	}
	if got := receivedPathMTU(resv.FlowDescriptors[0].FlowSpec, false); got != 0 {
		t.Fatalf("unsolicited maximum became discovered MTU: %d", got)
	}
}

// TestAdspecUnknownAndOverride exercises missing advertisements, incomplete
// paths, and the service-specific override that must take precedence.
func TestAdspecUnknownAndOverride(t *testing.T) {
	var storage [56]byte
	if n := encodeAdspec(storage[:], 0, serviceControlledLoad); n != 0 {
		t.Fatalf("unknown MTU emitted %d bytes", n)
	}
	encodeAdspec(storage[:], 9000, serviceControlledLoad)
	adspec := storage[:]
	binary.BigEndian.PutUint16(adspec[:2], 56)
	binary.BigEndian.PutUint16(adspec[6:8], 12)
	binary.BigEndian.PutUint16(adspec[46:48], 2)
	copy(adspec[48:], []byte{10, 0, 0, 1, 0, 0, 5, 120})
	if got := adspecPathMTU(adspec, serviceControlledLoad); got != 1400 {
		t.Fatalf("override MTU = %d, want 1400", got)
	}
	if err := updateAdspec(adspec, 1300); err != nil {
		t.Fatal(err)
	}
	if got := adspecPathMTU(adspec, serviceControlledLoad); got != 1300 {
		t.Fatalf("composed override MTU = %d, want 1300", got)
	}
	adspec[45] = 0x80
	if got := adspecPathMTU(adspec, serviceControlledLoad); got != 0 {
		t.Fatalf("broken service advertised MTU %d", got)
	}
	adspec[45] = 0
	if err := updateAdspec(adspec, 0); err != nil {
		t.Fatal(err)
	}
	if adspec[9]&0x80 == 0 || adspecPathMTU(adspec, serviceControlledLoad) != 0 {
		t.Fatal("unknown link MTU did not mark the advertisement incomplete")
	}
	msg, err := DecodeMessage(buildPath(samplePSB(), netip.MustParseAddr("10.0.0.1"), 64))
	if err != nil {
		t.Fatal(err)
	}
	if msg.PathMTU != 0 || msg.HasAdspec {
		t.Fatal("PATH without ADSPEC acquired a discovered MTU")
	}
}

// TestAdspecMalformed refuses inconsistent nested lengths and invalid MTUs even
// when the outer RSVP object and checksum remain valid.
func TestAdspecMalformed(t *testing.T) {
	cases := []struct {
		name   string
		change func([]byte)
	}{
		{"version", func(b []byte) { b[4] = 0x10 }},
		{"overall-length", func(b []byte) { b[7]-- }},
		{"service-length", func(b []byte) { b[11] = 9 }},
		{"parameter-length", func(b []byte) { b[39] = 2 }},
		{"missing-general", func(b []byte) { b[8] = 5 }},
		{"zero-mtu", func(b []byte) { clear(b[40:44]) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var b [48]byte
			encodeAdspec(b[:], 1500, serviceControlledLoad)
			tc.change(b[:])
			psb := samplePSB()
			psb.Adspec = b[:]
			if _, err := DecodeMessage(buildPath(psb, netip.MustParseAddr("10.0.0.1"), 64)); err == nil {
				t.Fatal("malformed ADSPEC accepted")
			}
		})
	}
}

// TestNullServiceMTU checks RFC 2997's distinct service 6 / parameter 128
// representation and its use of the same composed ADSPEC MTU.
func TestNullServiceMTU(t *testing.T) {
	psb := samplePSB()
	psb.SenderTSpec = FlowSpec{Service: serviceNull, MaxPacketSize: 9000}
	psb.Adspec = make([]byte, 48)
	encodeAdspec(psb.Adspec, 1500, serviceNull)
	msg, err := DecodeMessage(buildPath(psb, netip.MustParseAddr("10.0.0.1"), 64))
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{0, 20, 12, 2, 0, 0, 0, 3, 6, 0, 0, 2, 128, 0, 0, 1, 0, 0, 35, 40}
	if !bytes.Equal(msg.SenderTSpecRaw, want) || msg.PathMTU != 1500 {
		t.Fatalf("null TSPEC = %x, path MTU = %d", msg.SenderTSpecRaw, msg.PathMTU)
	}
	fs := msg.SenderTSpec
	fs.MaxPacketSize = msg.PathMTU
	rsb := &resvStateBlock{Session: psb.Session, FlowSpec: fs, Label: labelObject{Label: 16000}}
	resv, err := DecodeMessage(buildResv(rsb, psb.SenderTemplate, DefaultRefreshPeriod, psb.Session.TunnelEndpoint))
	if err != nil {
		t.Fatal(err)
	}
	if len(resv.FlowDescriptors) != 1 {
		t.Fatalf("reservation descriptors = %d, want 1", len(resv.FlowDescriptors))
	}
	if resv.FlowDescriptors[0].FlowSpec.Service != serviceNull || receivedPathMTU(resv.FlowDescriptors[0].FlowSpec, true) != 1500 {
		t.Fatalf("null return = %+v", resv.FlowDescriptors[0].FlowSpec)
	}
}

// TestAdspecOpaqueServicePreserved checks unknown service bytes survive transit
// while the service break bit records that this node could not process them.
func TestAdspecOpaqueServicePreserved(t *testing.T) {
	var b [56]byte
	encodeAdspec(b[:], 1500, serviceControlledLoad)
	binary.BigEndian.PutUint16(b[:2], 56)
	binary.BigEndian.PutUint16(b[6:8], 12)
	copy(b[48:], []byte{42, 0, 0, 1, 0xde, 0xad, 0xbe, 0xef})
	if err := updateAdspec(b[:], 1400); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(b[48:], []byte{42, 0x80, 0, 1, 0xde, 0xad, 0xbe, 0xef}) {
		t.Fatalf("opaque service changed: %x", b[48:])
	}
}

// TestMixedSenderTSpecPreserved checks that the optional RFC 2997 Null Service
// TSpec following a standard token bucket survives a PATH relay unchanged.
func TestMixedSenderTSpecPreserved(t *testing.T) {
	var raw [48]byte
	psb := samplePSB()
	encodeFlowSpec(raw[:], ClassSenderTSpec, psb.SenderTSpec)
	binary.BigEndian.PutUint16(raw[:2], 48)
	binary.BigEndian.PutUint16(raw[6:8], 10)
	copy(raw[36:], []byte{6, 0, 0, 2, 128, 0, 0, 1, 0, 0, 35, 40})
	psb.SenderTSpecRaw = raw[:]
	msg, err := DecodeMessage(buildPath(psb, netip.MustParseAddr("10.0.0.1"), 64))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(msg.SenderTSpecRaw, raw[:]) || msg.SenderTSpec.MaxPacketSize != 1500 {
		t.Fatalf("mixed TSpec changed: %x, %+v", msg.SenderTSpecRaw, msg.SenderTSpec)
	}
}

// TestReturnedFlowSpecPreservesRaw checks that an updated MTU changes only M
// in a retained receiver object, including its reserved bits.
func TestReturnedFlowSpecPreservesRaw(t *testing.T) {
	var raw [36]byte
	fs := samplePSB().SenderTSpec
	encodeFlowSpec(raw[:], ClassFlowSpec, fs)
	raw[5] = 0x12
	raw[9] = 0x02
	rsb := &resvStateBlock{Session: samplePSB().Session, FlowSpec: fs,
		FlowSpecRaw: raw[:], Label: labelObject{Label: 16000}}
	rsb.FlowSpec.MaxPacketSize = 1400
	msg, err := DecodeMessage(buildResv(rsb, samplePSB().SenderTemplate, DefaultRefreshPeriod,
		netip.MustParseAddr("10.0.0.9")))
	if err != nil {
		t.Fatal(err)
	}
	if len(msg.FlowDescriptors) != 1 {
		t.Fatalf("reservation descriptors = %d, want 1", len(msg.FlowDescriptors))
	}
	binary.BigEndian.PutUint32(raw[32:36], 1400)
	if !bytes.Equal(msg.FlowDescriptors[0].FlowSpecRaw, raw[:]) {
		t.Fatalf("retained FLOWSPEC changed: %x, want %x", msg.FlowDescriptors[0].FlowSpecRaw, raw)
	}
}
