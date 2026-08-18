// VALIDATES: AC-7 per-source neutral features (fan-out, out/in byte ratio,
// dest-port entropy, new-peer, rare-port), AC-8 coarse beaconing over active-tick
// gaps, and AC-10 cardinality caps + idle eviction -- all computed from the
// observation feed via internal/core/stats.
// PREVENTS: exfil ratio inversion (in vs out role mixup), counting a common port
// as rare, and unbounded per-source state under source churn.

package trafficfeature

import (
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/observation"
)

func featureOf(snap *Snapshot, addr netip.Addr) (FeatureEntry, bool) {
	for _, fe := range snap.Sources {
		if fe.Addr == addr {
			return fe, true
		}
	}
	return FeatureEntry{}, false
}

func flowObs(src, dst netip.Addr, port uint16, bytes float64, at time.Time) observation.Observation {
	return observation.Observation{
		Kind:    observation.KindFlow,
		Feature: observation.FeatureFlowBytes,
		Flow:    observation.FlowKey{Src: src, Dst: dst, DstPort: port, Proto: 6},
		Value:   bytes,
		At:      at,
	}
}

// flowObsAS is flowObs with an origin AS stamped on it, as the observation's
// publisher stamps it (0 means the AS is unknown).
func flowObsAS(src, dst netip.Addr, port uint16, bytes float64, at time.Time, srcAS uint32) observation.Observation {
	obs := flowObs(src, dst, port, bytes, at)
	obs.SrcAS = srcAS
	return obs
}

// VALIDATES: child-6 AC-2 -- an origin AS on a flow observation reaches the
// FeatureEntry of the source entity, at both ends of the uint32 range
// (boundary row of the spec's Boundary Tests table).
func TestFeatureEntryCarriesSrcAS(t *testing.T) {
	a := newAggregator()
	t0 := time.Now()
	s := netip.MustParseAddr("198.51.100.10")
	top := netip.MustParseAddr("198.51.100.11")
	d := netip.MustParseAddr("203.0.113.10")

	a.ingest(flowObsAS(s, d, 443, 1000, t0, 64500))
	a.ingest(flowObsAS(top, d, 443, 900, t0, 4294967295))

	snap := a.snapshot(t0.Add(time.Second))
	for addr, want := range map[netip.Addr]uint32{s: 64500, top: 4294967295} {
		fe, ok := featureOf(snap, addr)
		if !ok {
			t.Fatalf("source %v not in snapshot", addr)
		}
		if fe.SrcAS != want {
			t.Errorf("source %v: SrcAS = %d, want %d", addr, fe.SrcAS, want)
		}
	}
}

// VALIDATES: child-6 AC-3 and R-2 -- an unknown AS stays the 0 sentinel on the
// FeatureEntry, and a later unknown-AS flow (a RIB miss) does not erase an AS
// the same source was already attributed to in an earlier window.
func TestFeatureEntrySrcASUnsetWhenUnknown(t *testing.T) {
	a := newAggregator()
	t0 := time.Now()
	unknown := netip.MustParseAddr("198.51.100.20")
	known := netip.MustParseAddr("198.51.100.21")
	d := netip.MustParseAddr("203.0.113.20")

	a.ingest(flowObsAS(unknown, d, 443, 1000, t0, 0))
	a.ingest(flowObsAS(known, d, 443, 1000, t0, 64500))
	snap := a.snapshot(t0.Add(time.Second))

	fe, ok := featureOf(snap, unknown)
	if !ok {
		t.Fatalf("source %v not in snapshot", unknown)
	}
	if fe.SrcAS != 0 {
		t.Errorf("unknown-AS source: SrcAS = %d, want 0", fe.SrcAS)
	}

	// Second window: the same source is seen again with no AS (RIB miss). The AS
	// is a property of the source's prefix, so the known value must survive.
	a.ingest(flowObsAS(known, d, 443, 1000, t0.Add(time.Second), 0))
	snap = a.snapshot(t0.Add(2 * time.Second))
	fe, ok = featureOf(snap, known)
	if !ok {
		t.Fatalf("source %v not in second snapshot", known)
	}
	if fe.SrcAS != 64500 {
		t.Errorf("SrcAS = %d after an unknown-AS flow, want 64500 preserved", fe.SrcAS)
	}
}

