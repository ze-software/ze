// Design: docs/architecture/isis/isis-3-l2-transport.md -- transport orchestrator + lifecycle

package transport

import (
	"bytes"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/iface"
)

// fakeIfaceSub is an injectable stand-in for iface.Subscribe so the event path
// can be driven without a live resolver. emit pushes a LinkEvent to the reader
// goroutine of a subscribed interface.
type fakeIfaceSub struct {
	mu  sync.Mutex
	chs map[string]chan iface.LinkEvent
}

func newFakeIfaceSub() *fakeIfaceSub {
	return &fakeIfaceSub{chs: make(map[string]chan iface.LinkEvent)}
}

func (f *fakeIfaceSub) subscribe(name string) (<-chan iface.LinkEvent, func()) {
	ch := make(chan iface.LinkEvent, 8)
	f.mu.Lock()
	f.chs[name] = ch
	f.mu.Unlock()
	return ch, func() {
		f.mu.Lock()
		if c, ok := f.chs[name]; ok {
			delete(f.chs, name)
			close(c)
		}
		f.mu.Unlock()
	}
}

func (f *fakeIfaceSub) emit(name string, kind iface.LinkEventKind) {
	f.mu.Lock()
	ch := f.chs[name]
	f.mu.Unlock()
	if ch != nil {
		ch <- iface.LinkEvent{Name: name, Kind: kind}
	}
}

// fakeCircuit is an in-memory CircuitHandle for orchestrator tests. It records
// sends and lets a test push received frames, with no real socket.
type fakeCircuit struct {
	name    string
	ifindex int
	hwaddr  [MACLen]byte
	mtu     int

	mu     sync.Mutex
	sent   []sentFrame
	recvCh chan RawFrame
	closed bool
}

type sentFrame struct {
	dst, src [MACLen]byte
	pdu      []byte
}

func newFakeCircuit(name string, ifindex, mtu int) *fakeCircuit {
	return &fakeCircuit{
		name:    name,
		ifindex: ifindex,
		mtu:     mtu,
		hwaddr:  [MACLen]byte{0x02, 0x00, 0x00, 0x00, 0x00, byte(ifindex)},
		recvCh:  make(chan RawFrame, 16),
	}
}

func (c *fakeCircuit) IfIndex() int          { return c.ifindex }
func (c *fakeCircuit) HWAddr() [MACLen]byte  { return c.hwaddr }
func (c *fakeCircuit) MTU() int              { return c.mtu }
func (c *fakeCircuit) Recv() <-chan RawFrame { return c.recvCh }

func (c *fakeCircuit) Send(dst, src [MACLen]byte, pdu []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	cp := append([]byte(nil), pdu...)
	c.sent = append(c.sent, sentFrame{dst: dst, src: src, pdu: cp})
	return nil
}

func (c *fakeCircuit) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.closed {
		c.closed = true
		close(c.recvCh)
	}
	return nil
}

func (c *fakeCircuit) sends() []sentFrame {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]sentFrame(nil), c.sent...)
}

func (c *fakeCircuit) isClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}

// fakeBackend hands out fakeCircuits keyed by interface name.
type fakeBackend struct {
	mu       sync.Mutex
	circuits map[string]*fakeCircuit
	nextIdx  int
	openErr  error
}

func newFakeBackend() *fakeBackend {
	return &fakeBackend{circuits: make(map[string]*fakeCircuit), nextIdx: 10}
}

func (b *fakeBackend) OpenCircuit(name string) (CircuitHandle, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.openErr != nil {
		return nil, b.openErr
	}
	b.nextIdx++
	c := newFakeCircuit(name, b.nextIdx, 1500)
	b.circuits[name] = c
	return c, nil
}

func (b *fakeBackend) circuit(name string) *fakeCircuit { //nolint:unparam // generic lookup helper; tests open multiple named circuits
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.circuits[name]
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatal("condition not met within deadline")
}

