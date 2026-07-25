// VALIDATES: spec-ospf-ext-9 FIX 4 (RFC 3623 sec A grace clock) -- the opaque delivery seam
// surfaces the RECEIVED LSA's LS age in OpaqueDelivery.Age, so the IPv4 Grace-LSA helper can
// compute remaining grace as GracePeriod - LSAge (the OSPFv3 path already reads lsa.Header.Age).
// PREVENTS: the grace clock silently treating every received Grace-LSA as age 0, which would
// reset the grace window on every retransmit.
package lsdb

import (
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// opaqueLSAWithAge builds an opaque LSA carrying a specific LS age in its header (opaqueLSA
// hardcodes age 0), round-tripped through encode/decode so the wire age is authoritative.
func opaqueLSAWithAge(t *testing.T, scope types.LSType, opaqueType uint8, opaqueID uint32, adv types.RouterID, seq types.LSSequenceNumber, body []byte, age uint16) packet.LSA {
	t.Helper()
	lsa := packet.LSA{
		Header: packet.LSAHeader{
			Age:               types.LSAge(age),
			Options:           types.OptionO,
			Type:              scope,
			LinkStateID:       packet.OpaqueLinkStateID(opaqueType, opaqueID),
			AdvertisingRouter: adv,
			Sequence:          seq,
		},
		Opaque: &packet.OpaqueLSA{Type: scope, Data: body},
	}
	return encodeDecodeLSA(t, lsa)
}

func TestOpaqueDeliveryCarriesLSAge(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	db.SetTx((&txRecorder{}).Send)
	db.SetTopology(opaqueTopology)
	a0 := area("0.0.0.0")

	var deliveries []OpaqueDelivery
	db.SetOpaqueDelivery(func(d OpaqueDelivery) { deliveries = append(deliveries, d) })

	const lsAge = uint16(45)
	lsa := opaqueLSAWithAge(t, types.LSTypeOpaqueArea, 3, 0x00, rid("2.2.2.2"), types.InitialSequenceNumber, []byte{0xaa, 0xbb, 0xcc, 0xdd}, lsAge)
	db.ReceiveUpdate(ReceiveInput{Interface: "eth0", AreaID: a0, RouterID: rid("2.2.2.2"), Src: netip.MustParseAddr("10.0.0.2"), Update: packet.LSUpdate{LSAs: []packet.LSA{lsa}}})

	if len(deliveries) != 1 {
		t.Fatalf("newer opaque install: delivered %d times, want 1", len(deliveries))
	}
	if deliveries[0].Age != lsAge {
		t.Fatalf("OpaqueDelivery.Age = %d, want %d (surfaced from the received LSA header LS age)", deliveries[0].Age, lsAge)
	}
}
