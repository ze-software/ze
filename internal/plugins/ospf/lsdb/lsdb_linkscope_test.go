package lsdb

import (
	"net/netip"
	"testing"
	"time"

	ospfpacket "github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3packet "github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
	ospfv3types "github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

type rawSend struct {
	iface string
	dst   netip.Addr
	raw   []byte
}

type rawTxRecorder struct{ sends []rawSend }

func (r *rawTxRecorder) Send(iface string, dst netip.Addr, payload []byte) error {
	raw := make([]byte, len(payload))
	copy(raw, payload)
	r.sends = append(r.sends, rawSend{iface: iface, dst: dst, raw: raw})
	return nil
}

func linkLSAForTest(t *testing.T, adv types.RouterID, ifid uint32, ll netip.Addr, seq types.LSSequenceNumber) ospfpacket.LSA {
	t.Helper()
	body := ospfv3packet.LinkLSA{RtrPriority: 1, Options: ospfv3types.OptV6 | ospfv3types.OptR, LinkLocalAddr: ll.As16()}
	lsa := ospfv3packet.LSA{
		Header: ospfv3packet.LSAHeader{
			Type:              ospfv3types.LSTypeLink,
			LinkStateID:       ospfv3types.LinkStateID(v3LSID(ifid)),
			AdvertisingRouter: ospfv3types.RouterID(adv),
			Sequence:          ospfv3types.LSSequenceNumber(int32(uint32(seq))),
		},
		Link: &body,
	}
	raw := make([]byte, lsa.EncodedLen())
	lsa.WriteTo(raw, 0)
	decoded, err := ospfv3packet.DecodeLSA(raw)
	if err != nil {
		t.Fatalf("DecodeLSA(link): %v", err)
	}
	return ospfpacket.LSA{
		Header: ospfpacket.LSAHeader{
			Age:               types.LSAge(decoded.Header.Age),
			Type:              types.LSType(decoded.Header.Type),
			LinkStateID:       types.LinkStateID(decoded.Header.LinkStateID),
			AdvertisingRouter: types.RouterID(decoded.Header.AdvertisingRouter),
			Sequence:          types.LSSequenceNumber(uint32(decoded.Header.Sequence)),
			Checksum:          decoded.Header.Checksum,
			Length:            decoded.Header.Length,
		},
		Body:     decoded.Body,
		RawBytes: raw,
	}
}

func v3LSID(id uint32) types.LinkStateID {
	return types.LinkStateID{byte(id >> 24), byte(id >> 16), byte(id >> 8), byte(id)}
}

func receiveLinkLSA(t *testing.T, db *LSDB, iface string, areaID types.AreaID, lsa ospfpacket.LSA) {
	t.Helper()
	db.SetTopology(func() []InterfaceInfo {
		return []InterfaceInfo{{
			Name:        iface,
			AreaID:      areaID,
			AreaType:    AreaTypeNormal,
			NetworkType: NetworkBroadcast,
			State:       InterfaceStateBackup,
			RouterID:    rid("1.1.1.1"),
			DR:          lsa.Header.AdvertisingRouter,
			BDR:         rid("1.1.1.1"),
			IsV6:        true,
		}}
	})
	reason := db.ReceiveUpdate(ReceiveInput{
		Interface: iface,
		AreaID:    areaID,
		RouterID:  lsa.Header.AdvertisingRouter,
		Src:       netip.MustParseAddr("fe80::2"),
		Update:    ospfpacket.LSUpdate{LSAs: []ospfpacket.LSA{lsa}},
	})
	if reason != "" {
		t.Fatalf("ReceiveUpdate reason = %q", reason)
	}
}

func TestOSPFv6LinkScopeStore(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	areaID := area("0.0.0.0")
	lsa := linkLSAForTest(t, rid("2.2.2.2"), 10, netip.MustParseAddr("fe80::2"), types.InitialSequenceNumber)

	receiveLinkLSA(t, db, "eth0", areaID, lsa)
	if _, ok := db.LookupLink("eth0", lsa.Header.Key()); !ok {
		t.Fatal("Link-LSA not stored under receiving interface")
	}
	if _, ok := db.LookupLink("eth1", lsa.Header.Key()); ok {
		t.Fatal("Link-LSA leaked into a different interface store")
	}
	if _, ok := db.Lookup(areaID, lsa.Header.Key()); ok {
		t.Fatal("Link-LSA leaked into the area-scope store")
	}
	snap := db.Snapshot()
	if len(snap.Links) != 1 || snap.Links[0].Interface != "eth0" || len(snap.Links[0].LSAs) != 1 {
		t.Fatalf("link snapshot = %+v", snap.Links)
	}
	row := snap.Links[0].LSAs[0]
	if row.Type != "link" || row.Interface != "eth0" || row.LinkLocalAddress != "fe80::2" {
		t.Fatalf("link snapshot row = %+v", row)
	}

	if n := db.ReleaseLink("eth0"); n != 1 {
		t.Fatalf("ReleaseLink removed %d LSAs, want 1", n)
	}
	if _, ok := db.LookupLink("eth0", lsa.Header.Key()); ok {
		t.Fatal("ReleaseLink left Link-LSA behind")
	}
}

func TestOSPFv6LinkScopeAgesAndFlushes(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	lsa := linkLSAForTest(t, rid("2.2.2.2"), 10, netip.MustParseAddr("fe80::2"), types.InitialSequenceNumber)
	receiveLinkLSA(t, db, "eth0", area("0.0.0.0"), lsa)
	clock.Add(time.Duration(types.MaxAge+1) * time.Second)
	if tick := db.Tick(clock.Now()); tick.Purged != 1 {
		t.Fatalf("Tick purged %d LSAs, want 1", tick.Purged)
	}
	if _, ok := db.LookupLink("eth0", lsa.Header.Key()); ok {
		t.Fatal("MaxAge Link-LSA was not flushed from the link store")
	}
}

func TestOSPFv6ReceiveLinkLSALinkScoped(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	tx := &rawTxRecorder{}
	db.SetTx(tx.Send)
	db.SetTopology(func() []InterfaceInfo {
		return []InterfaceInfo{
			{Name: "eth0", AreaID: area("0.0.0.0"), AreaType: AreaTypeNormal, NetworkType: NetworkBroadcast, State: InterfaceStateBackup, RouterID: rid("1.1.1.1"), DR: rid("2.2.2.2"), BDR: rid("1.1.1.1"), IsV6: true, Neighbors: []NeighborInfo{{RouterID: rid("2.2.2.2"), Address: netip.MustParseAddr("fe80::2"), State: NeighborStateFull}}},
			{Name: "eth1", AreaID: area("0.0.0.0"), AreaType: AreaTypeNormal, NetworkType: NetworkBroadcast, State: InterfaceStateDR, RouterID: rid("1.1.1.1"), DR: rid("1.1.1.1"), IsV6: true, Neighbors: []NeighborInfo{{RouterID: rid("3.3.3.3"), Address: netip.MustParseAddr("fe80::3"), State: NeighborStateFull}}},
		}
	})
	lsa := linkLSAForTest(t, rid("2.2.2.2"), 10, netip.MustParseAddr("fe80::2"), types.InitialSequenceNumber)

	reason := db.ReceiveUpdate(ReceiveInput{Interface: "eth0", AreaID: area("0.0.0.0"), RouterID: rid("2.2.2.2"), Src: netip.MustParseAddr("fe80::2"), Update: ospfpacket.LSUpdate{LSAs: []ospfpacket.LSA{lsa}}})
	if reason != "" {
		t.Fatalf("ReceiveUpdate reason = %q", reason)
	}
	if _, ok := db.LookupLink("eth0", lsa.Header.Key()); !ok {
		t.Fatal("received Link-LSA not installed under arrival interface")
	}
	if _, ok := db.LookupLink("eth1", lsa.Header.Key()); ok {
		t.Fatal("received Link-LSA propagated to another link store")
	}
	if len(tx.sends) != 0 {
		t.Fatalf("received Link-LSA was re-flooded: %+v", tx.sends)
	}
	if n := db.FlushDelayedAcks("eth0"); n != 1 {
		t.Fatalf("delayed ack flush = %d, want 1", n)
	}
	if len(tx.sends) != 1 || tx.sends[0].iface != "eth0" {
		t.Fatalf("ack sends = %+v", tx.sends)
	}
}

func TestOSPFv6OriginateLinkLSAFloodsLoadingNeighbor(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	tx := &rawTxRecorder{}
	db.SetTx(tx.Send)
	areaID := area("0.0.0.0")
	router := rid("1.1.1.1")
	peer := rid("2.2.2.2")
	db.SetTopology(func() []InterfaceInfo {
		return []InterfaceInfo{{
			Name:        "eth0",
			AreaID:      areaID,
			AreaType:    AreaTypeNormal,
			NetworkType: NetworkPointToPoint,
			State:       "point-to-point",
			RouterID:    router,
			IsV6:        true,
			Neighbors: []NeighborInfo{{
				RouterID: peer,
				Address:  netip.MustParseAddr("fe80::2"),
				State:    NeighborStateLoading,
			}},
		}}
	})
	lsa := linkLSAForTest(t, router, 2, netip.MustParseAddr("fe80::1"), types.InitialSequenceNumber)
	body := append([]byte(nil), lsa.RawBytes[types.LSAHeaderLen:]...)

	if _, ok := db.OriginateLinkSelf("eth0", areaID, lsa.Header.Key(), body, func(seq types.LSSequenceNumber, purge bool) ospfpacket.LSA {
		return linkLSAForTest(t, router, 2, netip.MustParseAddr("fe80::1"), seq)
	}); !ok {
		t.Fatal("OriginateLinkSelf rejected Link-LSA")
	}
	if len(tx.sends) != 1 || tx.sends[0].iface != "eth0" {
		t.Fatalf("originated Link-LSA sends = %+v, want one flood to loading neighbor", tx.sends)
	}
}

func TestOSPFv6LinkLSAMaxSequencePurges(t *testing.T) {
	// At MaxSequenceNumber the link path MaxAge-flushes the Link-LSA (purge) instead of
	// re-originating at the max, mirroring the area path (RFC 2328 sec 12.1.6), so the LSA can
	// later restart from InitialSequenceNumber. Regression: the link path returned no purge flag.
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	areaID := area("0.0.0.0")
	router := rid("2.2.2.2")
	ifaceID := uint32(10)
	ll := netip.MustParseAddr("fe80::2")
	seed := linkLSAForTest(t, router, ifaceID, ll, types.MaxSequenceNumber)
	body := append([]byte(nil), seed.RawBytes[types.LSAHeaderLen:]...)
	key := seed.Header.Key()

	// Drive the link ownRecord to MaxSequenceNumber directly (reaching it organically needs 2^31
	// re-originations); last stays zero so the MinLSInterval rate-limit does not apply.
	db.linkOwn["eth0"] = map[types.LSAKey]ownRecord{key: {sequence: types.MaxSequenceNumber}}

	gotPurge := false
	gotSeq := types.LSSequenceNumber(0)
	if _, ok := db.OriginateLinkSelf("eth0", areaID, key, body, func(seq types.LSSequenceNumber, purge bool) ospfpacket.LSA {
		gotPurge, gotSeq = purge, seq
		lsa := linkLSAForTest(t, router, ifaceID, ll, seq)
		if purge {
			lsa.Header.Age = types.LSAge(types.MaxAge)
			lsa.Header.Age.WriteTo(lsa.RawBytes, 0)
		}
		return lsa
	}); !ok {
		t.Fatal("OriginateLinkSelf rejected the Link-LSA at MaxSequenceNumber")
	}
	if !gotPurge {
		t.Fatal("OriginateLinkSelf at MaxSequenceNumber did not request a purge (flush-then-restart)")
	}
	if gotSeq != types.MaxSequenceNumber {
		t.Fatalf("purge sequence = %v, want MaxSequenceNumber", gotSeq)
	}
}

func TestOSPFAreaInstallRejectsLinkLSA(t *testing.T) {
	// Link-scoped LSAs (Type 0x0008) must go through installLink, not the area Install path; a
	// misrouted Install of a Link-LSA is rejected so it cannot land in an area DB.
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	link := linkLSAForTest(t, rid("2.2.2.2"), 10, netip.MustParseAddr("fe80::2"), types.InitialSequenceNumber)
	if db.Install(area("0.0.0.0"), link) {
		t.Fatal("area Install accepted a link-scoped LSA (must use installLink)")
	}
}

func TestOSPFv6SelfLinkLSAFightBack(t *testing.T) {
	// RFC 2328 sec 13.4 fight-back: a self-originated Link-LSA received newer than our record (a
	// stale instance after a restart, or a router using our Router ID) must advance our sequence
	// so the next origination re-originates a strictly newer instance that reclaims the LSA.
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock) // sets the self Router ID to 1.1.1.1
	router := rid("1.1.1.1")
	ifaceID := uint32(10)
	ll := netip.MustParseAddr("fe80::2")
	highSeq := types.InitialSequenceNumber.Next().Next()
	received := linkLSAForTest(t, router, ifaceID, ll, highSeq)
	key := received.Header.Key()

	if !db.handleSelfLinkReceived("eth0", received) {
		t.Fatal("handleSelfLinkReceived did not recognize the self-originated Link-LSA")
	}
	if got := db.linkOwn["eth0"][key].sequence; got != highSeq {
		t.Fatalf("linkOwn sequence = %v, want %v (bumped to the received instance)", got, highSeq)
	}
	// A subsequent origination must produce a strictly newer instance that supersedes the
	// received one across the link.
	body := append([]byte(nil), received.RawBytes[types.LSAHeaderLen:]...)
	h, ok := db.OriginateLinkSelf("eth0", area("0.0.0.0"), key, body, func(seq types.LSSequenceNumber, _ bool) ospfpacket.LSA {
		return linkLSAForTest(t, router, ifaceID, ll, seq)
	})
	if !ok {
		t.Fatal("OriginateLinkSelf did not re-originate after the fight-back bump")
	}
	if !h.Sequence.NewerThan(highSeq) {
		t.Fatalf("re-originated sequence %v not newer than the received %v (fight-back failed)", h.Sequence, highSeq)
	}
}