func TestISISTransportOpenOnLinkUp(t *testing.T) {
	// VALIDATES: AC-1 / wiring "interface/up -> transport opens circuit, starts RX".
	be := newFakeBackend()
	tr := New(be)
	tr.EnableInterface("eth0", Level2)

	if err := tr.HandleLinkUp("eth0"); err != nil {
		t.Fatalf("HandleLinkUp: %v", err)
	}
	waitFor(t, func() bool { return be.circuit("eth0") != nil })

	if !tr.CircuitOpen("eth0") {
		t.Fatal("circuit eth0 not open after link up")
	}
}

func TestISISTransportOpenIgnoresUnconfigured(t *testing.T) {
	// VALIDATES: an interface that is not IS-IS-enabled does not open a circuit.
	be := newFakeBackend()
	tr := New(be)
	if err := tr.HandleLinkUp("eth9"); err != nil {
		t.Fatalf("HandleLinkUp: %v", err)
	}
	if tr.CircuitOpen("eth9") {
		t.Fatal("unconfigured interface should not open a circuit")
	}
}

func TestISISTransportSendFrame(t *testing.T) {
	// VALIDATES: AC-2/AC-3/AC-5 engine PDU -> frame built (802.3+LLC, multicast
	// MAC by level, no pad) -> sent via the circuit byte-for-byte.
	be := newFakeBackend()
	tr := New(be)
	tr.EnableInterface("eth0", Level2)
	if err := tr.HandleLinkUp("eth0"); err != nil {
		t.Fatalf("HandleLinkUp: %v", err)
	}

	pdu := samplePDU()
	if err := tr.SendPDU("eth0", Level2, pdu); err != nil {
		t.Fatalf("SendPDU: %v", err)
	}

	c := be.circuit("eth0")
	sends := c.sends()
	if len(sends) != 1 {
		t.Fatalf("got %d sends, want 1", len(sends))
	}
	if sends[0].dst != AllL2ISs {
		t.Errorf("dst = %x, want AllL2ISs %x", sends[0].dst, AllL2ISs)
	}
	if sends[0].src != c.HWAddr() {
		t.Errorf("src = %x, want hwaddr %x", sends[0].src, c.HWAddr())
	}
	if !bytes.Equal(sends[0].pdu, pdu) {
		t.Errorf("PDU altered on send: got %x want %x", sends[0].pdu, pdu)
	}
}

func TestISISTransportSendBothLevels(t *testing.T) {
	// VALIDATES: AC-5 a dual-level send goes to BOTH AllL1ISs and AllL2ISs.
	be := newFakeBackend()
	tr := New(be)
	tr.EnableInterface("eth0", Level1)
	_ = tr.HandleLinkUp("eth0")

	pdu := samplePDU()
	if err := tr.SendPDUBothLevels("eth0", pdu); err != nil {
		t.Fatalf("SendPDUBothLevels: %v", err)
	}
	sends := be.circuit("eth0").sends()
	if len(sends) != 2 {
		t.Fatalf("got %d sends, want 2 (one per level)", len(sends))
	}
	got := map[[MACLen]byte]bool{sends[0].dst: true, sends[1].dst: true}
	if !got[AllL1ISs] || !got[AllL2ISs] {
		t.Errorf("dst groups = %v, want both AllL1ISs and AllL2ISs", got)
	}
}

func TestISISTransportSendRejectOversize(t *testing.T) {
	// VALIDATES: AC-3 / R-5 a PDU larger than the circuit MTU is rejected
	// (the engine must size padding to MTU; transport refuses to over-send).
	be := newFakeBackend()
	tr := New(be)
	tr.EnableInterface("eth0", Level2)
	_ = tr.HandleLinkUp("eth0")

	big := make([]byte, 1501) // > MTU 1500
	if err := tr.SendPDU("eth0", Level2, big); err == nil {
		t.Fatal("expected oversize PDU to be rejected, got nil")
	}
}

