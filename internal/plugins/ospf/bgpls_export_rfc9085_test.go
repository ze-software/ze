// Design: docs/architecture/wire/nlri-bgpls.md -- native OSPF BGP-LS origination.
// RFC: rfc/short/rfc9085.md
// Related: bgpls_export.go -- routerInformation translates the RI SRGB and SRLB
//
// VALIDATES: the SR Capabilities TLV 1034 and the SR Local Block TLV 1036 Ze
// originates from an OSPFv2 Router Information LSA carry a Flags octet and a
// Reserved octet of 0, as RFC 9085 Sections 2.1.2 and 2.1.4 require for OSPF,
// whatever the native TLV carried in its own reserved octet.
// PREVENTS: a native OSPF reserved octet, or any source bit, reaching the
// Flags or Reserved octet a collector reads.
package ospf

import (
	"bytes"
	"testing"

	"github.com/ze-software/ze/internal/core/linkstateevents"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/sr"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// rfc9085OriginatedRanges installs a Router Information LSA carrying one SRGB
// (base 16000, size 8000) and one SRLB (base 15000, size 1000), each with the
// given native reserved octet, and returns the TLV 1034 and TLV 1036 values Ze
// originates for the advertising router.
func rfc9085OriginatedRanges(t *testing.T, reserved byte) (srgb, srlb []byte) {
	t.Helper()
	e, bus, events := bgplsTestSource(t, false)
	if !e.lsdb.Install(types.BackboneArea, bgplsTestRouter(10, types.InitialSequenceNumber)) {
		t.Fatal("install base router")
	}
	global := sr.EncodeRangeValue(sr.LabelRange{Base: 16000, Size: 8000})
	local := sr.EncodeRangeValue(sr.LabelRange{Base: 15000, Size: 1000})
	// RFC 8665 Sections 3.2/3.3: value octet 3 is the native Reserved octet.
	global[3] = reserved
	local[3] = reserved
	bgplsInstallOpaque(t, e, packet.RIOpaqueType, packet.EncodeRITLVs([]packet.RITLV{
		{Type: sr.V4TypeSRGB, Value: global}, {Type: sr.V4TypeSRLB, Value: local},
	}))
	if _, err := linkstateevents.Request.Emit(bus); err != nil {
		t.Fatal(err)
	}
	snapshot := bgplsAwait(t, events, func(s *linkstateevents.Snapshot) bool {
		for i := range s.Nodes {
			if bgplsTestAttribute(s.Nodes[i].Attributes, 1036) != nil {
				return true
			}
		}
		return false
	})
	for i := range snapshot.Nodes {
		srlb = bgplsTestAttribute(snapshot.Nodes[i].Attributes, 1036)
		if srlb != nil {
			return bgplsTestAttribute(snapshot.Nodes[i].Attributes, 1034), srlb
		}
	}
	t.Fatal("no node carries TLV 1036")
	return nil, nil
}

// The expected wire values: Flags 0, Reserved 0, Range Size, then SID/Label
// sub-TLV 1161 of length 3 carrying the first label.
var (
	rfc9085OSPFSRGB = []byte{0, 0, 0x00, 0x1f, 0x40, 0x04, 0x89, 0, 3, 0x00, 0x3e, 0x80}
	rfc9085OSPFSRLB = []byte{0, 0, 0x00, 0x03, 0xe8, 0x04, 0x89, 0, 3, 0x00, 0x3a, 0x98}
)

// TestRFC9085OSPFOriginatedCapabilitiesFlagsReservedZero originates an
// ordinary OSPF SRGB and SRLB and compares the exact TLV 1034 and 1036 values.
//
// RFC requirement: RFC9085-2.1.2-1 positive -- the SR Capabilities TLV 1034 Ze originates from an OSPF SRGB has Flags octet 0 (§2.1.2).
// RFC requirement: RFC9085-2.1.2-2 positive -- the SR Capabilities TLV 1034 Ze originates from an OSPF SRGB has Reserved octet 0 (§2.1.2).
// RFC requirement: RFC9085-2.1.2-3 positive -- the SR Capabilities TLV 1034 Ze originates from an OSPF SRGB has Reserved octet 0 (§2.1.2).
// RFC requirement: RFC9085-2.1.2-4 positive -- the SR Capabilities TLV 1034 Ze originates from an OSPF SRGB has Flags octet 0 (§2.1.2).
// RFC requirement: RFC9085-2.1.4-1 positive -- the SR Local Block TLV 1036 Ze originates from an OSPF SRLB has Flags octet 0 (§2.1.4).
// RFC requirement: RFC9085-2.1.4-2 positive -- the SR Local Block TLV 1036 Ze originates from an OSPF SRLB has Reserved octet 0 (§2.1.4).
func TestRFC9085OSPFOriginatedCapabilitiesFlagsReservedZero(t *testing.T) {
	srgb, srlb := rfc9085OriginatedRanges(t, 0)
	if !bytes.Equal(srgb, rfc9085OSPFSRGB) {
		t.Fatalf("TLV 1034 = %x, want %x", srgb, rfc9085OSPFSRGB)
	}
	if !bytes.Equal(srlb, rfc9085OSPFSRLB) {
		t.Fatalf("TLV 1036 = %x, want %x", srlb, rfc9085OSPFSRLB)
	}
}

// TestRFC9085OSPFSourceReservedNeverOriginated hands the producer native
// ranges whose reserved octet is 0xff, the input that reaches the Flags or
// Reserved octet if the producer copies the native header.
//
// RFC requirement: RFC9085-2.1.2-1 negative -- an OSPF SRGB whose native reserved octet is ff is originated as TLV 1034 with Flags octet 0 (§2.1.2).
// RFC requirement: RFC9085-2.1.2-2 negative -- an OSPF SRGB whose native reserved octet is ff is originated as TLV 1034 with Reserved octet 0 (§2.1.2).
// RFC requirement: RFC9085-2.1.2-3 negative -- an OSPF SRGB whose native reserved octet is ff is originated as TLV 1034 with Reserved octet 0 (§2.1.2).
// RFC requirement: RFC9085-2.1.2-4 negative -- an OSPF SRGB whose native reserved octet is ff is originated as TLV 1034 with Flags octet 0 (§2.1.2).
// RFC requirement: RFC9085-2.1.4-1 negative -- an OSPF SRLB whose native reserved octet is ff is originated as TLV 1036 with Flags octet 0 (§2.1.4).
// RFC requirement: RFC9085-2.1.4-2 negative -- an OSPF SRLB whose native reserved octet is ff is originated as TLV 1036 with Reserved octet 0 (§2.1.4).
func TestRFC9085OSPFSourceReservedNeverOriginated(t *testing.T) {
	srgb, srlb := rfc9085OriginatedRanges(t, 0xff)
	if !bytes.Equal(srgb, rfc9085OSPFSRGB) {
		t.Fatalf("TLV 1034 = %x, want %x", srgb, rfc9085OSPFSRGB)
	}
	if !bytes.Equal(srlb, rfc9085OSPFSRLB) {
		t.Fatalf("TLV 1036 = %x, want %x", srlb, rfc9085OSPFSRLB)
	}
}
