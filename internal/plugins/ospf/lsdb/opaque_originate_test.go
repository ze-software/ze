// VALIDATES: spec-ospf-ext-1 AC-8/AC-9/A-3/R-7 -- OriginateOpaque installs and floods a
// self-originated opaque LSA at each scope (Type 9 link / 10 area / 11 AS) by reusing the
// self-origination machinery, is idempotent for an unchanged body, and MaxAge-flushes on
// withdraw through the existing purge path.
// PREVENTS: a new bespoke opaque origination path that diverges from RFC 2328 sequencing,
// floods on every identical re-run, or leaves a withdrawn opaque LSA lingering in peers.
package lsdb

import (
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/packet"
	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/types"
)

func opaqueOriginateDB(t *testing.T) (*LSDB, *txRecorder, *fakeClock) {
	t.Helper()
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	tx := &txRecorder{}
	db.SetTx(tx.Send)
	db.SetTopology(opaqueTopology)
	return db, tx, clock
}

func lsUpdateSends(tx *txRecorder) int {
	n := 0
	for i := range tx.sends {
		if tx.sends[i].pkt.LSUpdate != nil {
			n++
		}
	}
	return n
}

func TestOpaqueOriginateArea(t *testing.T) {
	db, tx, _ := opaqueOriginateDB(t)
	a0 := area("0.0.0.0")
	h, ok := db.OriginateOpaque(OpaqueOriginateInput{
		Router: rid("1.1.1.1"), OpaqueType: 1, OpaqueID: 0x01, Scope: types.LSTypeOpaqueArea,
		Area: a0, Options: types.OptionO, Body: []byte{0xaa, 0xbb, 0xcc, 0xdd},
	})
	if !ok {
		t.Fatalf("OriginateOpaque(area) returned not-originated")
	}
	if h.Type != types.LSTypeOpaqueArea || h.LinkStateID != packet.OpaqueLinkStateID(1, 0x01) {
		t.Fatalf("bad originated header: %+v", h)
	}
	if _, ok := db.LookupLSA(a0, h.Key()); !ok {
		t.Fatalf("area opaque LSA not installed in the area store")
	}
	if lsUpdateSends(tx) == 0 {
		t.Fatalf("area opaque LSA was not flooded")
	}
}

func TestOpaqueOriginateAS(t *testing.T) {
	db, tx, _ := opaqueOriginateDB(t)
	h, ok := db.OriginateOpaque(OpaqueOriginateInput{
		Router: rid("1.1.1.1"), OpaqueType: 4, OpaqueID: 0x02, Scope: types.LSTypeOpaqueAS,
		Options: types.OptionO, Body: []byte{0x01, 0x02, 0x03, 0x04},
	})
	if !ok {
		t.Fatalf("OriginateOpaque(AS) returned not-originated")
	}
	if _, ok := db.LookupLSA(area("0.0.0.5"), h.Key()); !ok {
		t.Fatalf("AS opaque LSA not visible AS-wide")
	}
	if s := db.Snapshot(); len(s.ASOpaque) != 1 {
		t.Fatalf("AS opaque LSA not in the AS-opaque store: %+v", s.ASOpaque)
	}
	if lsUpdateSends(tx) == 0 {
		t.Fatalf("AS opaque LSA was not flooded")
	}
}

func TestOpaqueOriginateLink(t *testing.T) {
	db, tx, _ := opaqueOriginateDB(t)
	a0 := area("0.0.0.0")
	h, ok := db.OriginateOpaque(OpaqueOriginateInput{
		Router: rid("1.1.1.1"), OpaqueType: 1, OpaqueID: 0x03, Scope: types.LSTypeOpaqueLink,
		Interface: "eth0", Area: a0, Options: types.OptionO, Body: []byte{0x09, 0x08, 0x07, 0x06},
	})
	if !ok {
		t.Fatalf("OriginateOpaque(link) returned not-originated")
	}
	if _, ok := db.LookupLinkLSA("eth0", h.Key()); !ok {
		t.Fatalf("link opaque LSA not installed in the eth0 link store")
	}
	if lsUpdateSends(tx) == 0 {
		t.Fatalf("link opaque LSA was not flooded")
	}
}

func TestOpaqueOriginateIdempotent(t *testing.T) {
	db, tx, _ := opaqueOriginateDB(t)
	a0 := area("0.0.0.0")
	in := OpaqueOriginateInput{
		Router: rid("1.1.1.1"), OpaqueType: 1, OpaqueID: 0x04, Scope: types.LSTypeOpaqueArea,
		Area: a0, Options: types.OptionO, Body: []byte{0x11, 0x22, 0x33, 0x44},
	}
	if _, ok := db.OriginateOpaque(in); !ok {
		t.Fatalf("first origination failed")
	}
	first := lsUpdateSends(tx)
	// An identical re-origination changes nothing and floods nothing (AC-8).
	if _, ok := db.OriginateOpaque(in); ok {
		t.Fatalf("identical re-origination reported a change")
	}
	if lsUpdateSends(tx) != first {
		t.Fatalf("identical re-origination flooded again: %d -> %d", first, lsUpdateSends(tx))
	}
}

func TestOpaqueWithdrawFlushes(t *testing.T) {
	db, tx, clock := opaqueOriginateDB(t)
	a0 := area("0.0.0.0")
	in := OpaqueOriginateInput{
		Router: rid("1.1.1.1"), OpaqueType: 1, OpaqueID: 0x05, Scope: types.LSTypeOpaqueArea,
		Area: a0, Options: types.OptionO, Body: []byte{0x55, 0x66, 0x77, 0x88},
	}
	if _, ok := db.OriginateOpaque(in); !ok {
		t.Fatalf("origination failed")
	}
	tx.sends = nil
	clock.Add(2 * time.Second) // clear MinLSArrival so the flush installs
	in.Withdraw = true
	if _, ok := db.OriginateOpaque(in); !ok {
		t.Fatalf("withdraw reported no flush")
	}
	// The flushed instance must be at MaxAge (a purge) and flooded.
	sawMaxAge := false
	for _, s := range tx.sends {
		if s.pkt.LSUpdate == nil {
			continue
		}
		for _, l := range s.pkt.LSUpdate.LSAs {
			if l.Header.Type == types.LSTypeOpaqueArea && l.Header.Age.IsMaxAge() {
				sawMaxAge = true
			}
		}
	}
	if !sawMaxAge {
		t.Fatalf("withdraw did not flood a MaxAge purge: %+v", tx.sends)
	}
}