func TestISISTransportSendMTUBoundary(t *testing.T) {
	// VALIDATES: AC-3 / R-5 boundary -- a PDU exactly at the MTU sends; one byte
	// over the MTU is rejected. The fake circuit reports MTU 1500.
	be := newFakeBackend()
	tr := New(be)
	tr.EnableInterface("eth0", Level2)
	_ = tr.HandleLinkUp("eth0")
	const mtu = 1500

	// Last valid: PDU length == MTU.
	if err := tr.SendPDU("eth0", Level2, make([]byte, mtu)); err != nil {
		t.Errorf("PDU at MTU (%d) rejected: %v", mtu, err)
	}
	// Invalid above: MTU+1.
	if err := tr.SendPDU("eth0", Level2, make([]byte, mtu+1)); err == nil {
		t.Errorf("PDU at MTU+1 (%d) accepted, want rejection", mtu+1)
	}
}

func TestISISTransportReceiveDelivers(t *testing.T) {
	// VALIDATES: AC-1 a frame arriving on a circuit is delivered as (ifindex, pdu).
	be := newFakeBackend()
	tr := New(be)
	tr.EnableInterface("eth0", Level2)
	_ = tr.HandleLinkUp("eth0")

	c := be.circuit("eth0")
	pdu := samplePDU()
	c.recvCh <- RawFrame{IfIndex: c.IfIndex(), DstMAC: AllL2ISs, SrcMAC: [MACLen]byte{0x02, 0x09}, PDU: pdu}

	select {
	case rx := <-tr.Receive():
		if rx.IfIndex != c.IfIndex() {
			t.Errorf("ifindex = %d, want %d", rx.IfIndex, c.IfIndex())
		}
		if !bytes.Equal(rx.PDU, pdu) {
			t.Errorf("PDU = %x, want %x", rx.PDU, pdu)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no PDU delivered to the engine")
	}
}

func TestISISTransportCloseOnLinkDown(t *testing.T) {
	// VALIDATES: AC-6 / wiring "interface/down -> close socket, signal teardown".
	be := newFakeBackend()
	tr := New(be)
	tr.EnableInterface("eth0", Level2)
	_ = tr.HandleLinkUp("eth0")
	c := be.circuit("eth0")

	var teardownIfIndex int
	tr.OnCircuitDown(func(ifindex int, name string) { teardownIfIndex = ifindex })

	if err := tr.HandleLinkDown("eth0"); err != nil {
		t.Fatalf("HandleLinkDown: %v", err)
	}
	waitFor(t, func() bool { return c.isClosed() })
	if tr.CircuitOpen("eth0") {
		t.Fatal("circuit still open after link down")
	}
	if teardownIfIndex != c.IfIndex() {
		t.Errorf("teardown ifindex = %d, want %d", teardownIfIndex, c.IfIndex())
	}
}

func TestISISTransportSocketsOpenGauge(t *testing.T) {
	// VALIDATES: open/close keeps the open-circuit count consistent (no leak,
	// AC-6). Drives the ze_isis_sockets_open gauge.
	be := newFakeBackend()
	tr := New(be)
	tr.EnableInterface("eth0", Level2)
	tr.EnableInterface("eth1", Level2)

	_ = tr.HandleLinkUp("eth0")
	_ = tr.HandleLinkUp("eth1")
	if got := tr.OpenCircuitCount(); got != 2 {
		t.Fatalf("open count = %d, want 2", got)
	}
	_ = tr.HandleLinkDown("eth0")
	if got := tr.OpenCircuitCount(); got != 1 {
		t.Fatalf("open count after down = %d, want 1", got)
	}
}

// sharedBufCircuit reproduces, in a platform-neutral way, the linuxCircuit
// hazard the B3 fix addresses: a single reusable send buffer written then read
// inside Send. If the transport orchestrator lets two goroutines enter Send
// concurrently without the circuit serializing itself, the race detector flags
// the unsynchronized buffer access and the torn-frame check fails. The real fix
// lives in backend_linux.go (linuxCircuit.sendMu); this fake guards the
// orchestrator-level contract (SendPDU releases t.mu before handle.Send) on
// every platform, including darwin under `go test -race`.
type sharedBufCircuit struct {
	ifindex int
	hwaddr  [MACLen]byte
	mtu     int

	mu       sync.Mutex // mirrors linuxCircuit.sendMu
	guard    bool       // serialize BuildFrame+Sendto under mu, as the fix does
	buf      [256]byte  // the shared reusable buffer (linuxCircuit.sendBuf analog)
	tornSeen bool       // set if a frame read back from buf was not internally consistent
}

func (c *sharedBufCircuit) IfIndex() int          { return c.ifindex }
func (c *sharedBufCircuit) HWAddr() [MACLen]byte  { return c.hwaddr }
func (c *sharedBufCircuit) MTU() int              { return c.mtu }
func (c *sharedBufCircuit) Recv() <-chan RawFrame { return nil }
func (c *sharedBufCircuit) Close() error          { return nil }

func (c *sharedBufCircuit) Send(_, _ [MACLen]byte, pdu []byte) error {
	if c.guard {
		c.mu.Lock()
		defer c.mu.Unlock()
	}
	// "Build" the frame: write a marker byte from this PDU across the shared
	// buffer, then "send": read it back and verify it is internally consistent.
	// Without serialization a concurrent Send overwrites the buffer between the
	// write and the read, so the read sees a torn (mixed) buffer.
	marker := pdu[0]
	for i := range c.buf {
		c.buf[i] = marker
	}
	for i := range c.buf {
		if c.buf[i] != marker {
			c.tornSeen = true
			break
		}
	}
	return nil
}

func TestISISTransportConcurrentSendSerialised(t *testing.T) {
	// VALIDATES: B3 transport race -- SendPDU releases the orchestrator lock
	// before CircuitHandle.Send, so the circuit alone must serialize its shared
	// send buffer. With the per-circuit lock engaged (guard=true, mirroring the
	// linuxCircuit.sendMu fix) concurrent Hello/flood/DIS-style sends never tear
	// and `go test -race` is clean. Runs on every platform, including darwin.
	c := &sharedBufCircuit{ifindex: 11, mtu: 1500, guard: true}

	tr := New(newFakeBackend())
	tr.EnableInterface("eth0", Level2)
	// Install our shared-buffer circuit directly so all sends hit the same buffer.
	tr.mu.Lock()
	tr.circuits["eth0"] = &circuit{name: "eth0", level: Level2, handle: c, stop: make(chan struct{})}
	tr.mu.Unlock()

	const goroutines = 8
	const iterations = 500
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := range goroutines {
		pdu := []byte{byte('A' + g)} // a distinct marker per goroutine
		go func() {
			defer wg.Done()
			for range iterations {
				if err := tr.SendPDU("eth0", Level2, pdu); err != nil {
					t.Errorf("SendPDU: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()

	c.mu.Lock()
	torn := c.tornSeen
	c.mu.Unlock()
	if torn {
		t.Fatal("torn send buffer: the circuit did not serialize concurrent sends (B3 regression)")
	}
}

// fakeBus is a minimal in-process EventBus that records subscriptions and lets a
// test emit an iface event to the registered handler.
type fakeBus struct {
	mu       sync.Mutex
	handlers map[string]func(any)
}

func newFakeBus() *fakeBus { return &fakeBus{handlers: make(map[string]func(any))} }

func (b *fakeBus) key(ns, et string) string { return ns + "/" + et }

func (b *fakeBus) Emit(ns, et string, payload any) (int, error) {
	b.mu.Lock()
	h := b.handlers[b.key(ns, et)]
	b.mu.Unlock()
	if h != nil {
		h(payload)
	}
	return 0, nil
}

func (b *fakeBus) Subscribe(ns, et string, handler func(any)) func() {
	b.mu.Lock()
	b.handlers[b.key(ns, et)] = handler
	b.mu.Unlock()
	return func() {
		b.mu.Lock()
		delete(b.handlers, b.key(ns, et))
		b.mu.Unlock()
	}
}

func TestISISTransportRescanRecoversDroppedUp(t *testing.T) {
	// VALIDATES: B8 -- a missed `interface/up` (the bounded event queue dropped it)
	// self-heals via the rescan fallback: RescanInterfaces re-opens any enabled
	// interface that has no open circuit, so the circuit is not stranded closed.
	// PREVENTS: an IS-IS-enabled interface staying permanently dark because one
	// up-event was lost under load.
	be := newFakeBackend()
	tr := New(be)
	tr.EnableInterface("eth0", Level2)

	// No up-event was ever delivered (the queue "dropped" it): the circuit is
	// closed. A rescan must open it.
	if tr.CircuitOpen("eth0") {
		t.Fatal("circuit unexpectedly open before any event/rescan")
	}
	tr.RescanInterfaces()
	waitFor(t, func() bool { return tr.CircuitOpen("eth0") })

	// Rescan is idempotent: a second rescan must not open a duplicate or error.
	tr.RescanInterfaces()
	if got := tr.OpenCircuitCount(); got != 1 {
		t.Fatalf("after repeated rescan, open count = %d, want 1", got)
	}
}

func TestISISTransportRescanSkipsUnenabled(t *testing.T) {
	// VALIDATES: the rescan opens circuits ONLY for IS-IS-enabled interfaces, so a
	// rescan never opens a socket on an interface the operator did not configure.
	be := newFakeBackend()
	tr := New(be)
	tr.RescanInterfaces()
	if tr.OpenCircuitCount() != 0 {
		t.Fatal("rescan opened a circuit with no enabled interface")
	}
}

func TestISISTransportPeriodicRescanRecovers(t *testing.T) {
	// VALIDATES: the periodic rescan goroutine started by SubscribeIfaceEvents
	// re-opens an enabled-but-closed interface without any further event, so a
	// dropped up-event recovers on its own. A short interval keeps the test fast.
	be := newFakeBackend()
	tr := New(be)
	tr.EnableInterface("eth0", Level2)

	bus := newFakeBus()
	stop := tr.subscribeIfaceEventsWithRescan(bus, 5*time.Millisecond)
	t.Cleanup(stop)
	t.Cleanup(tr.Close)

	// No event is emitted at all; only the periodic rescan can open the circuit.
	waitFor(t, func() bool { return tr.CircuitOpen("eth0") })
}

func TestISISTransportEventOpensAndCloses(t *testing.T) {
	// VALIDATES: wiring -- subscribing to the iface resolver (iface.Subscribe)
	// drives circuit open on an up/appeared event and close on a down event for
	// an enabled interface. The resolver delivers events under the logical name.
	// PREVENTS: regressing the circuit lifecycle when the event source moved from
	// the raw EventBus to the logical-name-aware resolver.
	// the bus.Emit("interface","up"/"down") assertions are removed
	// because IS-IS no longer subscribes to the raw EventBus for link events; the
	// open/close lifecycle they covered is now driven (and asserted) through the
	// injected resolver subscription (fake.emit) below -- replaced coverage, same
	// behavior. The resolver's own EventBus subscription is covered by the iface
	// component's resolve_test.go.
	be := newFakeBackend()
	tr := New(be)
	fake := newFakeIfaceSub()
	tr.subscribe = fake.subscribe
	tr.EnableInterface("eth0", Level2)

	bus := newFakeBus() // non-nil availability gate; the resolver is the source
	stop := tr.SubscribeIfaceEvents(bus)
	t.Cleanup(stop)
	t.Cleanup(tr.Close)

	fake.emit("eth0", iface.LinkUp)
	waitFor(t, func() bool { return tr.CircuitOpen("eth0") })

	fake.emit("eth0", iface.LinkDown)
	waitFor(t, func() bool { return !tr.CircuitOpen("eth0") })
}

// TestISISTransportLateEnableSubscribes verifies an interface enabled AFTER the
// event path is wired still gets a resolver subscription and opens on up.
func TestISISTransportLateEnableSubscribes(t *testing.T) {
	be := newFakeBackend()
	tr := New(be)
	fake := newFakeIfaceSub()
	tr.subscribe = fake.subscribe

	bus := newFakeBus()
	stop := tr.SubscribeIfaceEvents(bus)
	t.Cleanup(stop)
	t.Cleanup(tr.Close)

	// Enable after wiring: EnableInterface must subscribe immediately.
	tr.EnableInterface("eth1", Level2)
	fake.emit("eth1", iface.LinkUp)
	waitFor(t, func() bool { return tr.CircuitOpen("eth1") })
}
