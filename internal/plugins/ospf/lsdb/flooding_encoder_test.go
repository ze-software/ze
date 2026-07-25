// VALIDATES: the address-family packet-encoder wiring in flooding.go and the RFC 3623
// content-change observer in lsdb.go. NewV4PacketEncoder stamps the RFC 6549 OSPFv2
// Instance ID into the flooded LSUpdate/LSAck common header; SetPacketEncoder makes the
// flood path use that encoder; SetContentChangeObserver fires on a newer received LSA so
// the OSPF Graceful Restart helper sees each content change.
// PREVENTS: a non-base Instance ID that never reaches the wire, and a GR observer that
// stays silent when a neighbor floods a content change during the strict-LSA-checking exit.
package lsdb

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func TestNewV4PacketEncoderStampsInstanceID(t *testing.T) {
	enc := NewV4PacketEncoder(9)
	lsa := routerLSA(t, rid("2.2.2.2"), types.InitialSequenceNumber, 10)

	up := enc.EncodeLSUpdate(rid("1.1.1.1"), area("0.0.0.0"), packet.LSUpdate{LSAs: []packet.LSA{lsa}})
	pUp, err := packet.DecodePacket(up)
	if err != nil {
		t.Fatalf("DecodePacket(LSUpdate): %v", err)
	}
	if pUp.Header.InstanceID != 9 {
		t.Fatalf("LSUpdate Instance ID = %d, want 9", pUp.Header.InstanceID)
	}
	if pUp.LSUpdate == nil || len(pUp.LSUpdate.LSAs) != 1 {
		t.Fatalf("LSUpdate did not round-trip one LSA: %+v", pUp.LSUpdate)
	}

	ack := enc.EncodeLSAck(rid("1.1.1.1"), area("0.0.0.0"), packet.LSAck{Headers: []packet.LSAHeader{lsa.Header}})
	pAck, err := packet.DecodePacket(ack)
	if err != nil {
		t.Fatalf("DecodePacket(LSAck): %v", err)
	}
	if pAck.Header.InstanceID != 9 {
		t.Fatalf("LSAck Instance ID = %d, want 9", pAck.Header.InstanceID)
	}
	if pAck.LSAck == nil || len(pAck.LSAck.Headers) != 1 {
		t.Fatalf("LSAck did not round-trip one header: %+v", pAck.LSAck)
	}
}

func TestSetPacketEncoderUsedByFlood(t *testing.T) {
	db, tx, _ := opaqueOriginateDB(t)
	db.SetPacketEncoder(NewV4PacketEncoder(7))

	if _, ok := db.OriginateOpaque(OpaqueOriginateInput{
		Router: rid("1.1.1.1"), OpaqueType: 1, OpaqueID: 0x01, Scope: types.LSTypeOpaqueArea,
		Area: area("0.0.0.0"), Options: types.OptionO, Body: []byte{0xaa, 0xbb, 0xcc, 0xdd},
	}); !ok {
		t.Fatalf("originate area opaque failed")
	}

	sawUpdate := false
	for i := range tx.sends {
		if tx.sends[i].pkt.LSUpdate == nil {
			continue
		}
		sawUpdate = true
		if tx.sends[i].pkt.Header.InstanceID != 7 {
			t.Fatalf("flooded LSUpdate Instance ID = %d, want 7 (custom encoder not used)", tx.sends[i].pkt.Header.InstanceID)
		}
	}
	if !sawUpdate {
		t.Fatalf("no LSUpdate flooded to assert the encoder on")
	}
}

func TestSetContentChangeObserverFiresOnNewerReceive(t *testing.T) {
	db, _, _ := opaqueOriginateDB(t) // opaqueTopology: eth0 in area 0.0.0.0, self 1.1.1.1
	a0 := area("0.0.0.0")
	type change struct {
		area types.AreaID
		typ  types.LSType
	}
	var changes []change
	db.SetContentChangeObserver(func(a types.AreaID, ty types.LSType) {
		changes = append(changes, change{area: a, typ: ty})
	})

	// A newer Router-LSA from another router installs as Newer -> content change observed.
	lsa := routerLSA(t, rid("2.2.2.2"), types.InitialSequenceNumber, 10)
	if reason := db.ReceiveUpdate(ReceiveInput{
		Interface: "eth0", AreaID: a0, RouterID: rid("2.2.2.2"),
		Src:    netip.MustParseAddr("10.0.0.2"),
		Update: packet.LSUpdate{LSAs: []packet.LSA{lsa}},
	}); reason != "" {
		t.Fatalf("ReceiveUpdate rejected: %q", reason)
	}
	if len(changes) != 1 || changes[0].area != a0 || changes[0].typ != types.LSTypeRouter {
		t.Fatalf("content-change observer fired with %+v, want one {%v, router}", changes, a0)
	}

	// Re-receiving the identical instance is a duplicate (Equal), not a content change.
	if reason := db.ReceiveUpdate(ReceiveInput{
		Interface: "eth0", AreaID: a0, RouterID: rid("2.2.2.2"),
		Src:    netip.MustParseAddr("10.0.0.2"),
		Update: packet.LSUpdate{LSAs: []packet.LSA{lsa}},
	}); reason != "" {
		t.Fatalf("ReceiveUpdate (duplicate) rejected: %q", reason)
	}
	if len(changes) != 1 {
		t.Fatalf("duplicate receive fired the content-change observer again: %+v", changes)
	}
}
