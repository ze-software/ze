// Design: docs/architecture/vrrp/vrrp-first-hop-redundancy.md -- transport orchestrator tests (fake backend)

package transport

import (
	"bytes"
	"net/netip"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/plugins/vrrp/packet"
)

// fakeHandle records sent frames and simulates the backend readLoop goroutine so
// the goroutine-leak test is meaningful.
type fakeHandle struct {
	mu           sync.Mutex
	adverts      [][]byte
	announces    [][]byte
	noLinkLocal  bool
	closed       bool
	stopReadLoop chan struct{}
	readLoopDone chan struct{}
}

func (h *fakeHandle) SendAdvert(frame []byte) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.noLinkLocal {
		return ErrNoLinkLocal
	}
	h.adverts = append(h.adverts, append([]byte(nil), frame...))
	return nil
}

func (h *fakeHandle) SendAnnounce(frame []byte) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.announces = append(h.announces, append([]byte(nil), frame...))
	return nil
}

func (h *fakeHandle) Close() error {
	h.mu.Lock()
	already := h.closed
	h.closed = true
	h.mu.Unlock()
	if !already {
		close(h.stopReadLoop)
		<-h.readLoopDone
	}
	return nil
}

func (h *fakeHandle) lastAdvert() []byte {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.adverts) == 0 {
		return nil
	}
	return h.adverts[len(h.adverts)-1]
}

func (h *fakeHandle) announceCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.announces)
}

// fakeBackend hands out fakeHandles and spawns a simulated readLoop per handle.
type fakeBackend struct {
	mu          sync.Mutex
	handles     []*fakeHandle
	noLinkLocal bool
}

func (b *fakeBackend) OpenInstance(_ InstanceSpec, _ rxSink) (InstanceHandle, error) {
	h := &fakeHandle{
		noLinkLocal:  b.noLinkLocal,
		stopReadLoop: make(chan struct{}),
		readLoopDone: make(chan struct{}),
	}
	go func() { defer close(h.readLoopDone); <-h.stopReadLoop }()
	b.mu.Lock()
	b.handles = append(b.handles, h)
	b.mu.Unlock()
	return h, nil
}

func (b *fakeBackend) last() *fakeHandle {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.handles[len(b.handles)-1]
}

func v4Spec() InstanceSpec {
	return InstanceSpec{
		Family:        packet.V4,
		VRID:          10,
		Parent:        "eth0",
		MacvlanDevice: "vrrp4-1-10",
		VirtualMAC:    packet.VirtualMAC(packet.V4, 10),
	}
}

func v6Spec() InstanceSpec {
	return InstanceSpec{
		Family:        packet.V6,
		VRID:          10,
		Parent:        "eth0",
		MacvlanDevice: "vrrp6-1-10",
		VirtualMAC:    packet.VirtualMAC(packet.V6, 10),
	}
}

func v4Params() AdvertParams {
	return AdvertParams{
		Version:         packet.VersionV3,
		Priority:        100,
		AdverIntervalMS: 1000,
		VIPs:            []netip.Addr{netip.MustParseAddr("192.0.2.1")},
	}
}

func withParentAddrs(t *testing.T, addrs []iface.AddrInfo) {
	t.Helper()
	old := resolveIfaceAddresses
	resolveIfaceAddresses = func(string) ([]iface.AddrInfo, error) { return addrs, nil }
	t.Cleanup(func() { resolveIfaceAddresses = old })
}

func TestTransportOpenInstanceWiring(t *testing.T) {
	// VALIDATES: Wiring -- OpenInstance drives Backend.OpenInstance, registers the
	// instance, and bumps the sockets_open gauge.
	withParentAddrs(t, []iface.AddrInfo{{Address: "192.0.2.251", Family: "ipv4"}})
	fb := &fakeBackend{}
	reg := newRecordingRegistry()
	tr := New(fb)
	tr.SetMetrics(reg)

	key, err := tr.OpenInstance(v4Spec())
	if err != nil {
		t.Fatalf("OpenInstance: %v", err)
	}
	if len(fb.handles) != 1 {
		t.Fatalf("backend opened %d handles, want 1", len(fb.handles))
	}
	if got := atomic.LoadInt64(reg.gauge); got != 1 {
		t.Fatalf("sockets_open gauge = %d, want 1", got)
	}
	if tr.lookup(key) == nil {
		t.Fatal("instance not registered")
	}
	tr.Close()
	if got := atomic.LoadInt64(reg.gauge); got != 0 {
		t.Fatalf("sockets_open gauge after Close = %d, want 0", got)
	}
}

