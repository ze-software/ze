// Design: plan/learned/956-ospf-2-wire.md -- packet body round-trip tests

package packet

import (
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// VALIDATES: AC-2 - Hello fixed fields and neighbor list round-trip.
// PREVENTS: adjacency bring-up using corrupt intervals, DR/BDR, or neighbor IDs.
func TestOSPFHelloRoundTrip(t *testing.T) {
	hello := sampleHello(t)
	p := Packet{Header: sampleHeader(t, PacketTypeHello), Hello: &hello}
	buf := encodePacket(t, p)
	got, err := DecodePacket(buf)
	if err != nil {
		t.Fatalf("DecodePacket hello: %v", err)
	}
	if got.Hello == nil || got.Hello.HelloInterval != 10 || got.Hello.DeadInterval != 40 || got.Hello.Priority != 1 || len(got.Hello.Neighbors) != 2 {
		t.Fatalf("decoded hello wrong: %+v", got.Hello)
	}
	if got.Hello.DR != [4]byte{10, 0, 0, 1} || got.Hello.BDR != [4]byte{10, 0, 0, 2} {
		t.Fatalf("DR/BDR wrong: %+v", got.Hello)
	}
}

// VALIDATES: AC-13 - DD fields, I/M/MS bits, and LSA headers round-trip.
// PREVENTS: neighbor NSM negotiating with wrong master/slave or summary list.
func TestOSPFDDRoundTrip(t *testing.T) {
	for flags := range uint8(8) {
		header := sampleLSAHeader(t, types.LSTypeRouter, "10.0.0.1")
		header.Length = types.LSAHeaderLen
		p := Packet{Header: sampleHeader(t, PacketTypeDBDesc), DBDesc: &DBDesc{
			InterfaceMTU: 1500,
			Options:      types.OptionE | types.OptionO,
			Flags:        flags,
			DDSequence:   0x01020304,
			Headers:      []LSAHeader{header},
		}}
		buf := encodePacket(t, p)
		got, err := DecodePacket(buf)
		if err != nil {
			t.Fatalf("DecodePacket DD flags=%#x: %v", flags, err)
		}
		if got.DBDesc == nil || got.DBDesc.Flags != flags || got.DBDesc.Flags&DDFlagInit != flags&DDFlagInit || got.DBDesc.Flags&DDFlagMore != flags&DDFlagMore || got.DBDesc.Flags&DDFlagMaster != flags&DDFlagMaster {
			t.Fatalf("decoded DD flags wrong: got %+v want %#x", got.DBDesc, flags)
		}
		if len(got.DBDesc.Headers) != 1 || got.DBDesc.Headers[0].Key() != p.DBDesc.Headers[0].Key() {
			t.Fatalf("decoded DD headers wrong: %+v", got.DBDesc.Headers)
		}
	}
}

// VALIDATES: AC-14 - Link State Request triples round-trip.
// PREVENTS: LSDB synchronization requesting the wrong LSA identity.
func TestOSPFLSReqRoundTrip(t *testing.T) {
	p := Packet{Header: sampleHeader(t, PacketTypeLSReq), LSReq: &LSReq{Requests: []LSRequestEntry{{
		Type:              types.LSTypeNetwork,
		LinkStateID:       mustLSID(t, "10.0.0.254"),
		AdvertisingRouter: mustRouterID(t, "10.0.0.1"),
	}}}}
	buf := encodePacket(t, p)
	got, err := DecodePacket(buf)
	if err != nil {
		t.Fatalf("DecodePacket LSReq: %v", err)
	}
	if got.LSReq == nil || len(got.LSReq.Requests) != 1 || got.LSReq.Requests[0] != p.LSReq.Requests[0] {
		t.Fatalf("decoded LSReq wrong: %+v", got.LSReq)
	}
}

// VALIDATES: AC-12 - LS Update count and Length-driven LSA iteration round-trip.
// PREVENTS: flooding code dropping or over-reading LSAs in an update.
func TestOSPFLSUpdateRoundTrip(t *testing.T) {
	p := Packet{Header: sampleHeader(t, PacketTypeLSUpdate), LSUpdate: &LSUpdate{LSAs: []LSA{sampleRouterLSA(t), sampleNetworkLSA(t)}}}
	buf := encodePacket(t, p)
	got, err := DecodePacket(buf)
	if err != nil {
		t.Fatalf("DecodePacket LSUpdate: %v", err)
	}
	if got.LSUpdate == nil || len(got.LSUpdate.LSAs) != 2 {
		t.Fatalf("decoded LSUpdate wrong: %+v", got.LSUpdate)
	}
	if got.LSUpdate.LSAs[0].Header.Type != types.LSTypeRouter || got.LSUpdate.LSAs[1].Header.Type != types.LSTypeNetwork {
		t.Fatalf("decoded LSA types wrong: %+v", got.LSUpdate.LSAs)
	}
}

// VALIDATES: AC-15 - LS Ack carries consecutive 20-byte LSA headers.
// PREVENTS: flooding acks from acknowledging the wrong LSA key.
func TestOSPFLSAckRoundTrip(t *testing.T) {
	header := sampleLSAHeader(t, types.LSTypeRouter, "10.0.0.1")
	header.Length = types.LSAHeaderLen
	p := Packet{Header: sampleHeader(t, PacketTypeLSAck), LSAck: &LSAck{Headers: []LSAHeader{header}}}
	buf := encodePacket(t, p)
	got, err := DecodePacket(buf)
	if err != nil {
		t.Fatalf("DecodePacket LSAck: %v", err)
	}
	if got.LSAck == nil || len(got.LSAck.Headers) != 1 || got.LSAck.Headers[0].Key() != header.Key() {
		t.Fatalf("decoded LSAck wrong: %+v", got.LSAck)
	}
}

// VALIDATES: AC-17 - LS Update count is bounded before allocating the LSA slice.
// PREVENTS: malformed packets with huge counts exhausting memory before validation.
func TestOSPFLSUpdateRejectsHugeCountBeforeAllocation(t *testing.T) {
	body := []byte{0xff, 0xff, 0xff, 0xff}
	if _, err := DecodeLSUpdate(body); !errors.Is(err, ErrLength) {
		t.Fatalf("DecodeLSUpdate huge count err = %v, want %v", err, ErrLength)
	}
}

// VALIDATES: AC-15 - DecodeLSAck rejects a body that is not a whole number of 20-octet LSA
// headers with ErrLength, and propagates a malformed inner header (unknown LS type) instead of
// returning a partial ack; a valid multiple-of-20 body decodes.
// PREVENTS: an LS Ack decoder desyncing on a truncated header or accepting an unknown LSA type.
func TestDecodeLSAckRejectsMalformed(t *testing.T) {
	if _, err := DecodeLSAck(make([]byte, types.LSAHeaderLen+1)); !errors.Is(err, ErrLength) {
		t.Errorf("DecodeLSAck(non-multiple) err = %v, want ErrLength", err)
	}
	// A full-width but all-zero header: LS type 0 is not a known LSA type.
	if _, err := DecodeLSAck(make([]byte, types.LSAHeaderLen)); !errors.Is(err, ErrUnknownLSAType) {
		t.Errorf("DecodeLSAck(zero header) err = %v, want ErrUnknownLSAType", err)
	}
	// A well-formed two-header body decodes to two acknowledged keys.
	h := sampleLSAHeader(t, types.LSTypeRouter, "10.0.0.1")
	h.Length = types.LSAHeaderLen
	body := make([]byte, 2*types.LSAHeaderLen)
	writeLSAHeader(h, body, 0)
	writeLSAHeader(h, body, types.LSAHeaderLen)
	ack, err := DecodeLSAck(body)
	if err != nil || len(ack.Headers) != 2 {
		t.Fatalf("DecodeLSAck(valid) = %d headers err=%v, want 2 headers", len(ack.Headers), err)
	}
}

// VALIDATES: AC-14 - DecodeLSReq rejects a body that is not a whole number of 12-octet entries
// (ErrLength), an LS type wider than one octet (ErrUnknownLSAType), and an in-range but unknown
// LS type (ErrUnknownLSAType).
// PREVENTS: an LS Request decoder mis-framing entries or accepting an unimplemented LSA type.
// RFC requirement: RFC2328-13-1 negative -- only LS types 1-5 (plus the RFC 5250 opaque types) are defined: an in-range but unknown LS type and a type wider than one octet are both rejected with ErrUnknownLSAType, so an unknown-type LSA never reaches the flooding procedure (LSType.Known types/lstype.go:99-119, DecodeLSAHeader lsa.go:40-43).
func TestDecodeLSReqRejectsMalformed(t *testing.T) {
	if _, err := DecodeLSReq(make([]byte, types.LSRequestEntryLen-1)); !errors.Is(err, ErrLength) {
		t.Errorf("DecodeLSReq(non-multiple) err = %v, want ErrLength", err)
	}
	// LS type field 0x00000100 > 0xff: the OSPFv2 type octet cannot exceed one octet.
	wide := make([]byte, types.LSRequestEntryLen)
	writeUint32(wide, 0, 0x0100)
	if _, err := DecodeLSReq(wide); !errors.Is(err, ErrUnknownLSAType) {
		t.Errorf("DecodeLSReq(wide type) err = %v, want ErrUnknownLSAType", err)
	}
	// LS type 6 fits one octet but is not a known OSPFv2 LSA type.
	unknown := make([]byte, types.LSRequestEntryLen)
	writeUint32(unknown, 0, 6)
	if _, err := DecodeLSReq(unknown); !errors.Is(err, ErrUnknownLSAType) {
		t.Errorf("DecodeLSReq(unknown type 6) err = %v, want ErrUnknownLSAType", err)
	}
}
