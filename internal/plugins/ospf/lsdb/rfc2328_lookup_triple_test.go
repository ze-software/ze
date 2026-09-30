package lsdb

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// VALIDATES: RFC 2328 Section 12.2, the area database is looked up by the whole triple
// (LS type, Link State ID, Advertising Router).
// PREVENTS: a lookup that ignores the LS type or the Link State ID answering with another LSA.

// lookupTripleDB installs three LSAs from 4.4.4.4 that pairwise share two of the three
// identity elements, each with its own sequence number: a Router-LSA with Link State ID
// 4.4.4.4, a Summary-LSA with the same Link State ID (differs from it only in LS type), and a
// Summary-LSA with Link State ID 9.9.9.9 (differs from the first Summary-LSA only in Link
// State ID).
func lookupTripleDB(t *testing.T) *LSDB {
	t.Helper()
	db := newTestDB(&fakeClock{now: time.Unix(0, 0)})
	adv := rid("4.4.4.4")
	router := routerLSA(t, adv, types.InitialSequenceNumber, 10)
	summaryFor := func(id types.LinkStateID, seq types.LSSequenceNumber) packet.LSA {
		return encodeDecodeLSA(t, packet.LSA{Header: packet.LSAHeader{Options: types.OptionE, Type: types.LSTypeSummaryNetwork,
			LinkStateID: id, AdvertisingRouter: adv, Sequence: seq},
			Summary: &packet.SummaryLSA{NetworkMask: ip4("255.255.255.255"), Metric: 20}})
	}
	summary := summaryFor(types.LinkStateID(adv), types.InitialSequenceNumber+5)
	other := summaryFor(lsid("9.9.9.9"), types.InitialSequenceNumber+9)
	for _, lsa := range []packet.LSA{router, summary, other} {
		if !db.Install(area("0.0.0.0"), lsa) {
			t.Fatalf("Install refused %v", lsa.Header.Key())
		}
	}
	return db
}

// TestRFC2328LookupTellsApartLSAsSharingTwoElements proves each of three LSAs sharing two
// identity elements is found by its own triple. RFC 2328 Section 12.2: "This lookup function
// is based on an LSA's LS type, Link State ID and Advertising Router."
//
// Goal: the lookup uses the LS type and the Link State ID, not only the Advertising Router.
// Method: lookupTripleDB, then each triple must return its own instance's sequence number.
func TestRFC2328LookupTellsApartLSAsSharingTwoElements(t *testing.T) {
	// RFC requirement: RFC2328-12.2-1 positive -- three LSAs from one router that differ only
	// in LS type or only in Link State ID are each found by their own triple (Lookup, lsdb.go).
	db := lookupTripleDB(t)
	adv := rid("4.4.4.4")
	for _, c := range []struct {
		key types.LSAKey
		seq types.LSSequenceNumber
	}{
		{types.LSAKey{Type: types.LSTypeRouter, LinkStateID: types.LinkStateID(adv), AdvertisingRouter: adv}, types.InitialSequenceNumber},
		{types.LSAKey{Type: types.LSTypeSummaryNetwork, LinkStateID: types.LinkStateID(adv), AdvertisingRouter: adv}, types.InitialSequenceNumber + 5},
		{types.LSAKey{Type: types.LSTypeSummaryNetwork, LinkStateID: lsid("9.9.9.9"), AdvertisingRouter: adv}, types.InitialSequenceNumber + 9},
	} {
		h, ok := db.Lookup(area("0.0.0.0"), c.key)
		if !ok || h.Sequence != c.seq {
			t.Fatalf("Lookup %v = seq %#x ok=%v, want the instance at seq %#x", c.key, uint32(h.Sequence), ok, uint32(c.seq))
		}
	}
}

// TestRFC2328LookupMissesWhenOneElementDiffers proves a triple matching an installed LSA in
// two elements only finds nothing, for each element in turn.
//
// Goal: no partial match answers for a full one, whichever element differs.
// Method: lookupTripleDB, then triples that differ from an installed LSA in the LS type
// alone, the Link State ID alone, or the Advertising Router alone.
func TestRFC2328LookupMissesWhenOneElementDiffers(t *testing.T) {
	// RFC requirement: RFC2328-12.2-1 negative -- a triple that differs from every installed
	// LSA in exactly one element (LS type, Link State ID, or Advertising Router) finds nothing
	// (Lookup, lsdb.go).
	db := lookupTripleDB(t)
	adv := rid("4.4.4.4")
	for name, key := range map[string]types.LSAKey{
		"LS type":            {Type: types.LSTypeSummaryASBR, LinkStateID: types.LinkStateID(adv), AdvertisingRouter: adv},
		"Link State ID":      {Type: types.LSTypeSummaryNetwork, LinkStateID: lsid("8.8.8.8"), AdvertisingRouter: adv},
		"Advertising Router": {Type: types.LSTypeRouter, LinkStateID: types.LinkStateID(adv), AdvertisingRouter: rid("5.5.5.5")},
	} {
		if h, ok := db.Lookup(area("0.0.0.0"), key); ok {
			t.Fatalf("a triple differing in its %s answered %+v", name, h)
		}
	}
}