func TestUpdateAdvertReencodes(t *testing.T) {
	// VALIDATES: AC-5 / holo bug 8 -- a priority change between sends makes the
	// next datagram carry the new parameters (no stale pre-encoded advert).
	withParentAddrs(t, []iface.AddrInfo{{Address: "192.0.2.251", Family: "ipv4"}})
	fb := &fakeBackend{}
	tr := New(fb)
	key, err := tr.OpenInstance(v4Spec())
	if err != nil {
		t.Fatalf("OpenInstance: %v", err)
	}
	h := fb.last()

	p := v4Params()
	p.Priority = 100
	if err := tr.UpdateAdvert(key, p); err != nil {
		t.Fatalf("UpdateAdvert: %v", err)
	}
	if err := tr.SendAdvert(key); err != nil {
		t.Fatalf("SendAdvert: %v", err)
	}
	// VRRP priority byte sits after the 20-byte IPv4 header, at payload offset 2.
	if got := h.lastAdvert()[ipv4HeaderLen+2]; got != 100 {
		t.Fatalf("first advert priority = %d, want 100", got)
	}

	p.Priority = 200
	if err := tr.UpdateAdvert(key, p); err != nil {
		t.Fatalf("UpdateAdvert 2: %v", err)
	}
	if err := tr.SendAdvert(key); err != nil {
		t.Fatalf("SendAdvert 2: %v", err)
	}
	if got := h.lastAdvert()[ipv4HeaderLen+2]; got != 200 {
		t.Fatalf("second advert priority = %d, want 200 (stale advert bug)", got)
	}
}

func TestSendAdvertUsesParentPrimaryV4Source(t *testing.T) {
	// RFC requirement: RFC3768-7.2-3 positive -- the transmitted advert's source IP is the parent unit's primary IPv4 address, re-resolved on update and on an address-change event (resolveParentPrimaryV4 transport.go:573).
	// RFC requirement: RFC9568-7.2-3 positive -- the transmitted advert's source IP is the sending interface's primary IPv4 address, re-resolved on update and on an address-change event (resolveParentPrimaryV4 transport.go:573); the IPv6 counterpart is the macvlan's link-local, pinned per send (macvlanLinkLocal backend_linux.go:466).
	// VALIDATES: AC-3 / A-7 -- the IPv4 source is the parent unit's first IPv4
	// address, re-resolved on UpdateAdvert and on an address-change event.
	old := resolveIfaceAddresses
	t.Cleanup(func() { resolveIfaceAddresses = old })
	set := func(a string) {
		resolveIfaceAddresses = func(string) ([]iface.AddrInfo, error) {
			return []iface.AddrInfo{{Address: a, Family: "ipv4"}}, nil
		}
	}

	set("192.0.2.10")
	fb := &fakeBackend{}
	tr := New(fb)
	key, err := tr.OpenInstance(v4Spec())
	if err != nil {
		t.Fatalf("OpenInstance: %v", err)
	}
	h := fb.last()

	if err := tr.UpdateAdvert(key, v4Params()); err != nil {
		t.Fatalf("UpdateAdvert: %v", err)
	}
	if err := tr.SendAdvert(key); err != nil {
		t.Fatalf("SendAdvert: %v", err)
	}
	if src := netip.AddrFrom4([4]byte(h.lastAdvert()[12:16])); src != netip.MustParseAddr("192.0.2.10") {
		t.Fatalf("first source = %v, want 192.0.2.10", src)
	}

	// Re-resolution on UpdateAdvert.
	set("192.0.2.20")
	if err := tr.UpdateAdvert(key, v4Params()); err != nil {
		t.Fatalf("UpdateAdvert 2: %v", err)
	}
	if err := tr.SendAdvert(key); err != nil {
		t.Fatalf("SendAdvert 2: %v", err)
	}
	if src := netip.AddrFrom4([4]byte(h.lastAdvert()[12:16])); src != netip.MustParseAddr("192.0.2.20") {
		t.Fatalf("source after UpdateAdvert = %v, want 192.0.2.20", src)
	}

	// Re-resolution on an address-change event (no param change).
	set("192.0.2.30")
	tr.RefreshParentAddresses(key)
	if err := tr.SendAdvert(key); err != nil {
		t.Fatalf("SendAdvert 3: %v", err)
	}
	if src := netip.AddrFrom4([4]byte(h.lastAdvert()[12:16])); src != netip.MustParseAddr("192.0.2.30") {
		t.Fatalf("source after address event = %v, want 192.0.2.30", src)
	}
}

