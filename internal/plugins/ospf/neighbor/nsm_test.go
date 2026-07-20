// Design: plan/learned/960-ospf-6-neighbor-nsm.md -- NSM transition and DD tests
package neighbor

import (
	"net/netip"
	"strings"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/packet"
	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/types"
)

func TestOSPFInitialDDSequenceFromLocalRouterID(t *testing.T) {
	// The initial DD sequence seeds from the LOCAL Router ID's four octets, big-endian
	// (RFC 2328 sec 10.6). Regression: the old code mixed the local ID's high bytes with the
	// neighbor's low bytes, producing a meaningless half-and-half value.
	if got := initialDDSequence(types.RouterID{1, 2, 3, 4}); got != 0x01020304 {
		t.Fatalf("initialDDSequence = %#08x, want 0x01020304 (all four local octets)", got)
	}
	if got := initialDDSequence(types.RouterID{0, 0, 0, 0}); got != 1 {
		t.Fatalf("zero Router ID -> %d, want 1 (non-zero floor)", got)
	}
}

type fakeLSDB map[types.LSAKey]packet.LSA

func (db fakeLSDB) Lookup(_ types.AreaID, k types.LSAKey) (packet.LSAHeader, bool) {
	lsa, ok := db[k]
	return lsa.Header, ok
}

func (db fakeLSDB) LookupLSA(_ types.AreaID, k types.LSAKey) (packet.LSA, bool) {
	lsa, ok := db[k]
	return lsa, ok
}

func (db fakeLSDB) Install(_ types.AreaID, lsa packet.LSA) bool {
	db[lsa.Header.Key()] = lsa
	return true
}

func (db fakeLSDB) Summary(_ types.AreaID) []packet.LSAHeader {
	out := make([]packet.LSAHeader, 0, len(db))
	for _, lsa := range db {
		out = append(out, lsa.Header)
	}
	return out
}

type fakeScopedLSDB struct {
	area  fakeLSDB
	links map[string]fakeLSDB
}

func (db *fakeScopedLSDB) Lookup(area types.AreaID, k types.LSAKey) (packet.LSAHeader, bool) {
	if db.area == nil {
		return packet.LSAHeader{}, false
	}
	return db.area.Lookup(area, k)
}

func (db *fakeScopedLSDB) LookupLSA(area types.AreaID, k types.LSAKey) (packet.LSA, bool) {
	if db.area == nil {
		return packet.LSA{}, false
	}
	return db.area.LookupLSA(area, k)
}

func (db *fakeScopedLSDB) Install(area types.AreaID, lsa packet.LSA) bool {
	if db.area == nil {
		db.area = fakeLSDB{}
	}
	return db.area.Install(area, lsa)
}

func (db *fakeScopedLSDB) Summary(area types.AreaID) []packet.LSAHeader {
	if db.area == nil {
		return nil
	}
	return db.area.Summary(area)
}

func (db *fakeScopedLSDB) LookupLink(iface string, k types.LSAKey) (packet.LSAHeader, bool) {
	if db.links == nil || db.links[iface] == nil {
		return packet.LSAHeader{}, false
	}
	lsa, ok := db.links[iface][k]
	return lsa.Header, ok
}

func (db *fakeScopedLSDB) LookupLinkLSA(iface string, k types.LSAKey) (packet.LSA, bool) {
	if db.links == nil || db.links[iface] == nil {
		return packet.LSA{}, false
	}
	lsa, ok := db.links[iface][k]
	return lsa, ok
}

func (db *fakeScopedLSDB) LinkLSAs(iface string) []packet.LSA {
	if db.links == nil || db.links[iface] == nil {
		return nil
	}
	out := make([]packet.LSA, 0, len(db.links[iface]))
	for _, lsa := range db.links[iface] {
		out = append(out, lsa)
	}
	return out
}

type fakeSender struct {
	dst  []netip.Addr
	sent [][]byte
}

func (s *fakeSender) SendPacket(_ string, dst netip.Addr, payload []byte) error {
	s.dst = append(s.dst, dst)
	cp := append([]byte(nil), payload...)
	s.sent = append(s.sent, cp)
	return nil
}

func sentPacket(t *testing.T, s *fakeSender, idx int) packet.Packet {
	t.Helper()
	if idx >= len(s.sent) {
		t.Fatalf("sent packets = %d, want index %d", len(s.sent), idx)
	}
	p, err := packet.DecodePacket(s.sent[idx])
	if err != nil {
		t.Fatal(err)
	}
	return p
}

type fakeGaugeVec struct {
	values map[string]float64
}

func (v *fakeGaugeVec) With(labelValues ...string) metrics.Gauge {
	if v.values == nil {
		v.values = make(map[string]float64)
	}
	return fakeGauge{v: v, key: strings.Join(labelValues, "\xff")}
}

func (v *fakeGaugeVec) Delete(labelValues ...string) bool {
	key := strings.Join(labelValues, "\xff")
	_, ok := v.values[key]
	delete(v.values, key)
	return ok
}

type fakeGauge struct {
	v   *fakeGaugeVec
	key string
}

func (g fakeGauge) Set(v float64) { g.v.values[g.key] = v }
func (g fakeGauge) Inc()          { g.v.values[g.key]++ }
func (g fakeGauge) Dec()          { g.v.values[g.key]-- }
func (g fakeGauge) Add(v float64) { g.v.values[g.key] += v }

type fakeCounterVec struct{}

func (fakeCounterVec) With(...string) metrics.Counter { return fakeCounter{} }
func (fakeCounterVec) Delete(...string) bool          { return false }

type fakeCounter struct{}

func (fakeCounter) Inc()        {}
func (fakeCounter) Add(float64) {}

