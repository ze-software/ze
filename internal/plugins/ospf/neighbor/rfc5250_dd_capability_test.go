// Design: docs/architecture/ospf/ospf-ext-1-opaque-framework.md -- negotiated DD contents.
// Related: rfc5250_summary_entire_area_test.go -- real native LSDB fixture helpers.
// RFC: rfc/short/rfc5250.md -- Section 3.1; rfc/short/rfc2328.md -- Section 10.6.
// RFC naming: untagged -- regression for the observed FRR DD capability mismatch, not whole-clause conformance coverage.
package neighbor

import (
	"fmt"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// RFC 5250 Section 3.1: "In the next step of the Database Exchange process,
// Opaque LSAs are included in the Database summary list that is sent to the
// neighbor (see Sections 3.2 below and 10.3 of [OSPF]) when the neighbor is
// opaque capable." A peer's O-bit, not our own or its Hello, supplies that fact.
//
// Observed ospf-te-frr exchange: FRR's master DD had Options 2, I/M/MS and
// sequence 0x14f94b26. Ze's slave response retained Options 66 but advertised
// Type 10, causing FRR's "Opaque capability mismatch?" and SeqNumberMismatch.
func TestDDNegotiatedOpaqueCapabilityControlsAdvertisedSummary(t *testing.T) {
	for _, localMaster := range []bool{false, true} {
		for _, peerOpaque := range []bool{false, true} {
			t.Run(fmt.Sprintf("master-%t/peer-opaque-%t", localMaster, peerOpaque), func(t *testing.T) {
				tbl, cfg := testTable(t, types.NetworkPointToPoint)
				cfg.RouterID = types.RouterID{172, 30, 0, 2}
				peer := types.RouterID{172, 30, 0, 3}
				if localMaster {
					cfg.RouterID, peer = peer, cfg.RouterID
				}
				cfg.Options = types.OptionE | types.OptionO
				tbl.ConfigureInterface(cfg)
				sender := &fakeSender{}
				tbl.SetSender(sender)
				db := lsdb.New(func() time.Time { return time.Unix(1, 0) })
				db.SetSelfRouterID(cfg.RouterID)
				db.SetAreaTypes(map[types.AreaID]string{cfg.AreaID: types.AreaTypeNormal})
				router := packet.RouterLSA{}
				routerKey := rfc5250Install(t, db, cfg.AreaID, packet.LSA{
					Header: rfc5250Header(types.LSTypeRouter, types.LinkStateID(cfg.RouterID), cfg.RouterID), Router: &router,
				})
				opaque := []types.LSAKey{
					rfc5250Opaque(t, db, types.LSTypeOpaqueLink, cfg.AreaID, cfg.Name, cfg.RouterID, 9),
					rfc5250Opaque(t, db, types.LSTypeOpaqueArea, cfg.AreaID, "", cfg.RouterID, 10),
					rfc5250Opaque(t, db, types.LSTypeOpaqueAS, cfg.AreaID, "", cfg.RouterID, 11),
				}
				tbl.SetLSDB(db)
				if reason := tbl.Hello(hello(cfg, peer, true, time.Unix(1, 0))); reason != "" {
					t.Fatal(reason)
				}
				initial := lastDD(t, sender)
				if initial.Options != cfg.Options || len(initial.Headers) != 0 {
					t.Fatalf("initial DD options=%#x headers=%v, want local O-bit and no headers", initial.Options, initial.Headers)
				}
				incoming := packet.DBDesc{InterfaceMTU: 1500, Options: types.OptionE,
					Flags: packet.DDFlagInit | packet.DDFlagMore | packet.DDFlagMaster, DDSequence: 0x14f94b26}
				if peerOpaque {
					incoming.Options |= types.OptionO
				}
				if localMaster {
					incoming.Flags = 0
					incoming.DDSequence = initial.DDSequence
				}
				if reason := tbl.HandleDBDesc(cfg.Name, peer, incoming); reason != "" {
					t.Fatalf("negotiation refused: %s", reason)
				}
				advertised := lastDD(t, sender)
				if advertised.Options != cfg.Options {
					t.Fatalf("negotiation changed our advertised capability: got=%#x want=%#x", advertised.Options, cfg.Options)
				}
				want := map[types.LSAKey]bool{routerKey: true}
				if peerOpaque {
					for _, key := range opaque {
						want[key] = true
					}
				}
				if len(advertised.Headers) != len(want) {
					t.Fatalf("DD advertised keys %v, want exactly %v", summaryKeys(advertised.Headers), want)
				}
				for _, header := range advertised.Headers {
					if !want[header.Key()] {
						t.Fatalf("DD advertised inapplicable LSA %v", header.Key())
					}
					delete(want, header.Key())
				}
				if len(want) != 0 {
					t.Fatalf("DD omitted applicable headers %v", want)
				}
				// Finish the unchanged RFC 2328 master/slave sequence. With no
				// received LSA headers, the request list is empty and Full follows.
				incoming.Flags = packet.DDFlagMaster
				incoming.DDSequence++
				if localMaster {
					incoming.Flags = 0
					incoming.DDSequence = advertised.DDSequence
				}
				if reason := tbl.HandleDBDesc(cfg.Name, peer, incoming); reason != "" {
					t.Fatalf("final DD refused: %s", reason)
				}
				snapshot, ok := tbl.Lookup(cfg.Name, peer)
				if !ok || snapshot.State != stateNameFull || snapshot.OpaqueCapable != peerOpaque {
					t.Fatalf("negotiated adjacency did not reach Full with correct peer capability: %+v", snapshot)
				}
			})
		}
	}
}

// The shared NSM receives full-width OSPFv3 LS types, with Grace mapped to the
// existing distinct sentinel by codec_v6.go::v6NeutralLSType. This is a neutral
// summary-selection control, not a second OSPFv3 wire-encoding claim.
func TestDDOpaqueCapabilityPreservesV3NeutralSummary(t *testing.T) {
	tbl, cfg := testTable(t, types.NetworkPointToPoint)
	peer := rid(t, "10.0.0.2")
	db := fakeLSDB{}
	want := make(map[types.LSAKey]bool)
	for _, kind := range []types.LSType{0x2001, 0x2009, types.LSTypeGraceV6} {
		header := packet.LSAHeader{Type: kind, AdvertisingRouter: cfg.RouterID,
			LinkStateID: types.LinkStateID{0, 0, 0, 1}, Sequence: types.InitialSequenceNumber}
		db[header.Key()] = packet.LSA{Header: header}
		want[header.Key()] = true
	}
	tbl.SetLSDB(db)
	if reason := tbl.Hello(hello(cfg, peer, true, time.Unix(1, 0))); reason != "" {
		t.Fatal(reason)
	}
	if reason := tbl.HandleDBDesc(cfg.Name, peer, masterDD(99, packet.DDFlagInit|packet.DDFlagMore|packet.DDFlagMaster)); reason != "" {
		t.Fatalf("v3 neutral negotiation refused: %s", reason)
	}
	n, ok := tbl.lookupLocked(cfg.Name, peer)
	if !ok {
		t.Fatal("negotiated neighbor missing")
	}
	if len(n.SummaryList) != len(want) {
		t.Fatalf("v3 neutral summary lost native types: got=%v want=%v", summaryKeys(n.SummaryList), want)
	}
	for _, header := range n.SummaryList {
		if !want[header.Key()] {
			t.Fatalf("unexpected or duplicate v3 summary header %v", header.Key())
		}
		delete(want, header.Key())
	}
	if len(want) != 0 {
		t.Fatalf("v3 summary omitted native headers %v", want)
	}
}