// VALIDATES: child-6 A-4 -- the observation's SrcAS describes the flow's SOURCE,
// so an address that is only the DESTINATION of that flow must not inherit it.
func TestFeatureEntrySrcASSourceRoleOnly(t *testing.T) {
	a := newAggregator()
	t0 := time.Now()
	s := netip.MustParseAddr("198.51.100.30")
	mid := netip.MustParseAddr("203.0.113.30")
	far := netip.MustParseAddr("203.0.113.31")

	// mid is the destination of a flow from AS 64500, then a source itself with
	// no AS of its own. Its FeatureEntry must not report 64500.
	a.ingest(flowObsAS(s, mid, 443, 1000, t0, 64500))
	a.ingest(flowObsAS(mid, far, 80, 500, t0, 0))

	snap := a.snapshot(t0.Add(time.Second))
	fe, ok := featureOf(snap, mid)
	if !ok {
		t.Fatalf("source %v not in snapshot", mid)
	}
	if fe.SrcAS != 0 {
		t.Errorf("destination-role entity inherited SrcAS = %d, want 0", fe.SrcAS)
	}
}

// destOf finds a destination-axis entry in a snapshot.
func destOf(snap *Snapshot, addr netip.Addr) (FeatureEntry, bool) {
	for _, fe := range snap.Dests {
		if fe.Addr == addr {
			return fe, true
		}
	}
	return FeatureEntry{}, false
}

// portOf finds a port-axis entry in a snapshot.
func portOf(snap *Snapshot, key PortKey) (PortFeatureEntry, bool) {
	for _, pe := range snap.Ports {
		if pe.PortKey == key {
			return pe, true
		}
	}
	return PortFeatureEntry{}, false
}

// flowObsPorts is flowObs with both transport ports and the protocol spelled out,
// for the port axis (whose amplification ratio reads the SOURCE port).
func flowObsPorts(src, dst netip.Addr, srcPort, dstPort uint16, proto uint8, bytes float64, at time.Time) observation.Observation {
	return observation.Observation{
		Kind:    observation.KindFlow,
		Feature: observation.FeatureFlowBytes,
		Flow: observation.FlowKey{
			Src: src, Dst: dst, SrcPort: srcPort, DstPort: dstPort, Proto: proto,
		},
		Value: bytes,
		At:    at,
	}
}

// VALIDATES: child-5 AC-1 -- many sources to one destination produce a
// destination-axis FeatureEntry whose fan-in, in/out ratio and received-port
// entropy describe the TARGET, and an address that only ever sent is absent from
// that axis.
// PREVENTS: the dest ratio being computed in the source (out/in) sense, fan-in
// counting flows instead of distinct sources, and a pure sender being duplicated
// onto the destination axis.
func TestFeatureDestEntry(t *testing.T) {
	a := newAggregator()
	t0 := time.Now()
	target := netip.MustParseAddr("203.0.113.50")
	s1 := netip.MustParseAddr("198.51.100.1")
	s2 := netip.MustParseAddr("198.51.100.2")
	s3 := netip.MustParseAddr("198.51.100.3")

	// Three sources send 1000 each to the target on two distinct ports; the target
	// answers s1 with 200. s1/s2/s3 never receive anything.
	a.ingest(flowObs(s1, target, 443, 1000, t0))
	a.ingest(flowObs(s2, target, 443, 1000, t0))
	a.ingest(flowObs(s3, target, 8080, 1000, t0))
	a.ingest(flowObs(target, s1, 443, 200, t0))

	snap := a.snapshot(t0.Add(time.Second))
	fe, ok := destOf(snap, target)
	if !ok {
		t.Fatalf("destination %v not on the dest axis", target)
	}
	if fe.FanOut != 3 {
		t.Errorf("fan-in = %d, want 3 (distinct sources)", fe.FanOut)
	}
	// in=3000, out=200 -> 15 in the destination (in/out) sense, NOT 0.0667.
	if fe.OutInRatio != 15 {
		t.Errorf("dest ratio = %v, want 15 (in/out, not the source sense)", fe.OutInRatio)
	}
	if fe.PortEntropy <= 0 {
		t.Errorf("dest port entropy = %v, want > 0 (received on 443 and 8080)", fe.PortEntropy)
	}
	if !fe.NewPeer {
		t.Error("first-sighting destination must report new-peer")
	}
	if fe.SrcAS != 0 {
		t.Errorf("dest entry SrcAS = %d, want 0 (stamped on the source axis only)", fe.SrcAS)
	}
	for _, pure := range []netip.Addr{s2, s3} {
		if _, ok := destOf(snap, pure); ok {
			t.Errorf("pure sender %v must not appear on the dest axis", pure)
		}
	}
	// The target also sent, so it IS on the source axis: the two axes are two
	// entities, not one shared entry.
	if _, ok := featureOf(snap, target); !ok {
		t.Error("target sent 200 bytes, so it must also appear on the source axis")
	}
}