// TestSendAdvertIPv4HeaderTTLProtoDst asserts the IPv4 header ze builds for a v2
// advertisement carries the RFC-mandated TTL, protocol, and destination.
func TestSendAdvertIPv4HeaderTTLProtoDst(t *testing.T) {
	// RFC requirement: RFC3768-5.2.3-1 positive -- the transmitted IPv4 datagram carries TTL 255 (buildIPv4Header transport.go:562).
	// RFC requirement: RFC3768-7.2-4 positive -- the transmitted datagram carries IP protocol 112 and destination 224.0.0.18 (buildIPv4Header transport.go:563; SendAdvert targets MulticastV4).
	withParentAddrs(t, []iface.AddrInfo{{Address: "192.0.2.10", Family: "ipv4"}})
	fb := &fakeBackend{}
	tr := New(fb)
	key, err := tr.OpenInstance(v4Spec())
	if err != nil {
		t.Fatalf("OpenInstance: %v", err)
	}
	h := fb.last()
	if err := tr.UpdateAdvert(key, AdvertParams{
		Version:         packet.VersionV2,
		Priority:        100,
		AdverIntervalMS: 1000,
		VIPs:            []netip.Addr{netip.MustParseAddr("192.0.2.1")},
	}); err != nil {
		t.Fatalf("UpdateAdvert: %v", err)
	}
	if err := tr.SendAdvert(key); err != nil {
		t.Fatalf("SendAdvert: %v", err)
	}
	frame := h.lastAdvert()
	if len(frame) < ipv4HeaderLen {
		t.Fatalf("frame too short: %d bytes", len(frame))
	}
	if frame[8] != 255 {
		t.Errorf("TTL = %d, want 255", frame[8])
	}
	if frame[9] != packet.ProtoNumber {
		t.Errorf("protocol = %d, want %d", frame[9], packet.ProtoNumber)
	}
	if dst := netip.AddrFrom4([4]byte(frame[16:20])); dst != packet.MulticastV4 {
		t.Errorf("dst = %v, want 224.0.0.18", dst)
	}
}

// TestSendAdvertV3IPv4HeaderTTLProtoDst asserts the IPv4 header ze builds for a
// VRRPv3 advertisement carries the RFC 9568 TTL, protocol number, and
// destination group.
//
// RFC requirement: RFC9568-5.1.1.3-1 positive -- the transmitted IPv4 datagram carries TTL 255 (buildIPv4Header transport.go:562)
// RFC requirement: RFC9568-7.2-4 positive -- the transmitted datagram carries IP protocol 112 and is sent to the VRRP IPv4 multicast group 224.0.0.18 (buildIPv4Header transport.go:563; SendAdvert backend_linux.go:256).
func TestSendAdvertV3IPv4HeaderTTLProtoDst(t *testing.T) {
	withParentAddrs(t, []iface.AddrInfo{{Address: "192.0.2.10", Family: "ipv4"}})
	fb := &fakeBackend{}
	tr := New(fb)
	key, err := tr.OpenInstance(v4Spec())
	if err != nil {
		t.Fatalf("OpenInstance: %v", err)
	}
	h := fb.last()
	if err := tr.UpdateAdvert(key, AdvertParams{
		Version:         packet.VersionV3,
		Priority:        100,
		AdverIntervalMS: 1000,
		VIPs:            []netip.Addr{netip.MustParseAddr("192.0.2.1")},
	}); err != nil {
		t.Fatalf("UpdateAdvert: %v", err)
	}
	if err := tr.SendAdvert(key); err != nil {
		t.Fatalf("SendAdvert: %v", err)
	}
	frame := h.lastAdvert()
	if len(frame) < ipv4HeaderLen {
		t.Fatalf("frame too short: %d bytes", len(frame))
	}
	if frame[8] != 255 {
		t.Errorf("TTL = %d, want 255", frame[8])
	}
	if frame[9] != packet.ProtoNumber {
		t.Errorf("protocol = %d, want %d", frame[9], packet.ProtoNumber)
	}
	if dst := netip.AddrFrom4([4]byte(frame[16:20])); dst != packet.MulticastV4 {
		t.Errorf("dst = %v, want 224.0.0.18", dst)
	}
	if v := frame[ipv4HeaderLen] >> 4; v != packet.VersionV3 {
		t.Errorf("VRRP version = %d, want 3", v)
	}
}

