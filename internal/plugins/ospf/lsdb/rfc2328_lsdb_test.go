package lsdb

import (
	"bytes"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time      { return c.now }
func (c *fakeClock) Add(d time.Duration) { c.now = c.now.Add(d) }

type sentPacket struct {
	iface string
	dst   netip.Addr
	pkt   packet.Packet
	raw   []byte
}

type txRecorder struct{ sends []sentPacket }

func (r *txRecorder) Send(iface string, dst netip.Addr, payload []byte) error {
	p, err := packet.DecodePacket(payload)
	if err != nil {
		return err
	}
	raw := make([]byte, len(payload))
	copy(raw, payload)
	r.sends = append(r.sends, sentPacket{iface: iface, dst: dst, pkt: p, raw: raw})
	return nil
}

func rid(s string) types.RouterID {
	id, err := types.ParseRouterID(s)
	if err != nil {
		panic(err)
	}
	return id
}

func area(s string) types.AreaID {
	id, err := types.ParseAreaID(s)
	if err != nil {
		panic(err)
	}
	return id
}

func lsid(s string) types.LinkStateID {
	id, err := types.ParseLinkStateID(s)
	if err != nil {
		panic(err)
	}
	return id
}

func ip4(s string) [4]byte { return netip.MustParseAddr(s).As4() }

// naddr4 is the netip.Addr form for NeighborInfo.Address (a reachable address).
func naddr4(s string) netip.Addr { return netip.MustParseAddr(s) }

func newTestDB(c *fakeClock) *LSDB {
	db := New(c.Now)
	db.SetSelfRouterID(rid("1.1.1.1"))
	db.SetTimers(TimerConfig{MinLSArrival: time.Second, MinLSInterval: 5 * time.Second})
	return db
}

func routerLSA(t *testing.T, adv types.RouterID, seq types.LSSequenceNumber, metric types.Metric) packet.LSA {
	t.Helper()
	body := packet.RouterLSA{Links: []packet.RouterLink{{LinkID: lsid("10.0.0.0"), LinkData: ip4("255.255.255.0"), Type: packet.RouterLinkTypeStub, Metric: metric}}}
	lsa := packet.LSA{Header: packet.LSAHeader{Age: 0, Options: types.OptionE, Type: types.LSTypeRouter, LinkStateID: types.LinkStateID(adv), AdvertisingRouter: adv, Sequence: seq}, Router: &body}
	return encodeDecodeLSA(t, lsa)
}

func externalLSA(t *testing.T, adv types.RouterID, seq types.LSSequenceNumber) packet.LSA {
	t.Helper()
	body := packet.ExternalLSA{NetworkMask: ip4("255.255.255.0"), Metric: 20}
	lsa := packet.LSA{Header: packet.LSAHeader{Age: 0, Options: types.OptionE, Type: types.LSTypeASExternal, LinkStateID: lsid("203.0.113.0"), AdvertisingRouter: adv, Sequence: seq}, External: &body}
	return encodeDecodeLSA(t, lsa)
}

func encodeDecodeLSA(t *testing.T, lsa packet.LSA) packet.LSA {
	t.Helper()
	buf := make([]byte, lsa.EncodedLen())
	lsa.WriteTo(buf, 0)
	decoded, err := packet.DecodeLSA(buf)
	if err != nil {
		t.Fatalf("DecodeLSA: %v", err)
	}
	if !decoded.VerifyChecksum() {
		t.Fatalf("checksum invalid for %v", decoded.Header.Key())
	}
	return decoded
}

func TestOSPFLSDBStoreRetrieve(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	a0 := area("0.0.0.0")
	a1 := area("0.0.0.1")
	r := routerLSA(t, rid("2.2.2.2"), types.InitialSequenceNumber, 10)
	if !db.Install(a0, r) {
		t.Fatalf("Install router LSA rejected")
	}
	if _, ok := db.Lookup(a0, r.Header.Key()); !ok {
		t.Fatalf("router LSA missing from area")
	}
	if _, ok := db.Lookup(a1, r.Header.Key()); ok {
		t.Fatalf("router LSA leaked across areas")
	}
	ext := externalLSA(t, rid("3.3.3.3"), types.InitialSequenceNumber)
	if !db.Install(a0, ext) {
		t.Fatalf("Install external LSA rejected")
	}
	if _, ok := db.Lookup(a1, ext.Header.Key()); !ok {
		t.Fatalf("type 5 LSA not visible through AS-wide store")
	}
}

// RFC requirement: RFC2328-13.1-1 positive -- the more recent of two instances is determined in order: higher LS sequence number, then larger LS checksum, then the MaxAge instance, then the younger age when the ages differ by more than MaxAgeDiff, else identical (CompareHeaders, entry.go:115-143).
func TestOSPFFreshnessCompareMatrix(t *testing.T) {
	base := packet.LSAHeader{Age: 10, Type: types.LSTypeRouter, LinkStateID: lsid("1.1.1.1"), AdvertisingRouter: rid("1.1.1.1"), Sequence: types.InitialSequenceNumber, Checksum: 10, Length: types.LSAHeaderLen}
	newerSeq := base
	newerSeq.Sequence = base.Sequence.Next()
	if CompareHeaders(newerSeq, base) != Newer || CompareHeaders(base, newerSeq) != Older {
		t.Fatalf("sequence freshness wrong")
	}
	newerChecksum := base
	newerChecksum.Checksum = 11
	if CompareHeaders(newerChecksum, base) != Newer {
		t.Fatalf("checksum freshness wrong")
	}
	maxAge := base
	maxAge.Age = types.LSAge(types.MaxAge)
	if CompareHeaders(maxAge, base) != Newer {
		t.Fatalf("MaxAge freshness wrong")
	}
	young := base
	old := base
	young.Age = 1
	old.Age = types.LSAge(types.MaxAgeDiff + 20)
	if CompareHeaders(young, old) != Newer {
		t.Fatalf("age-diff freshness wrong")
	}
	nearOld := base
	nearOld.Age = types.LSAge(types.MaxAgeDiff)
	if CompareHeaders(base, nearOld) != Equal {
		t.Fatalf("MaxAgeDiff equality wrong")
	}
}

func TestOSPFLSDBStoreVerbatim(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	a0 := area("0.0.0.0")
	lsa := routerLSA(t, rid("2.2.2.2"), types.InitialSequenceNumber, 10)
	original := append([]byte(nil), lsa.RawBytes...)
	if !db.Install(a0, lsa) {
		t.Fatalf("install rejected")
	}
	for i := range lsa.RawBytes {
		lsa.RawBytes[i] = 0xee
	}
	got, ok := db.LookupLSA(a0, types.LSAKey{Type: types.LSTypeRouter, LinkStateID: types.LinkStateID(rid("2.2.2.2")), AdvertisingRouter: rid("2.2.2.2")})
	if !ok {
		t.Fatalf("lookup failed")
	}
	if !bytes.Equal(got.RawBytes, original) {
		t.Fatalf("stored raw aliased caller buffer")
	}
	clock.Add(2 * time.Second)
	got, _ = db.LookupLSA(a0, got.Header.Key())
	if got.RawBytes[0] != 0 || got.RawBytes[1] != 2 {
		t.Fatalf("age not updated in raw copy: % x", got.RawBytes[:2])
	}
	if !got.VerifyChecksum() {
		t.Fatalf("age update changed Fletcher checksum validity")
	}
}

func TestOSPFLSDBSnapshot(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	a0 := area("0.0.0.0")
	db.Install(a0, routerLSA(t, rid("2.2.2.2"), types.InitialSequenceNumber, 10))
	snap := db.Snapshot()
	if len(snap.Areas) != 1 || len(snap.Areas[0].LSAs) != 1 {
		t.Fatalf("snapshot = %+v", snap)
	}
	got := snap.Areas[0].LSAs[0]
	if got.Type != "router" || got.AdvertisingRouter != "2.2.2.2" || got.Length == 0 || got.Checksum == 0 {
		t.Fatalf("bad snapshot entry: %+v", got)
	}
}