// VALIDATES: child-5 AC-2 -- a destination port that many sources send to becomes
// its own entity, carrying the source spread, the amplification ratio, and whether
// the port number is well-known.
// PREVENTS: fan-out on the port axis counting destinations instead of sources, an
// ephemeral SOURCE port becoming an entity, and a well-known port being called rare.
func TestFeaturePortEntry(t *testing.T) {
	a := newAggregator()
	t0 := time.Now()
	d := netip.MustParseAddr("203.0.113.60")
	s1 := netip.MustParseAddr("198.51.100.11")
	s2 := netip.MustParseAddr("198.51.100.12")

	// Two sources sweep udp/31337 from ephemeral source ports; the service answers
	// s1 with 4000 bytes FROM 31337 (an amplification reply).
	a.ingest(flowObsPorts(s1, d, 40001, 31337, 17, 1000, t0))
	a.ingest(flowObsPorts(s2, d, 40002, 31337, 17, 1000, t0))
	a.ingest(flowObsPorts(d, s1, 31337, 40001, 17, 4000, t0))
	// A well-known port for contrast.
	a.ingest(flowObsPorts(s1, d, 40003, 443, 6, 500, t0))

	snap := a.snapshot(t0.Add(time.Second))
	rare, ok := portOf(snap, PortKey{Port: 31337, Proto: 17})
	if !ok {
		t.Fatalf("udp/31337 not on the port axis: %+v", snap.Ports)
	}
	if rare.FanOut != 2 {
		t.Errorf("port fan-out = %d, want 2 (distinct sources)", rare.FanOut)
	}
	if rare.SrcEntropy <= 0 {
		t.Errorf("port source entropy = %v, want > 0 (two sources)", rare.SrcEntropy)
	}
	// out=4000 from the port, in=2000 to it -> 2.0 amplification.
	if rare.OutInRatio != 2 {
		t.Errorf("port ratio = %v, want 2 (bytes from the port over bytes to it)", rare.OutInRatio)
	}
	if !rare.RarePort {
		t.Error("udp/31337 must report rare-port")
	}
	if !rare.NewPort {
		t.Error("first-sighting port must report new-port")
	}

	common, ok := portOf(snap, PortKey{Port: 443, Proto: 6})
	if !ok {
		t.Fatal("tcp/443 not on the port axis")
	}
	if common.RarePort {
		t.Error("tcp/443 is well-known and must not report rare-port")
	}
	if common.FanOut != 1 {
		t.Errorf("tcp/443 fan-out = %d, want 1", common.FanOut)
	}
	// An ephemeral SOURCE port must never become an entity of its own: that is what
	// keeps the port map bounded by service ports rather than by client sockets.
	if _, ok := portOf(snap, PortKey{Port: 40001, Proto: 17}); ok {
		t.Error("ephemeral source port 40001 must not be a port entity")
	}
}

