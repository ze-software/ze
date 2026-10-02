// Design: docs/architecture/ospf/ospf-7-lsdb-flooding.md.
// VALIDATES: RFC 2328 Section 13 steps (1) and (2) through the engine: an LSA with an
// invalid LS checksum, and an LSA of an unknown LS type, are each discarded on their own,
// and the next LSA of the same Link State Update is still processed.
// PREVENTS: one bad LSA discarding the whole Link State Update, so the valid LSAs it
// carries are never installed or acknowledged.
package ospf

import (
	"net/netip"
	"slices"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// lsuAckRecorder keeps the key of every LSA header the LSDB acknowledges.
type lsuAckRecorder struct {
	mu    sync.Mutex
	acked []types.LSAKey
}

func (r *lsuAckRecorder) send(_ string, _ netip.Addr, payload []byte) error {
	p, err := packet.DecodePacket(payload)
	if err != nil {
		return err
	}
	if p.LSAck == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, h := range p.LSAck.Headers {
		r.acked = append(r.acked, h.Key())
	}
	return nil
}

func (r *lsuAckRecorder) has(key types.LSAKey) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Contains(r.acked, key)
}

// exchangeNeighborEngine is floodGateEngine with 2.2.2.2 driven to Exchange and an
// acknowledgment recorder on the LSDB.
func exchangeNeighborEngine(t *testing.T) (*engine, *lsuAckRecorder, func(packet.Packet)) {
	t.Helper()
	eng, _, dispatch := floodGateEngine(t)
	self := mustRouterID(t, "1.1.1.1")
	peer := mustRouterID(t, "2.2.2.2")
	header := packet.Header{RouterID: peer, AreaID: types.BackboneArea}
	dispatch(packet.Packet{Header: header, Hello: floodGateHello(self, true)})
	dispatch(packet.Packet{Header: header, DBDesc: &packet.DBDesc{
		InterfaceMTU: 1500, Options: types.OptionE,
		Flags: packet.DDFlagInit | packet.DDFlagMore | packet.DDFlagMaster, DDSequence: 7,
	}})
	if reason := eng.neighbors.AcceptsFlooding("eth0", peer); reason != "" {
		t.Fatalf("setup: neighbor did not reach Exchange: %s", reason)
	}
	acks := &lsuAckRecorder{}
	eng.lsdb.SetTx(acks.send)
	return eng, acks, dispatch
}

// rawOnlyLSA returns an LSA carried by its raw octets alone, so the packet encoder writes
// them unchanged.
func rawOnlyLSA(raw []byte) packet.LSA {
	return packet.LSA{RawBytes: raw}
}

// unknownTypeLSA is the router-LSA of adv re-typed to LS type 6, which RFC 2328 does not
// define, with a recomputed valid LS checksum.
func unknownTypeLSA(t *testing.T, adv types.RouterID) []byte {
	t.Helper()
	raw := append([]byte(nil), routerLSAForTest(t, adv, types.InitialSequenceNumber, 0).RawBytes...)
	raw[3] = 6
	raw[16], raw[17] = 0, 0
	packet.FinalizeLSAChecksum(raw)
	if !packet.VerifyLSAChecksum(raw) {
		t.Fatal("setup: the re-typed LSA must carry a valid LS checksum")
	}
	if types.LSTypeFromByte(raw[3]).Known() {
		t.Fatal("setup: LS type 6 must be unknown to Ze")
	}
	return raw
}

// RFC requirement: RFC2328-13-1 negative -- in one Link State Update dispatched through the engine from an Exchange neighbor, an LSA whose LS checksum is invalid, and an LSA of unknown LS type 6 with a valid checksum, are each discarded: neither is in the database nor acknowledged (DecodeLSUpdate skips the unknown type, lsupdate.go; ReceiveUpdate skips the bad checksum, flooding.go).
// RFC requirement: RFC2328-13-1 positive -- the LSA that follows the discarded one in the same Link State Update is processed ("get the next one"): it is installed and acknowledged, for both the checksum case and the unknown-type case (DecodeLSUpdate, lsupdate.go; ReceiveUpdate, flooding.go).
func TestRFC2328DiscardedLSAThenNextLSAProcessed(t *testing.T) {
	cases := []struct {
		name string
		bad  func(t *testing.T) ([]byte, types.LSAKey)
	}{
		{name: "bad-checksum", bad: func(t *testing.T) ([]byte, types.LSAKey) {
			lsa := routerLSAForTest(t, mustRouterID(t, "5.5.5.5"), types.InitialSequenceNumber, 0)
			raw := append([]byte(nil), lsa.RawBytes...)
			raw[len(raw)-1] ^= 0xff
			if packet.VerifyLSAChecksum(raw) {
				t.Fatal("setup: the corrupted LSA must fail the LS checksum")
			}
			return raw, lsa.Header.Key()
		}},
		{name: "unknown-type", bad: func(t *testing.T) ([]byte, types.LSAKey) {
			raw := unknownTypeLSA(t, mustRouterID(t, "5.5.5.5"))
			return raw, types.LSAKey{Type: types.LSTypeFromByte(raw[3]), LinkStateID: types.LinkStateID{5, 5, 5, 5}, AdvertisingRouter: mustRouterID(t, "5.5.5.5")}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			eng, acks, dispatch := exchangeNeighborEngine(t)
			badRaw, badKey := tc.bad(t)
			next := routerLSAForTest(t, mustRouterID(t, "4.4.4.4"), types.InitialSequenceNumber, 0)
			header := packet.Header{RouterID: mustRouterID(t, "2.2.2.2"), AreaID: types.BackboneArea}
			dispatch(packet.Packet{Header: header, LSUpdate: &packet.LSUpdate{LSAs: []packet.LSA{rawOnlyLSA(badRaw), next}}})
			eng.lsdb.FlushDelayedAcks("eth0")
			if _, ok := eng.lsdb.Lookup(types.BackboneArea, badKey); ok {
				t.Fatalf("%s: the discarded LSA was installed", tc.name)
			}
			if acks.has(badKey) {
				t.Fatalf("%s: the discarded LSA was acknowledged", tc.name)
			}
			got, ok := eng.lsdb.Lookup(types.BackboneArea, next.Header.Key())
			if !ok || got.Sequence != next.Header.Sequence {
				t.Fatalf("%s: the LSA after the discarded one was not installed: %+v, %v", tc.name, got, ok)
			}
			if !acks.has(next.Header.Key()) {
				t.Fatalf("%s: the LSA after the discarded one was not acknowledged", tc.name)
			}
		})
	}
}