func TestSendAdvertNoLinkLocalSkipsAndCounts(t *testing.T) {
	// VALIDATES: AC-10 -- a v6 send with no macvlan link-local yet is skipped and
	// counted {reason=no-link-local}, returns no upward error, and a retry
	// succeeds once the link-local appears.
	fb := &fakeBackend{noLinkLocal: true}
	tr := New(fb)
	key, err := tr.OpenInstance(v6Spec())
	if err != nil {
		t.Fatalf("OpenInstance: %v", err)
	}
	h := fb.last()
	if err := tr.UpdateAdvert(key, AdvertParams{
		Version:         packet.VersionV3,
		Priority:        100,
		AdverIntervalMS: 1000,
		VIPs:            []netip.Addr{netip.MustParseAddr("fe80::1")},
	}); err != nil {
		t.Fatalf("UpdateAdvert: %v", err)
	}

	if err := tr.SendAdvert(key); err != nil {
		t.Fatalf("SendAdvert returned upward error: %v", err)
	}
	snap, _ := tr.CounterSnapshot(key)
	if snap.PacketErrors[reasonNoLinkLocal] != 1 || snap.AdvertsSent != 0 {
		t.Fatalf("no-link-local not counted: %+v", snap)
	}

	// Link-local appears; retry succeeds.
	h.mu.Lock()
	h.noLinkLocal = false
	h.mu.Unlock()
	if err := tr.SendAdvert(key); err != nil {
		t.Fatalf("SendAdvert after link-local: %v", err)
	}
	snap, _ = tr.CounterSnapshot(key)
	if snap.AdvertsSent != 1 {
		t.Fatalf("retry did not send: %+v", snap)
	}
}

func TestReadLoopDeliversRxMeta(t *testing.T) {
	// VALIDATES: AC-6 -- a received datagram reaches the engine channel with
	// Src/Dst/TTL/Family/IfIndex + payload; TTL 254 passes through unmodified
	// (GTSM discard is the codec's job, fed by this meta).
	ch := make(chan RxItem, 4)
	var mp atomic.Pointer[transportMetrics]
	mp.Store(nopTransportMetrics())
	c := newInstanceCounters(&mp, "eth0", 10, packet.V4)
	sink := rxSink{ch: ch, counters: c}

	item := RxItem{
		Meta: packet.RxMeta{
			TTL:     254,
			Src:     netip.MustParseAddr("192.0.2.9"),
			Dst:     packet.MulticastV4,
			Family:  packet.V4,
			IfIndex: 42,
		},
		Payload: []byte{0x31, 0x0a, 0x64, 0x01},
	}
	if !sink.deliver(item) {
		t.Fatal("deliver dropped an item on an empty channel")
	}
	got := <-ch
	if got.Meta.TTL != 254 || got.Meta.IfIndex != 42 || got.Meta.Family != packet.V4 {
		t.Fatalf("rx meta wrong: %+v", got.Meta)
	}
	if got.Meta.Src != item.Meta.Src || got.Meta.Dst != item.Meta.Dst {
		t.Fatalf("rx src/dst wrong: %+v", got.Meta)
	}
	if !bytes.Equal(got.Payload, item.Payload) {
		t.Fatalf("rx payload = % x, want % x", got.Payload, item.Payload)
	}
	if c.snapshot().AdvertsReceived != 1 {
		t.Fatalf("adverts_received not counted")
	}
}

func TestRxOverflowDropsAndCounts(t *testing.T) {
	// VALIDATES: AC-7 -- a full engine channel drops the datagram without blocking
	// and counts {reason=rx-overflow}.
	ch := make(chan RxItem, 1)
	var mp atomic.Pointer[transportMetrics]
	mp.Store(nopTransportMetrics())
	c := newInstanceCounters(&mp, "eth0", 10, packet.V4)
	sink := rxSink{ch: ch, counters: c}

	item := RxItem{Meta: packet.RxMeta{Family: packet.V4}, Payload: []byte{1}}
	if !sink.deliver(item) {
		t.Fatal("first deliver dropped")
	}
	// Channel now full; second deliver must drop (non-blocking) and count.
	if sink.deliver(item) {
		t.Fatal("deliver did not drop on a full channel")
	}
	if c.snapshot().PacketErrors[reasonRxOverflow] != 1 {
		t.Fatalf("rx-overflow not counted: %+v", c.snapshot().PacketErrors)
	}
}

func TestCloseStopsGoroutines(t *testing.T) {
	// VALIDATES: AC-14 -- Close joins the readLoop (fake) and announcer goroutines
	// with no leak.
	withParentAddrs(t, []iface.AddrInfo{{Address: "192.0.2.251", Family: "ipv4"}})
	base := runtime.NumGoroutine()
	fb := &fakeBackend{}
	tr := New(fb)
	if _, err := tr.OpenInstance(v4Spec()); err != nil {
		t.Fatalf("OpenInstance: %v", err)
	}
	tr.Close()

	deadline := time.After(2 * time.Second)
	for runtime.NumGoroutine() > base {
		select {
		case <-deadline:
			t.Fatalf("goroutine leak: %d > baseline %d", runtime.NumGoroutine(), base)
		default:
			time.Sleep(time.Millisecond)
		}
	}
}