// VALIDATES: child-5 AC-2 -- which side of a flow becomes a port entity, including
// the equal-port case (a service answering from its own port, DNS/NTP style) and the
// omitted-source-port case.
// PREVENTS: the port axis being silently empty for an exporter that leaves the
// source port at 0, and a client socket being tracked as a service.
func TestIsServicePort(t *testing.T) {
	cases := []struct {
		name             string
		srcPort, dstPort uint16
		want             bool
	}{
		{"client to service", 40001, 443, true},
		{"service reply to client", 443, 40001, false},
		{"equal ports (dns/ntp)", 53, 53, true},
		{"source port omitted", 0, 443, true},
		{"both omitted (icmp)", 0, 0, true},
		{"highest port as service, source omitted", 0, 65535, true},
		{"spoofed low source port hides the swept port", 22, 31337, false},
	}
	for _, c := range cases {
		if got := isServicePort(c.srcPort, c.dstPort); got != c.want {
			t.Errorf("%s: isServicePort(%d, %d) = %v, want %v", c.name, c.srcPort, c.dstPort, got, c.want)
		}
	}
}

// VALIDATES: child-5 AC-3 and the Boundary Tests rows -- each axis is capped by its
// OWN ceiling and evicts its own idle entities, so churn on one axis cannot starve
// another. Also the extreme port/proto values.
// PREVENTS: a shared cap letting a source flood evict the victim's dest baseline,
// and unbounded growth under a port sweep.
func TestFeatureDestPortCapsAndEviction(t *testing.T) {
	a := newAggregator()
	t0 := time.Now()
	src := netip.MustParseAddr("198.51.100.99")

	// Sweep more destinations and more ports than either ceiling allows: the
	// destination count passes maxTrackedDest and the distinct-port count passes
	// maxTrackedPort, so both ceilings are genuinely reached rather than assumed.
	for i := range maxTrackedDest + 50 {
		dst := netip.AddrFrom4([4]byte{10, byte(i >> 16), byte(i >> 8), byte(i)})
		a.ingest(flowObs(src, dst, uint16(1024+i%(2*maxTrackedPort)), 100, t0))
	}
	a.mu.Lock()
	nDest, nPort := len(a.dests), len(a.ports)
	a.mu.Unlock()
	if nDest > maxTrackedDest {
		t.Errorf("dest map = %d entries, want <= %d", nDest, maxTrackedDest)
	}
	if nPort > maxTrackedPort {
		t.Errorf("port map = %d entries, want <= %d", nPort, maxTrackedPort)
	}

	// The extreme port and protocol values are valid keys, not rejected input. The
	// source port is 0 here, the case of an exporter that omits it.
	b := newAggregator()
	d := netip.MustParseAddr("203.0.113.70")
	b.ingest(flowObsPorts(src, d, 0, 65535, 255, 100, t0))
	snap := b.snapshot(t0.Add(time.Second))
	if _, ok := portOf(snap, PortKey{Port: 65535, Proto: 255}); !ok {
		t.Errorf("port 65535 proto 255 not tracked: %+v", snap.Ports)
	}

	// Idle eviction runs per axis.
	for i := 1; i <= evictIdleTicks+2; i++ {
		b.snapshot(t0.Add(time.Duration(i+1) * time.Second))
	}
	b.mu.Lock()
	leftDest, leftPort := len(b.dests), len(b.ports)
	b.mu.Unlock()
	if leftDest != 0 {
		t.Errorf("idle dest entities not evicted: %d left", leftDest)
	}
	if leftPort != 0 {
		t.Errorf("idle port entities not evicted: %d left", leftPort)
	}
}