func TestOSPFv6RefreshesSelfLinkLSA(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	areaID := area("0.0.0.0")
	router := rid("2.2.2.2")
	ifaceID := uint32(10)
	ll := netip.MustParseAddr("fe80::2")
	seed := linkLSAForTest(t, router, ifaceID, ll, types.InitialSequenceNumber)
	body := append([]byte(nil), seed.RawBytes[types.LSAHeaderLen:]...)
	key := seed.Header.Key()

	if _, ok := db.OriginateLinkSelf("eth0", areaID, key, body, func(seq types.LSSequenceNumber, purge bool) ospfpacket.LSA {
		lsa := linkLSAForTest(t, router, ifaceID, ll, seq)
		if purge {
			lsa.Header.Age = types.LSAge(types.MaxAge)
			lsa.Header.Age.WriteTo(lsa.RawBytes, 0)
		}
		return lsa
	}); !ok {
		t.Fatal("OriginateLinkSelf rejected Link-LSA")
	}

	clock.Add(time.Duration(types.LSRefreshTime) * time.Second)
	if n := db.RefreshSelf(clock.Now()); n != 1 {
		t.Fatalf("RefreshSelf refreshed %d LSAs, want 1", n)
	}
	refreshed, ok := db.LookupLinkLSA("eth0", key)
	if !ok {
		t.Fatal("refreshed Link-LSA missing")
	}
	if refreshed.Header.Sequence != types.InitialSequenceNumber.Next() {
		t.Fatalf("sequence = %v, want %v", refreshed.Header.Sequence, types.InitialSequenceNumber.Next())
	}
	if !ospfv3packet.VerifyLSAChecksum(refreshed.RawBytes) {
		t.Fatal("refreshed OSPFv3 Link-LSA checksum invalid")
	}
	decoded, err := ospfv3packet.DecodeLSA(refreshed.RawBytes)
	if err != nil {
		t.Fatalf("DecodeLSA(refreshed): %v", err)
	}
	if decoded.Header.Type != ospfv3types.LSTypeLink {
		t.Fatalf("refreshed LS type = %#x, want Link-LSA", uint16(decoded.Header.Type))
	}
	if _, err := decoded.DecodeLink(); err != nil {
		t.Fatalf("DecodeLink(refreshed): %v", err)
	}
}