func rid(t *testing.T, s string) types.RouterID {
	t.Helper()
	id, err := types.ParseRouterID(s)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func area(t *testing.T, s string) types.AreaID {
	t.Helper()
	id, err := types.ParseAreaID(s)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func testTable(t *testing.T, network string) (*Table, InterfaceConfig) {
	t.Helper()
	cfg := InterfaceConfig{
		Name:             "eth0",
		AreaID:           area(t, "0"),
		RouterID:         rid(t, "10.0.0.1"),
		NetworkType:      network,
		InterfaceAddress: [4]byte{10, 0, 0, 1},
		Options:          types.OptionE,
		InterfaceMTU:     1500,
		DeadInterval:     40,
	}
	tbl := NewTable(NopMetrics())
	tbl.now = func() time.Time { return time.Unix(1, 0) }
	tbl.ConfigureInterface(cfg)
	return tbl, cfg
}

func hello(cfg InterfaceConfig, peer types.RouterID, twoWay bool, now time.Time) HelloInput {
	return HelloInput{
		InterfaceName: cfg.Name,
		AreaID:        cfg.AreaID,
		LocalRouterID: cfg.RouterID,
		NeighborID:    peer,
		Address:       netip.AddrFrom4([4]byte(peer)),
		Priority:      1,
		TwoWay:        twoWay,
		NetworkType:   cfg.NetworkType,
		DeadInterval:  cfg.DeadInterval,
		InterfaceMTU:  cfg.InterfaceMTU,
		Now:           now,
	}
}
func peerExStartDD() packet.DBDesc {
	return packet.DBDesc{InterfaceMTU: 1500, Options: types.OptionE, Flags: packet.DDFlagInit | packet.DDFlagMore | packet.DDFlagMaster, DDSequence: 7}
}

func driveNegotiation(t *testing.T, tbl *Table, cfg InterfaceConfig, peer types.RouterID) {
	t.Helper()
	if reason := tbl.HandleDBDesc(cfg.Name, peer, peerExStartDD()); reason != "" {
		t.Fatalf("ExStart DD: %s", reason)
	}
}

func finishExchange(t *testing.T, tbl *Table, cfg InterfaceConfig, peer types.RouterID, headers ...packet.LSAHeader) {
	t.Helper()
	n, ok := tbl.lookupLocked(cfg.Name, peer)
	if !ok {
		t.Fatalf("neighbor %s missing", peer)
	}
	seq := n.DDSequence
	flags := uint8(0)
	if !n.Master {
		seq++
		flags = packet.DDFlagMaster
	}
	dd := packet.DBDesc{InterfaceMTU: 1500, Options: types.OptionE, Flags: flags, DDSequence: seq, Headers: headers}
	if reason := tbl.HandleDBDesc(cfg.Name, peer, dd); reason != "" {
		t.Fatalf("Exchange DD: %s", reason)
	}
}

func driveFull(t *testing.T, tbl *Table, cfg InterfaceConfig, peer types.RouterID) {
	t.Helper()
	driveNegotiation(t, tbl, cfg, peer)
	finishExchange(t, tbl, cfg, peer)
}

// TestOSPFNeighborInterfaceIDFlows checks the neighbor's advertised OSPFv3 Interface ID
// (RFC 5340 sec 3.4.3) is recorded from its Hello and surfaced through FloodNeighbors, so
// it can be echoed as the Neighbor Interface ID in this router's Router-LSA link. The DD
// exchange does not reset it (only Hellos carry the Interface ID).
func TestOSPFNeighborInterfaceIDFlows(t *testing.T) {
	tbl, cfg := testTable(t, NetworkPointToPoint)
	peer := rid(t, "10.0.0.2")
	in := hello(cfg, peer, true, time.Unix(1, 0))
	in.InterfaceID = 99
	if reason := tbl.Hello(in); reason != "" {
		t.Fatalf("Hello: %s", reason)
	}
	driveFull(t, tbl, cfg, peer)
	flood := tbl.FloodNeighbors(cfg.Name)
	if len(flood) != 1 || flood[0].InterfaceID != 99 {
		t.Fatalf("FloodNeighbors = %+v, want one neighbor with InterfaceID 99", flood)
	}
}

func TestOSPFNSMDownToInit(t *testing.T) {
	tbl, cfg := testTable(t, NetworkPointToPoint)
	peer := rid(t, "10.0.0.2")
	if reason := tbl.Hello(hello(cfg, peer, false, time.Unix(1, 0))); reason != "" {
		t.Fatalf("Hello: %s", reason)
	}
	snap, ok := tbl.Lookup(cfg.Name, peer)
	if !ok || snap.State != "init" || snap.DeadTime != 40 {
		t.Fatalf("snapshot = %+v ok=%v, want init with dead timer", snap, ok)
	}
}

func TestOSPFNSMDownToFull(t *testing.T) {
	tbl, cfg := testTable(t, NetworkPointToPoint)
	peer := rid(t, "10.0.0.2")
	if reason := tbl.Hello(hello(cfg, peer, true, time.Unix(1, 0))); reason != "" {
		t.Fatalf("Hello: %s", reason)
	}
	driveFull(t, tbl, cfg, peer)
	snap, _ := tbl.Lookup(cfg.Name, peer)
	if snap.State != "full" {
		t.Fatalf("state = %s, want full", snap.State)
	}
}

func TestOSPFShouldAdjBroadcast(t *testing.T) {
	tbl, cfg := testTable(t, NetworkBroadcast)
	peer := rid(t, "10.0.0.2")
	if reason := tbl.Hello(hello(cfg, peer, true, time.Unix(1, 0))); reason != "" {
		t.Fatalf("Hello: %s", reason)
	}
	snap, _ := tbl.Lookup(cfg.Name, peer)
	if snap.State != "2-way" {
		t.Fatalf("DROther peer state = %s, want 2-way", snap.State)
	}
	tbl.AdjOK(cfg.Name, cfg.RouterID, types.RouterID{})
	snap, _ = tbl.Lookup(cfg.Name, peer)
	if snap.State != "exstart" {
		t.Fatalf("local DR peer state = %s, want exstart", snap.State)
	}
}

func TestOSPFDDNegotiation(t *testing.T) {
	tbl, cfg := testTable(t, NetworkPointToPoint)
	peer := rid(t, "10.0.0.2")
	_ = tbl.Hello(hello(cfg, peer, true, time.Unix(1, 0)))
	if reason := tbl.HandleDBDesc(cfg.Name, peer, packet.DBDesc{InterfaceMTU: 1500, Options: types.OptionE, Flags: packet.DDFlagInit | packet.DDFlagMore | packet.DDFlagMaster, DDSequence: 99}); reason != "" {
		t.Fatalf("DD: %s", reason)
	}
	n, _ := tbl.lookupLocked(cfg.Name, peer)
	if n.Master || n.DDSequence != 99 || n.State != stateExchange {
		t.Fatalf("neighbor master=%v seq=%d state=%s, want slave seq 99 exchange", n.Master, n.DDSequence, n.State)
	}
}

func TestOSPFLocalMasterSendsNextDDAfterNegotiation(t *testing.T) {
	tbl, cfg := testTable(t, NetworkPointToPoint)
	cfg.RouterID = rid(t, "10.0.0.3")
	tbl.ConfigureInterface(cfg)
	sender := &fakeSender{}
	tbl.SetSender(sender)
	peer := rid(t, "10.0.0.2")
	_ = tbl.Hello(hello(cfg, peer, true, time.Unix(1, 0)))
	n, _ := tbl.lookupLocked(cfg.Name, peer)
	initial := n.DDSequence
	if got := tbl.HandleDBDesc(cfg.Name, peer, packet.DBDesc{InterfaceMTU: 1500, Options: types.OptionE, DDSequence: initial}); got != "" {
		t.Fatalf("slave DD: %s", got)
	}
	if len(sender.sent) != 2 {
		t.Fatalf("sent DD count = %d, want initial plus next DD", len(sender.sent))
	}
	p := sentPacket(t, sender, 1)
	if p.DBDesc == nil || p.DBDesc.Flags != 0 || p.DBDesc.DDSequence != initial+1 {
		t.Fatalf("next DD = %+v, want final master DD seq %d", p.DBDesc, initial+1)
	}
	if got := tbl.HandleDBDesc(cfg.Name, peer, packet.DBDesc{InterfaceMTU: 1500, Options: types.OptionE, DDSequence: initial + 1}); got != "" {
		t.Fatalf("slave echo: %s", got)
	}
	snap, _ := tbl.Lookup(cfg.Name, peer)
	if snap.State != "full" {
		t.Fatalf("state = %s, want full", snap.State)
	}
}

// RFC requirement: RFC2328-10.1-1 positive -- only one Database Description packet is outstanding on an adjacency at a time: the master holds its single unacknowledged DD and retransmits THAT packet each RetransmitInterval rather than issuing a new one (sendDBDescLocked/lastSentDD dd.go:165-181, Table.Retransmit resendLastDDLocked dd.go:194-201).
func TestOSPFDDRetransmit(t *testing.T) {
	// The master retransmits its unacked Database Description every RetransmitInterval until the
	// slave responds (RFC 2328 sec 10.8); the InactivityTimer bounds the retries. Covers the
	// previously untested neighbor Table.Retransmit DD path.
	tbl, cfg := testTable(t, NetworkPointToPoint)
	cfg.RouterID = rid(t, "10.0.0.3") // higher than the peer -> local is master, sends the initial DD
	cfg.RetransmitInterval = 5
	tbl.ConfigureInterface(cfg)
	sender := &fakeSender{}
	tbl.SetSender(sender)
	peer := rid(t, "10.0.0.2")
	start := time.Unix(1, 0)
	_ = tbl.Hello(hello(cfg, peer, true, start))
	n, _ := tbl.lookupLocked(cfg.Name, peer)
	if n.State != stateExStart {
		t.Fatalf("neighbor state = %s, want exstart (master sends the initial DD)", n.State)
	}
	before := len(sender.sent)
	if before == 0 {
		t.Fatal("master did not send an initial DD on reaching ExStart")
	}

	// Before the interval elapses, nothing is retransmitted.
	if got := tbl.Retransmit(start); got != 0 {
		t.Fatalf("retransmit before the interval = %d, want 0", got)
	}
	// After the RetransmitInterval, the master resends its DD.
	later := start.Add(time.Duration(cfg.RetransmitInterval+1) * time.Second)
	if got := tbl.Retransmit(later); got != 1 {
		t.Fatalf("retransmit after the interval = %d, want 1 (the master's DD)", got)
	}
	if len(sender.sent) != before+1 {
		t.Fatalf("sent count = %d, want %d (one DD resent)", len(sender.sent), before+1)
	}
	if p := sentPacket(t, sender, len(sender.sent)-1); p.DBDesc == nil {
		t.Fatalf("retransmitted packet is not a Database Description")
	}
}

func TestOSPFDDMTUMismatch(t *testing.T) {
	tbl, cfg := testTable(t, NetworkPointToPoint)
	peer := rid(t, "10.0.0.2")
	_ = tbl.Hello(hello(cfg, peer, true, time.Unix(1, 0)))
	if got := tbl.HandleDBDesc(cfg.Name, peer, packet.DBDesc{InterfaceMTU: 9000}); got != "mtu-mismatch" {
		t.Fatalf("reason = %q, want mtu-mismatch", got)
	}
	snap, _ := tbl.Lookup(cfg.Name, peer)
	if snap.State != "exstart" {
		t.Fatalf("state = %s, want exstart", snap.State)
	}
}

func TestOSPFDDMTUIgnore(t *testing.T) {
	tbl, cfg := testTable(t, NetworkPointToPoint)
	cfg.MTUIgnore = true
	tbl.ConfigureInterface(cfg)
	peer := rid(t, "10.0.0.2")
	in := hello(cfg, peer, true, time.Unix(1, 0))
	in.MTUIgnore = true
	_ = tbl.Hello(in)
	dd := peerExStartDD()
	dd.InterfaceMTU = 9000
	if got := tbl.HandleDBDesc(cfg.Name, peer, dd); got != "" {
		t.Fatalf("reason = %q, want accepted", got)
	}
}

// RFC requirement: RFC2328-10.1-1 negative -- a duplicate Database Description does not open a second outstanding DD: the slave resends the previously sent DD verbatim (sameDD holds) instead of advancing the sequence and emitting a new one (handleDBDesc duplicate branch, dd.go:52-58).
func TestOSPFDuplicateDD(t *testing.T) {
	tbl, cfg := testTable(t, NetworkPointToPoint)
	sender := &fakeSender{}
	tbl.SetSender(sender)
	peer := rid(t, "10.0.0.2")
	_ = tbl.Hello(hello(cfg, peer, true, time.Unix(1, 0)))
	dd := peerExStartDD()
	if got := tbl.HandleDBDesc(cfg.Name, peer, dd); got != "" {
		t.Fatalf("DD result = %q, want accepted", got)
	}
	before := len(sender.sent)
	if got := tbl.HandleDBDesc(cfg.Name, peer, dd); got != "duplicate-resend" {
		t.Fatalf("duplicate result = %q, want duplicate-resend", got)
	}
	if len(sender.sent) != before+1 {
		t.Fatalf("sent DD count after duplicate = %d, want %d", len(sender.sent), before+1)
	}
	if a, b := sentPacket(t, sender, before-1).DBDesc, sentPacket(t, sender, before).DBDesc; a == nil || b == nil || !sameDD(*a, *b) {
		t.Fatalf("duplicate resend = %+v, want previous sent DD %+v", b, a)
	}
}

func testHeader(t *testing.T, seq types.LSSequenceNumber) packet.LSAHeader {
	t.Helper()
	return packet.LSAHeader{
		Age:               types.LSAge(10),
		Type:              types.LSTypeRouter,
		LinkStateID:       types.LinkStateID{1, 1, 1, 1},
		AdvertisingRouter: rid(t, "10.0.0.2"),
		Sequence:          seq,
		Checksum:          10,
		Length:            types.LSAHeaderLen,
	}
}

func testHeaderIndex(t *testing.T, i int) packet.LSAHeader {
	h := testHeader(t, types.InitialSequenceNumber+1)
	h.LinkStateID = types.LinkStateID{byte(i >> 24), byte(i >> 16), byte(i >> 8), byte(i)}
	return h
}

func TestOSPFLSRequestListPopulated(t *testing.T) {
	tbl, cfg := testTable(t, NetworkPointToPoint)
	peer := rid(t, "10.0.0.2")
	h := testHeader(t, types.InitialSequenceNumber+1)
	older := h
	older.Sequence = types.InitialSequenceNumber
	tbl.lsdb = fakeLSDB{h.Key(): {Header: older}}
	_ = tbl.Hello(hello(cfg, peer, true, time.Unix(1, 0)))
	driveNegotiation(t, tbl, cfg, peer)
	finishExchange(t, tbl, cfg, peer, h)
	snap, _ := tbl.Lookup(cfg.Name, peer)
	if snap.State != "loading" || snap.RequestCount != 1 {
		t.Fatalf("snapshot = %+v, want loading with one request", snap)
	}
}

func TestOSPFNilLSDBDDDoesNotRequest(t *testing.T) {
	tbl, cfg := testTable(t, NetworkPointToPoint)
	peer := rid(t, "10.0.0.2")
	h := testHeader(t, types.InitialSequenceNumber+1)
	_ = tbl.Hello(hello(cfg, peer, true, time.Unix(1, 0)))
	driveNegotiation(t, tbl, cfg, peer)
	finishExchange(t, tbl, cfg, peer, h)
	snap, _ := tbl.Lookup(cfg.Name, peer)
	if snap.State != "full" || snap.RequestCount != 0 {
		t.Fatalf("snapshot = %+v, want full with no requests when LSDB is absent", snap)
	}
}

func TestOSPFLoadingDrainToFull(t *testing.T) {
	tbl, cfg := testTable(t, NetworkPointToPoint)
	peer := rid(t, "10.0.0.2")
	h := testHeader(t, types.InitialSequenceNumber+1)
	tbl.lsdb = fakeLSDB{}
	tbl.lsdb.Install(cfg.AreaID, packet.LSA{Header: h})
	_ = tbl.Hello(hello(cfg, peer, true, time.Unix(1, 0)))
	driveNegotiation(t, tbl, cfg, peer)
	finishExchange(t, tbl, cfg, peer, h)
	if got := tbl.HandleLSUpdate(cfg.Name, peer, packet.LSUpdate{LSAs: []packet.LSA{{Header: h}}}); got != "" {
		t.Fatalf("LSUpdate: %s", got)
	}
	snap, _ := tbl.Lookup(cfg.Name, peer)
	if snap.State != "full" || snap.RequestCount != 0 {
		t.Fatalf("snapshot = %+v, want full with empty request list", snap)
	}
}

func TestOSPFv3LinkScopedLSAsEnterDDDatabaseSummary(t *testing.T) {
	tbl, cfg := testTable(t, NetworkPointToPoint)
	peer := rid(t, "10.0.0.2")
	h := testHeader(t, types.InitialSequenceNumber)
	h.Type = types.LSTypeLink
	h.LinkStateID = types.LinkStateID{0, 0, 0, 2}
	db := &fakeScopedLSDB{links: map[string]fakeLSDB{cfg.Name: {h.Key(): {Header: h}}}}
	tbl.SetLSDB(db)

	_ = tbl.Hello(hello(cfg, peer, true, time.Unix(1, 0)))
	n, ok := tbl.lookupLocked(cfg.Name, peer)
	if !ok {
		t.Fatalf("neighbor %s missing", peer)
	}
	if len(n.SummaryList) != 1 || n.SummaryList[0].Key() != h.Key() {
		t.Fatalf("DD summary = %+v, want link-scoped LSA %v", n.SummaryList, h.Key())
	}
}

func TestOSPFv3LinkScopedLoadingDrainToFull(t *testing.T) {
	tbl, cfg := testTable(t, NetworkPointToPoint)
	peer := rid(t, "10.0.0.2")
	h := testHeader(t, types.InitialSequenceNumber+1)
	h.Type = types.LSTypeLink
	h.LinkStateID = types.LinkStateID{0, 0, 0, 2}
	db := &fakeScopedLSDB{area: fakeLSDB{}, links: map[string]fakeLSDB{cfg.Name: {}}}
	tbl.SetLSDB(db)

	_ = tbl.Hello(hello(cfg, peer, true, time.Unix(1, 0)))
	driveNegotiation(t, tbl, cfg, peer)
	finishExchange(t, tbl, cfg, peer, h)
	snap, _ := tbl.Lookup(cfg.Name, peer)
	if snap.State != "loading" || snap.RequestCount != 1 {
		t.Fatalf("snapshot = %+v, want loading with one link-scoped request", snap)
	}

	db.links[cfg.Name][h.Key()] = packet.LSA{Header: h}
	if got := tbl.HandleLSUpdate(cfg.Name, peer, packet.LSUpdate{LSAs: []packet.LSA{{Header: h}}}); got != "" {
		t.Fatalf("LSUpdate: %s", got)
	}
	snap, _ = tbl.Lookup(cfg.Name, peer)
	if snap.State != "full" || snap.RequestCount != 0 {
		t.Fatalf("snapshot = %+v, want full after link-scoped request is satisfied", snap)
	}
}

// RFC requirement: RFC2328-10.2-1 positive -- an LS Request naming an LSA that is not in the database generates the BadLSReq event: the adjacency is torn down back to ExStart and the Database Exchange restarts with a fresh initial DD (handleLSReq, lsreq.go:69-74).
func TestOSPFBadLSReqRestart(t *testing.T) {
	tbl, cfg := testTable(t, NetworkPointToPoint)
	sender := &fakeSender{}
	tbl.SetSender(sender)
	peer := rid(t, "10.0.0.2")
	h := testHeader(t, types.InitialSequenceNumber)
	tbl.lsdb = fakeLSDB{}
	_ = tbl.Hello(hello(cfg, peer, true, time.Unix(1, 0)))
	driveFull(t, tbl, cfg, peer)
	before := len(sender.sent)
	if got := tbl.HandleLSReq(cfg.Name, peer, packet.LSReq{Requests: []packet.LSRequestEntry{{Type: h.Type, LinkStateID: h.LinkStateID, AdvertisingRouter: h.AdvertisingRouter}}}); got != reasonBadLSReq {
		t.Fatalf("LSReq: %s, want bad-lsreq", got)
	}
	snap, _ := tbl.Lookup(cfg.Name, peer)
	if snap.State != "exstart" {
		t.Fatalf("state = %s, want exstart", snap.State)
	}
	if len(sender.sent) != before+1 {
		t.Fatalf("sent DD count after BadLSReq = %d, want %d", len(sender.sent), before+1)
	}
	restart := sentPacket(t, sender, before)
	if restart.DBDesc == nil || restart.DBDesc.Flags != packet.DDFlagInit|packet.DDFlagMore|packet.DDFlagMaster {
		t.Fatalf("restart DD = %+v, want initial DD", restart.DBDesc)
	}
}

func TestOSPFSeqNumberMismatchRestart(t *testing.T) {
	tbl, cfg := testTable(t, NetworkPointToPoint)
	peer := rid(t, "10.0.0.2")
	_ = tbl.Hello(hello(cfg, peer, true, time.Unix(1, 0)))
	driveNegotiation(t, tbl, cfg, peer)
	if got := tbl.HandleDBDesc(cfg.Name, peer, packet.DBDesc{InterfaceMTU: 1500, Flags: packet.DDFlagInit, DDSequence: 7}); got != "seq-number-mismatch" {
		t.Fatalf("DD: %s, want seq-number-mismatch", got)
	}
}

func TestOSPFAdjOKDropsToTwoWay(t *testing.T) {
	tbl, cfg := testTable(t, NetworkBroadcast)
	cfg.LocalDR = cfg.RouterID
	tbl.ConfigureInterface(cfg)
	peer := rid(t, "10.0.0.2")
	in := hello(cfg, peer, true, time.Unix(1, 0))
	in.LocalDR = cfg.RouterID
	_ = tbl.Hello(in)
	driveNegotiation(t, tbl, cfg, peer)
	tbl.AdjOK(cfg.Name, types.RouterID{}, types.RouterID{})
	snap, _ := tbl.Lookup(cfg.Name, peer)
	if snap.State != "2-way" {
		t.Fatalf("state = %s, want 2-way", snap.State)
	}
}

func TestOSPFNSMSendsInitialDDOnExStart(t *testing.T) {
	tbl, cfg := testTable(t, NetworkPointToPoint)
	sender := &fakeSender{}
	tbl.SetSender(sender)
	peer := rid(t, "10.0.0.2")
	if reason := tbl.Hello(hello(cfg, peer, true, time.Unix(1, 0))); reason != "" {
		t.Fatalf("Hello: %s", reason)
	}
	if len(sender.sent) != 1 {
		t.Fatalf("sent DD count = %d, want 1", len(sender.sent))
	}
	if sender.dst[0] != netip.MustParseAddr("10.0.0.2") {
		t.Fatalf("dst = %s, want peer address", sender.dst[0])
	}
	p := sentPacket(t, sender, 0)
	if p.DBDesc == nil || p.DBDesc.Flags != packet.DDFlagInit|packet.DDFlagMore|packet.DDFlagMaster || p.DBDesc.Options != types.OptionE {
		t.Fatalf("sent packet = %+v, want initial DBDesc with E option", p.DBDesc)
	}
}

func TestOSPFDDChunkedByInterfaceMTU(t *testing.T) {
	tbl, cfg := testTable(t, NetworkPointToPoint)
	sender := &fakeSender{}
	tbl.SetSender(sender)
	peer := rid(t, "10.0.0.2")
	_ = tbl.Hello(hello(cfg, peer, true, time.Unix(1, 0)))
	sender.sent = nil
	sender.dst = nil
	n, _ := tbl.lookupLocked(cfg.Name, peer)
	capacity := ddHeaderCapacity(cfg.InterfaceMTU)
	n.SummaryList = make([]packet.LSAHeader, capacity+1)
	for i := range n.SummaryList {
		n.SummaryList[i] = testHeaderIndex(t, i+1)
	}
	tbl.sendDBDescLocked(cfg, n, 0)
	if len(sender.sent) != 1 {
		t.Fatalf("DD packets = %d, want 1", len(sender.sent))
	}
	p := sentPacket(t, sender, 0)
	payloadLimit := ospfPayloadLimit(cfg.InterfaceMTU)
	if len(sender.sent[0]) > payloadLimit {
		t.Fatalf("DD payload length = %d, want <= %d", len(sender.sent[0]), payloadLimit)
	}
	if p.DBDesc == nil || len(p.DBDesc.Headers) > capacity || p.DBDesc.Flags&packet.DDFlagMore == 0 {
		t.Fatalf("DD = %+v, want at most %d headers and More set", p.DBDesc, capacity)
	}
}

func TestOSPFDDRejectedBeforeShouldAdj(t *testing.T) {
	tbl, cfg := testTable(t, NetworkBroadcast)
	peer := rid(t, "10.0.0.2")
	_ = tbl.Hello(hello(cfg, peer, true, time.Unix(1, 0)))
	if got := tbl.HandleDBDesc(cfg.Name, peer, peerExStartDD()); got != "adjacency-not-ready" {
		t.Fatalf("DD result = %q, want adjacency-not-ready", got)
	}
	snap, _ := tbl.Lookup(cfg.Name, peer)
	if snap.State != "2-way" {
		t.Fatalf("state = %s, want 2-way", snap.State)
	}
}

func TestOSPFDDInvalidExStartRejected(t *testing.T) {
	tbl, cfg := testTable(t, NetworkPointToPoint)
	peer := rid(t, "10.0.0.2")
	_ = tbl.Hello(hello(cfg, peer, true, time.Unix(1, 0)))
	if got := tbl.HandleDBDesc(cfg.Name, peer, packet.DBDesc{InterfaceMTU: 1500, DDSequence: 7}); got != "negotiation" {
		t.Fatalf("DD result = %q, want negotiation", got)
	}
	snap, _ := tbl.Lookup(cfg.Name, peer)
	if snap.State != "exstart" {
		t.Fatalf("state = %s, want exstart", snap.State)
	}
}

func TestOSPFDDExStartMissingMoreRejected(t *testing.T) {
	tbl, cfg := testTable(t, NetworkPointToPoint)
	peer := rid(t, "10.0.0.2")
	_ = tbl.Hello(hello(cfg, peer, true, time.Unix(1, 0)))
	if got := tbl.HandleDBDesc(cfg.Name, peer, packet.DBDesc{InterfaceMTU: 1500, Options: types.OptionE, Flags: packet.DDFlagInit | packet.DDFlagMaster, DDSequence: 7}); got != "negotiation" {
		t.Fatalf("DD result = %q, want negotiation", got)
	}
	snap, _ := tbl.Lookup(cfg.Name, peer)
	if snap.State != "exstart" {
		t.Fatalf("state = %s, want exstart", snap.State)
	}
}

func TestOSPFLSReqWithoutLSDBDoesNotRestart(t *testing.T) {
	tbl, cfg := testTable(t, NetworkPointToPoint)
	peer := rid(t, "10.0.0.2")
	h := testHeader(t, types.InitialSequenceNumber)
	_ = tbl.Hello(hello(cfg, peer, true, time.Unix(1, 0)))
	driveFull(t, tbl, cfg, peer)
	if got := tbl.HandleLSReq(cfg.Name, peer, packet.LSReq{Requests: []packet.LSRequestEntry{{Type: h.Type, LinkStateID: h.LinkStateID, AdvertisingRouter: h.AdvertisingRouter}}}); got != "lsdb-unavailable" {
		t.Fatalf("LSReq: %s, want lsdb-unavailable", got)
	}
	snap, _ := tbl.Lookup(cfg.Name, peer)
	if snap.State != "full" {
		t.Fatalf("state = %s, want full", snap.State)
	}
}

// RFC requirement: RFC2328-10.2-1 negative -- the BadLSReq restart is confined to unsatisfiable requests: an LS Request for an LSA the database holds is answered with an LS Update and does not restart the exchange (handleLSReq, lsreq.go:75-82).
func TestOSPFValidLSReqSendsLSUpdate(t *testing.T) {
	tbl, cfg := testTable(t, NetworkPointToPoint)
	sender := &fakeSender{}
	tbl.SetSender(sender)
	peer := rid(t, "10.0.0.2")
	h := testHeader(t, types.InitialSequenceNumber)
	tbl.lsdb = fakeLSDB{h.Key(): {Header: h}}
	_ = tbl.Hello(hello(cfg, peer, true, time.Unix(1, 0)))
	driveFull(t, tbl, cfg, peer)
	before := len(sender.sent)
	req := packet.LSReq{Requests: []packet.LSRequestEntry{{Type: h.Type, LinkStateID: h.LinkStateID, AdvertisingRouter: h.AdvertisingRouter}}}
	if got := tbl.HandleLSReq(cfg.Name, peer, req); got != "" {
		t.Fatalf("LSReq: %s", got)
	}
	if len(sender.sent) != before+1 {
		t.Fatalf("sent packet count = %d, want %d", len(sender.sent), before+1)
	}
	p := sentPacket(t, sender, before)
	if p.LSUpdate == nil || len(p.LSUpdate.LSAs) != 1 || p.LSUpdate.LSAs[0].Header.Key() != h.Key() {
		t.Fatalf("sent packet = %+v, want one LSUpdate LSA", p.LSUpdate)
	}
}

func TestOSPFLSReqChunked(t *testing.T) {
	tbl, cfg := testTable(t, NetworkPointToPoint)
	sender := &fakeSender{}
	tbl.SetSender(sender)
	peer := rid(t, "10.0.0.2")
	_ = tbl.Hello(hello(cfg, peer, true, time.Unix(1, 0)))
	sender.sent = nil
	sender.dst = nil
	n, _ := tbl.lookupLocked(cfg.Name, peer)
	n.State = stateLoading
	capacity := lsReqEntryCapacity(cfg.InterfaceMTU)
	n.RequestList = make([]packet.LSAHeader, capacity+1)
	for i := range n.RequestList {
		n.RequestList[i] = testHeaderIndex(t, i+1)
	}
	tbl.sendLSReqLocked(cfg, n)
	if len(sender.sent) != 2 {
		t.Fatalf("LSReq packets = %d, want 2", len(sender.sent))
	}
	payloadLimit := ospfPayloadLimit(cfg.InterfaceMTU)
	for i := range sender.sent {
		p := sentPacket(t, sender, i)
		if len(sender.sent[i]) > payloadLimit {
			t.Fatalf("packet %d LSReq payload length = %d, want <= %d", i, len(sender.sent[i]), payloadLimit)
		}
		if p.LSReq == nil || len(p.LSReq.Requests) > capacity {
			t.Fatalf("packet %d LSReq = %+v", i, p.LSReq)
		}
	}
}

func TestOSPFRequestListLimitRestartsExchange(t *testing.T) {
	tbl, cfg := testTable(t, NetworkPointToPoint)
	sender := &fakeSender{}
	tbl.SetSender(sender)
	peer := rid(t, "10.0.0.2")
	tbl.lsdb = fakeLSDB{}
	_ = tbl.Hello(hello(cfg, peer, true, time.Unix(1, 0)))
	driveNegotiation(t, tbl, cfg, peer)
	n, _ := tbl.lookupLocked(cfg.Name, peer)
	n.RequestList = make([]packet.LSAHeader, maxRequestList)
	for i := range n.RequestList {
		n.RequestList[i] = testHeaderIndex(t, i+1)
	}
	dd := packet.DBDesc{
		InterfaceMTU: 1500,
		Options:      types.OptionE,
		Flags:        packet.DDFlagMaster,
		DDSequence:   n.DDSequence + 1,
		Headers:      []packet.LSAHeader{testHeaderIndex(t, maxRequestList+1)},
	}
	if got := tbl.HandleDBDesc(cfg.Name, peer, dd); got != reasonRequestListLimit {
		t.Fatalf("DD result = %q, want %s", got, reasonRequestListLimit)
	}
	snap, _ := tbl.Lookup(cfg.Name, peer)
	if snap.State != "exstart" || snap.RequestCount != 0 {
		t.Fatalf("snapshot = %+v, want ExStart with request list cleared", snap)
	}
}

func TestOSPFLSUpdateChunked(t *testing.T) {
	tbl, cfg := testTable(t, NetworkPointToPoint)
	sender := &fakeSender{}
	tbl.SetSender(sender)
	peer := rid(t, "10.0.0.2")
	_ = tbl.Hello(hello(cfg, peer, true, time.Unix(1, 0)))
	sender.sent = nil
	sender.dst = nil
	n, _ := tbl.lookupLocked(cfg.Name, peer)
	capacity := lsUpdateBodyCapacity(cfg.InterfaceMTU)
	lsaCount := capacity/types.LSAHeaderLen + 1
	lsas := make([]packet.LSA, lsaCount)
	for i := range lsas {
		lsas[i] = packet.LSA{Header: testHeaderIndex(t, i+1)}
	}
	tbl.sendLSUpdateLocked(cfg, n, lsas)
	if len(sender.sent) != 2 {
		t.Fatalf("LSUpdate packets = %d, want 2", len(sender.sent))
	}
	payloadLimit := ospfPayloadLimit(cfg.InterfaceMTU)
	for i := range sender.sent {
		p := sentPacket(t, sender, i)
		if len(sender.sent[i]) > payloadLimit {
			t.Fatalf("packet %d LSUpdate payload length = %d, want <= %d", i, len(sender.sent[i]), payloadLimit)
		}
		if p.LSUpdate == nil || len(p.LSUpdate.LSAs) == 0 {
			t.Fatalf("packet %d LSUpdate = %+v", i, p.LSUpdate)
		}
	}
}

func TestOSPFLSReqBeforeExchangeDoesNotStartAdjacency(t *testing.T) {
	tbl, cfg := testTable(t, NetworkBroadcast)
	tbl.lsdb = fakeLSDB{}
	peer := rid(t, "10.0.0.2")
	h := testHeader(t, types.InitialSequenceNumber)
	_ = tbl.Hello(hello(cfg, peer, true, time.Unix(1, 0)))
	if got := tbl.HandleLSReq(cfg.Name, peer, packet.LSReq{Requests: []packet.LSRequestEntry{{Type: h.Type, LinkStateID: h.LinkStateID, AdvertisingRouter: h.AdvertisingRouter}}}); got != "state" {
		t.Fatalf("LSReq before Exchange = %q, want state", got)
	}
	snap, _ := tbl.Lookup(cfg.Name, peer)
	if snap.State != "2-way" {
		t.Fatalf("state = %s, want 2-way", snap.State)
	}
}
func TestOSPFLoadingIgnoresOlderLSUpdate(t *testing.T) {
	tbl, cfg := testTable(t, NetworkPointToPoint)
	peer := rid(t, "10.0.0.2")
	h := testHeader(t, types.InitialSequenceNumber+2)
	older := h
	older.Sequence = types.InitialSequenceNumber + 1
	tbl.lsdb = fakeLSDB{}
	_ = tbl.Hello(hello(cfg, peer, true, time.Unix(1, 0)))
	driveNegotiation(t, tbl, cfg, peer)
	finishExchange(t, tbl, cfg, peer, h)
	if got := tbl.HandleLSUpdate(cfg.Name, peer, packet.LSUpdate{LSAs: []packet.LSA{{Header: older}}}); got != "" {
		t.Fatalf("older LSUpdate: %s", got)
	}
	snap, _ := tbl.Lookup(cfg.Name, peer)
	if snap.State != "loading" || snap.RequestCount != 1 {
		t.Fatalf("snapshot = %+v, want loading with pending request", snap)
	}
	tbl.lsdb.Install(cfg.AreaID, packet.LSA{Header: h})
	if got := tbl.HandleLSUpdate(cfg.Name, peer, packet.LSUpdate{LSAs: []packet.LSA{{Header: h}}}); got != "" {
		t.Fatalf("matching LSUpdate: %s", got)
	}
	snap, _ = tbl.Lookup(cfg.Name, peer)
	if snap.State != "full" || snap.RequestCount != 0 {
		t.Fatalf("snapshot = %+v, want full with drained request", snap)
	}
}

func TestOSPFNeighborMetricCounts(t *testing.T) {
	neighbors := &fakeGaugeVec{}
	tbl := NewTable(Metrics{Neighbors: neighbors, AdjacenciesFull: &fakeGaugeVec{}, NSMEvents: fakeCounterVec{}})
	cfg := InterfaceConfig{
		Name:             "eth0",
		AreaID:           area(t, "0"),
		RouterID:         rid(t, "10.0.0.1"),
		NetworkType:      NetworkBroadcast,
		InterfaceAddress: [4]byte{10, 0, 0, 1},
		InterfaceMTU:     1500,
		DeadInterval:     40,
	}
	tbl.ConfigureInterface(cfg)
	peer1 := rid(t, "10.0.0.2")
	peer2 := rid(t, "10.0.0.3")
	_ = tbl.Hello(hello(cfg, peer1, false, time.Unix(1, 0)))
	_ = tbl.Hello(hello(cfg, peer2, false, time.Unix(1, 0)))
	key := strings.Join([]string{cfg.AreaID.String(), cfg.Name, "init"}, "\xff")
	if got := neighbors.values[key]; got != 2 {
		t.Fatalf("init gauge = %v, want 2", got)
	}
	tbl.NeighborDown(cfg.Name, peer1)
	if got := neighbors.values[key]; got != 1 {
		t.Fatalf("init gauge after down = %v, want 1", got)
	}
}

func TestOSPFInactivityTimerKills(t *testing.T) {
	tbl, cfg := testTable(t, NetworkPointToPoint)
	peer := rid(t, "10.0.0.2")
	_ = tbl.Hello(hello(cfg, peer, true, time.Unix(1, 0)))
	if expired := tbl.Expire(time.Unix(42, 0)); expired != 1 {
		t.Fatalf("expired = %d, want 1", expired)
	}
	snap, _ := tbl.Lookup(cfg.Name, peer)
	if snap.State != stateNameDown {
		t.Fatalf("state = %s, want down", snap.State)
	}
}

func TestOSPFDownNeighborsReapedForAdmission(t *testing.T) {
	tbl, cfg := testTable(t, NetworkPointToPoint)
	for i := range maxNeighbors {
		peer := types.RouterID{10, 1, byte(i >> 8), byte(i)}
		if got := tbl.Hello(hello(cfg, peer, false, time.Unix(1, 0))); got != "" {
			t.Fatalf("Hello %d: %s", i, got)
		}
	}
	if expired := tbl.Expire(time.Unix(42, 0)); expired != maxNeighbors {
		t.Fatalf("expired = %d, want %d", expired, maxNeighbors)
	}
	peer := types.RouterID{10, 2, 0, 1}
	if got := tbl.Hello(hello(cfg, peer, false, time.Unix(43, 0))); got != "" {
		t.Fatalf("new Hello after reaping Down neighbors: %s", got)
	}
	snap, ok := tbl.Lookup(cfg.Name, peer)
	if !ok || snap.State != "init" {
		t.Fatalf("new neighbor snapshot = %+v ok=%v, want init", snap, ok)
	}
}

func TestOSPFKillNbr(t *testing.T) {
	tbl, cfg := testTable(t, NetworkPointToPoint)
	peer := rid(t, "10.0.0.2")
	_ = tbl.Hello(hello(cfg, peer, true, time.Unix(1, 0)))
	tbl.NeighborDown(cfg.Name, peer)
	snap, _ := tbl.Lookup(cfg.Name, peer)
	if snap.State != stateNameDown {
		t.Fatalf("state = %s, want down", snap.State)
	}
}

func TestOSPFNeighborTableKeying(t *testing.T) {
	tbl, cfg := testTable(t, NetworkBroadcast)
	_ = tbl.Hello(hello(cfg, rid(t, "10.0.0.2"), false, time.Unix(1, 0)))
	_ = tbl.Hello(hello(cfg, rid(t, "10.0.0.3"), false, time.Unix(1, 0)))
	if got := len(tbl.Snapshot()); got != 2 {
		t.Fatalf("snapshot rows = %d, want 2", got)
	}
}

func TestOSPFNeighborSnapshot(t *testing.T) {
	tbl, cfg := testTable(t, NetworkPointToPoint)
	peer := rid(t, "10.0.0.2")
	_ = tbl.Hello(hello(cfg, peer, false, time.Unix(1, 0)))
	snap, ok := tbl.Lookup(cfg.Name, peer)
	if !ok || snap.Interface != "eth0" || snap.RouterID != "10.0.0.2" || snap.Address != "10.0.0.2" || snap.State != "init" {
		t.Fatalf("snapshot = %+v ok=%v", snap, ok)
	}
}