func TestFeatureFanOutRatioEntropy(t *testing.T) {
	a := newAggregator()
	t0 := time.Now()
	s := netip.MustParseAddr("198.51.100.5")
	d1 := netip.MustParseAddr("203.0.113.1")
	d2 := netip.MustParseAddr("203.0.113.2")
	d3 := netip.MustParseAddr("203.0.113.3")
	x := netip.MustParseAddr("192.0.2.9")

	a.ingest(flowObs(s, d1, 443, 1000, t0))
	a.ingest(flowObs(s, d2, 443, 1000, t0))
	a.ingest(flowObs(s, d3, 80, 500, t0))
	a.ingest(flowObs(x, s, 22, 200, t0)) // s receives 200 inbound

	snap := a.snapshot(t0.Add(time.Second))
	fe, ok := featureOf(snap, s)
	if !ok {
		t.Fatalf("source %v not in snapshot", s)
	}
	if fe.FanOut != 3 {
		t.Errorf("fan-out = %d, want 3 (distinct dests)", fe.FanOut)
	}
	// out=2500, in=200 -> exfil ratio 12.5 (NOT the inverse 0.08)
	if fe.OutInRatio != 12.5 {
		t.Errorf("out/in ratio = %v, want 12.5", fe.OutInRatio)
	}
	if fe.PortEntropy <= 0 {
		t.Errorf("port entropy = %v, want > 0 (ports 443 + 80)", fe.PortEntropy)
	}
	// A pure destination (only ever received) must NOT appear: the features are
	// source-behavior signals, so emitting it would be all-zero noise under a
	// "source" heading. d1 only ever appeared as a flow destination.
	if _, ok := featureOf(snap, d1); ok {
		t.Error("pure-destination d1 must not appear in the source-feature snapshot")
	}
	// The other sender (x sent 200 to s) SHOULD appear.
	if _, ok := featureOf(snap, x); !ok {
		t.Error("source x (sent 200) should appear in the snapshot")
	}
}

func TestFeatureNewPeerRarePort(t *testing.T) {
	t0 := time.Now()
	s := netip.MustParseAddr("198.51.100.7")
	d := netip.MustParseAddr("203.0.113.9")

	a := newAggregator()
	a.ingest(flowObs(s, d, 31337, 1000, t0)) // uncommon port
	fe, ok := featureOf(a.snapshot(t0.Add(time.Second)), s)
	if !ok {
		t.Fatal("source not present")
	}
	if !fe.NewPeer {
		t.Error("expected new-peer=true on first sighting")
	}
	if !fe.RarePort {
		t.Error("expected rare-port=true for dominant port 31337")
	}

	a2 := newAggregator()
	a2.ingest(flowObs(s, d, 443, 1000, t0)) // common port
	fe2, _ := featureOf(a2.snapshot(t0.Add(time.Second)), s)
	if fe2.RarePort {
		t.Error("dominant port 443 must not be rare")
	}
}

func TestFeatureBeaconingCoarse(t *testing.T) {
	a := newAggregator()
	t0 := time.Now()
	s := netip.MustParseAddr("198.51.100.11")
	d := netip.MustParseAddr("203.0.113.11")

	var snap *Snapshot
	for tick := range 41 {
		if tick%5 == 0 { // active every 5 seconds
			a.ingest(flowObs(s, d, 443, 1000, t0))
		}
		snap = a.snapshot(t0.Add(time.Duration(tick+1) * time.Second))
	}
	fe, ok := featureOf(snap, s) // tick 40 is active
	if !ok {
		t.Fatal("beaconing source not present on its active tick")
	}
	if fe.Beaconing < 0.5 {
		t.Errorf("beaconing = %v, want >= 0.5 for a regular 5s beacon", fe.Beaconing)
	}
}

func TestFeatureCapsAndEviction(t *testing.T) {
	a := newAggregator()
	t0 := time.Now()
	s := netip.MustParseAddr("192.0.2.50")
	d := netip.MustParseAddr("192.0.2.51")

	a.ingest(flowObs(s, d, 443, 500, t0))
	for i := 1; i <= evictIdleTicks+2; i++ {
		a.snapshot(t0.Add(time.Duration(i) * time.Second))
	}
	a.mu.Lock()
	_, present := a.sources[s]
	a.mu.Unlock()
	if present {
		t.Fatalf("idle source not evicted after %d idle ticks", evictIdleTicks)
	}
}
