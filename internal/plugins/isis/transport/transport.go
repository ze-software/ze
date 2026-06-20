// Design: plan/spec-isis-3-l2-transport.md -- raw L2 transport orchestrator
// Related: frame.go -- 802.3 + LLC frame build/parse
// Related: multicast.go -- ISO multicast MAC selection by level
//
// The transport is the byte pipe between the raw socket and the IS-IS engine.
// It owns a per-interface circuit registry, opens a circuit on `interface/up`
// and closes it on `interface/down`, sends final engine PDUs to the level
// multicast group (adding ONLY 802.3+LLC framing, never padding), and delivers
// received PDUs to the engine as (ifindex, pdu). The platform raw-socket details
// live behind the Backend interface so a future BSD or VPP backend can drop in;
// v1 ships only the Linux AF_PACKET backend.
//
// Concurrency: a circuit's RX is driven by the backend's per-circuit receive
// channel; the transport runs one goroutine per open circuit to fan those into
// a single delivery channel for the engine's PDU dispatcher (isis-4 server.go).
// The transport holds NO protocol switch -- it never inspects the PDU type.

package transport

import (
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	ifaceevents "codeberg.org/thomas-mangin/ze/internal/component/iface/events"
	"codeberg.org/thomas-mangin/ze/internal/core/events"
	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// ifaceEventQueueDepth bounds the worker queue of iface up/down events. It is
// generously sized so a burst of link flaps does not overflow it and silently
// drop an `interface/up` (which would strand a circuit closed); the periodic
// rescan (rescanInterval) is the backstop if it ever does overflow.
const ifaceEventQueueDepth = 256

// rescanInterval is how often the fallback rescan re-attempts opening circuits
// for enabled-but-closed interfaces, so a dropped `interface/up` event recovers
// without operator action. Short enough that recovery is timely, long enough
// that the periodic socket-open attempts are negligible.
const rescanInterval = 30 * time.Second

var loggerPtr atomic.Pointer[slog.Logger]

func init() { loggerPtr.Store(slogutil.DiscardLogger()) }

func logger() *slog.Logger { return loggerPtr.Load() }

// SetLogger configures the transport logger. Called by the IS-IS component
// (isis-4) at startup. Defaults to a discard logger so unit tests are quiet.
func SetLogger(l *slog.Logger) {
	if l != nil {
		loggerPtr.Store(l)
	}
}

// RawFrame is a received frame surfaced by a backend's receive channel. PDU is
// the LLC-stripped IS-IS PDU as a copy owned by the receiver (the backend copies
// out of its shared receive buffer before queueing, so the engine may retain it).
type RawFrame struct {
	IfIndex int
	DstMAC  [MACLen]byte
	SrcMAC  [MACLen]byte
	PDU     []byte
}

// CircuitHandle is one open per-interface raw-socket circuit. Implementations
// are platform-specific (AF_PACKET on Linux); tests substitute a fake.
type CircuitHandle interface {
	// IfIndex is the kernel interface index, used to dispatch received PDUs.
	IfIndex() int
	// HWAddr is the interface MAC, used as the frame source address.
	HWAddr() [MACLen]byte
	// MTU is the interface MTU (ioctl), exposed so the engine can size the
	// Padding TLV. The transport itself never pads.
	MTU() int
	// Send transmits a final, already-padded, already-signed PDU framed with
	// 802.3+LLC to dst. The PDU bytes are sent verbatim.
	Send(dst, src [MACLen]byte, pdu []byte) error
	// Recv returns the channel of received frames; it closes on Close.
	Recv() <-chan RawFrame
	// Close releases the socket and stops the backend receive loop.
	Close() error
}

// Backend opens per-interface circuits. The Linux implementation opens an
// AF_PACKET/SOCK_RAW socket per circuit; the non-Linux stub returns an
// unsupported error.
type Backend interface {
	// OpenCircuit opens a raw L2 circuit on the named interface and starts its
	// receive loop. It resolves ifindex / hwaddr / MTU via ioctl.
	OpenCircuit(name string) (CircuitHandle, error)
}

// circuit is the transport's per-interface bookkeeping around a CircuitHandle.
type circuit struct {
	name   string
	level  Level
	handle CircuitHandle
	stop   chan struct{}
}

// Transport is the IS-IS raw L2 transport orchestrator.
type Transport struct {
	backend Backend

	mu       sync.Mutex
	enabled  map[string]Level    // configured circuits and their level(s)
	circuits map[string]*circuit // open circuits keyed by interface name

	deliver chan RawFrame

	onDown     func(ifindex int, name string)
	onMismatch func(name string, localMTU, neighborMTU int)

	// teardown holds cleanup funcs (EventBus unsubscribe + worker-channel close)
	// registered by SubscribeIfaceEvents. Close runs them before waiting on wg so
	// the subscription worker goroutine exits and wg.Wait does not deadlock.
	teardown []func()

	metrics *transportMetrics
	wg      sync.WaitGroup
}

// New constructs a Transport over the given backend. The backend is the only
// platform-specific dependency; pass NewBackend() in production or a fake in
// tests.
func New(backend Backend) *Transport {
	return &Transport{
		backend:  backend,
		enabled:  make(map[string]Level),
		circuits: make(map[string]*circuit),
		deliver:  make(chan RawFrame, 256),
		metrics:  nopTransportMetrics(),
	}
}

// SetMetrics wires the Prometheus registry. Called by the IS-IS component
// (isis-4) via Registration.ConfigureMetrics. This spec OWNS and registers the
// transport series from the umbrella Metrics table.
func (t *Transport) SetMetrics(reg metrics.Registry) {
	if reg == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.metrics = newTransportMetrics(reg)
}

// EnableInterface marks an interface as IS-IS-enabled at the given level(s). A
// circuit is only opened on `interface/up` for an enabled interface. Pass
// Level1 / Level2 for a single level; the engine selects per-level sends, and
// SendPDUBothLevels covers an L1L2 circuit.
func (t *Transport) EnableInterface(name string, level Level) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.enabled[name] = level
}

// DisableInterface removes an interface from the enabled set and closes any open
// circuit on it.
func (t *Transport) DisableInterface(name string) {
	t.mu.Lock()
	delete(t.enabled, name)
	t.mu.Unlock()
	if err := t.HandleLinkDown(name); err != nil {
		logger().Warn("isis/transport: close on disable", "interface", name, "err", err)
	}
}

// OnCircuitDown registers the callback invoked when a circuit closes (link down
// or disable) so the engine can tear down adjacencies on that circuit.
func (t *Transport) OnCircuitDown(fn func(ifindex int, name string)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onDown = fn
}

// OnMTUMismatch registers the callback invoked when an observed neighbor frame
// size implies an MTU different from the local interface MTU (ISO/IEC 10589 sec
// 8.2.3). The transport does not act on the mismatch; the engine does.
func (t *Transport) OnMTUMismatch(fn func(name string, localMTU, neighborMTU int)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onMismatch = fn
}

// Receive returns the channel of PDUs delivered to the engine. Each item is a
// (ifindex, pdu) pair after 802.3+LLC stripping. The transport never switches on
// PDU type; the dispatcher (isis-4 server.go) does.
func (t *Transport) Receive() <-chan RawFrame { return t.deliver }

// linkEvent is an iface up/down event queued for the worker. up=false means
// down.
type linkEvent struct {
	name string
	up   bool
}

// ifacePayload is the JSON shape of an iface up/down event (monitor_linux.go
// stateEventPayload / linkEventPayload). Only the name is needed here.
type ifacePayload struct {
	Name string `json:"name"`
}

// SubscribeIfaceEvents wires the iface EventBus so `interface/up` opens a
// circuit and `interface/down` closes it for each IS-IS-enabled interface. The
// EventBus handler MUST NOT block on I/O (pkg/ze/eventbus.go), so it only
// enqueues the interface name; a worker goroutine performs the socket open/close
// (which is I/O). A periodic rescan (rescanInterval) is the backstop: if a burst
// of flaps overflows the bounded queue and an `interface/up` is dropped, the
// rescan re-opens the stranded circuit without operator action. Returns an
// unsubscribe function. Called by the IS-IS component (isis-4) at startup with
// the engine EventBus.
func (t *Transport) SubscribeIfaceEvents(eb ze.EventBus) func() {
	return t.subscribeIfaceEventsWithRescan(eb, rescanInterval)
}

// subscribeIfaceEventsWithRescan is SubscribeIfaceEvents with an explicit rescan
// interval so tests can drive the fallback quickly. A non-positive interval
// disables the periodic rescan (the event path alone drives circuits); the
// production caller passes rescanInterval.
func (t *Transport) subscribeIfaceEventsWithRescan(eb ze.EventBus, interval time.Duration) func() {
	if eb == nil {
		return func() {}
	}
	work := make(chan linkEvent, ifaceEventQueueDepth)
	t.wg.Go(func() {
		for ev := range work {
			var err error
			if ev.up {
				err = t.HandleLinkUp(ev.name)
			} else {
				err = t.HandleLinkDown(ev.name)
			}
			if err != nil {
				logger().Warn("isis/transport: iface event handling", "interface", ev.name, "up", ev.up, "err", err)
			}
		}
	})

	enqueue := func(name string, up bool) {
		if name == "" {
			return
		}
		select {
		case work <- linkEvent{name: name, up: up}:
		default: // worker backed up; the periodic rescan recovers a dropped up-event
			logger().Warn("isis/transport: iface event queue full, dropping", "interface", name, "up", up)
		}
	}

	unUp := eb.Subscribe(ifaceevents.Namespace, ifaceevents.EventUp, events.AsString(func(data string) {
		var p ifacePayload
		if json.Unmarshal([]byte(data), &p) == nil {
			enqueue(p.Name, true)
		}
	}))
	unDown := eb.Subscribe(ifaceevents.Namespace, ifaceevents.EventDown, events.AsString(func(data string) {
		var p ifacePayload
		if json.Unmarshal([]byte(data), &p) == nil {
			enqueue(p.Name, false)
		}
	}))

	// Periodic rescan backstop: re-open any enabled-but-closed circuit so a
	// dropped `interface/up` self-heals. stopRescan is closed by cleanup.
	stopRescan := make(chan struct{})
	if interval > 0 {
		t.wg.Go(func() {
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for {
				select {
				case <-stopRescan:
					return
				case <-ticker.C:
					t.RescanInterfaces()
				}
			}
		})
	}

	var once sync.Once
	cleanup := func() {
		once.Do(func() {
			unUp()
			unDown()
			close(stopRescan)
			close(work)
		})
	}
	t.mu.Lock()
	t.teardown = append(t.teardown, cleanup)
	t.mu.Unlock()
	return cleanup
}

// RescanInterfaces re-attempts opening a circuit for every IS-IS-enabled
// interface that has no open circuit. It is the fallback for a dropped
// `interface/up` event (the bounded event queue overflowed): HandleLinkUp is
// idempotent and a no-op for an interface that is not enabled or already open,
// so a rescan never opens a duplicate or an unconfigured interface. Safe to call
// repeatedly and concurrently with the event worker.
func (t *Transport) RescanInterfaces() {
	t.mu.Lock()
	pending := make([]string, 0, len(t.enabled))
	for name := range t.enabled {
		if _, open := t.circuits[name]; !open {
			pending = append(pending, name)
		}
	}
	t.mu.Unlock()

	for _, name := range pending {
		if err := t.HandleLinkUp(name); err != nil {
			logger().Warn("isis/transport: rescan open circuit", "interface", name, "err", err)
		}
	}
}

// HandleLinkUp opens a circuit on an enabled interface. It is a no-op for an
// interface that is not IS-IS-enabled or already open. Driven by the iface
// EventBus `interface/up` event.
func (t *Transport) HandleLinkUp(name string) error {
	t.mu.Lock()
	level, enabled := t.enabled[name]
	_, open := t.circuits[name]
	t.mu.Unlock()
	if !enabled || open {
		return nil
	}

	handle, err := t.backend.OpenCircuit(name)
	if err != nil {
		return err
	}

	c := &circuit{name: name, level: level, handle: handle, stop: make(chan struct{})}
	t.mu.Lock()
	// Re-check under lock in case a concurrent up raced us.
	if _, dup := t.circuits[name]; dup {
		t.mu.Unlock()
		if cerr := handle.Close(); cerr != nil {
			logger().Warn("isis/transport: close duplicate circuit", "interface", name, "err", cerr)
		}
		return nil
	}
	t.circuits[name] = c
	t.metrics.socketsOpen.Set(float64(len(t.circuits)))
	t.mu.Unlock()

	t.wg.Go(func() { t.rxLoop(c) })
	return nil
}

// HandleLinkDown closes the circuit on the named interface, stops its RX loop,
// and signals teardown. Driven by the iface EventBus `interface/down` event.
func (t *Transport) HandleLinkDown(name string) error {
	t.mu.Lock()
	c, open := t.circuits[name]
	if !open {
		t.mu.Unlock()
		return nil
	}
	delete(t.circuits, name)
	t.metrics.socketsOpen.Set(float64(len(t.circuits)))
	onDown := t.onDown
	t.mu.Unlock()

	close(c.stop)
	ifindex := c.handle.IfIndex()
	err := c.handle.Close()
	if onDown != nil {
		onDown(ifindex, name)
	}
	return err
}

// rxLoop fans a circuit's received frames into the shared delivery channel,
// keyed by source ifindex, until the circuit is stopped. It also feeds the
// neighbor-MTU observer so a padded-Hello frame size surfaces a mismatch.
func (t *Transport) rxLoop(c *circuit) {
	recv := c.handle.Recv()
	for {
		select {
		case <-c.stop:
			return
		case rf, ok := <-recv:
			if !ok {
				return
			}
			t.metrics.framesReceived.With(c.name).Inc()
			// Surface an inferred neighbor MTU for comparison (sec 8.2.3).
			t.ObserveNeighborFrame(c.name, FrameHeaderLen+len(rf.PDU))
			select {
			case t.deliver <- rf:
			case <-c.stop:
				return
			}
		}
	}
}

// Send/PDU errors.
var (
	// ErrCircuitNotOpen is returned when sending on or querying an interface
	// that has no open circuit.
	ErrCircuitNotOpen = errors.New("isis/transport: circuit not open")
	// ErrNoMulticastForLevel is returned when SendPDU is given a level with no
	// multicast group (LevelNone or an invalid level).
	ErrNoMulticastForLevel = errors.New("isis/transport: no multicast group for level")
	// ErrPDUExceedsMTU is returned when the final PDU is larger than the circuit
	// MTU; the engine must size padding to the MTU, so an oversize PDU is a bug.
	ErrPDUExceedsMTU = errors.New("isis/transport: PDU exceeds interface MTU")
)

// SendPDU frames a final engine PDU with 802.3+LLC and sends it to the level
// multicast group on the named circuit. The PDU is already padded and signed by
// the engine; the transport adds ONLY framing and MUST NOT alter the bytes. A
// PDU larger than the interface MTU is rejected (R-5).
func (t *Transport) SendPDU(name string, level Level, pdu []byte) error {
	dst, ok := MulticastMACForLevel(level)
	if !ok {
		return ErrNoMulticastForLevel
	}

	t.mu.Lock()
	c, open := t.circuits[name]
	t.mu.Unlock()
	if !open {
		return ErrCircuitNotOpen
	}

	if len(pdu) > c.handle.MTU() {
		return ErrPDUExceedsMTU
	}

	if err := c.handle.Send(dst, c.handle.HWAddr(), pdu); err != nil {
		t.metrics.framesDropped.With(name, "send-error").Inc()
		return err
	}
	t.metrics.framesSent.With(name).Inc()
	return nil
}

// SendPDUBothLevels sends the PDU to BOTH AllL1ISs and AllL2ISs on the circuit,
// for an L1L2 circuit that must reach receivers in either group.
func (t *Transport) SendPDUBothLevels(name string, pdu []byte) error {
	if err := t.SendPDU(name, Level1, pdu); err != nil {
		return err
	}
	return t.SendPDU(name, Level2, pdu)
}

// CircuitOpen reports whether a circuit is open on the named interface.
func (t *Transport) CircuitOpen(name string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	_, ok := t.circuits[name]
	return ok
}

// OpenCircuitCount returns the number of open circuits (drives the
// ze_isis_sockets_open gauge).
func (t *Transport) OpenCircuitCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.circuits)
}

// InterfaceMTU returns the ioctl MTU of an open circuit so the engine can size
// the Padding TLV. The transport never pads. The boolean is false when no
// circuit is open on the interface.
func (t *Transport) InterfaceMTU(name string) (int, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	c, ok := t.circuits[name]
	if !ok {
		return 0, false
	}
	return c.handle.MTU(), true
}

// CircuitInfo returns the resolved ifindex, source MAC (SNPA), and MTU of an open
// circuit so the engine (isis-5) can build a per-interface adjacency circuit with
// its own identity. The boolean is false when no circuit is open on the named
// interface. The MAC is the frame source address the circuit advertises and the
// LAN three-way echo target.
func (t *Transport) CircuitInfo(name string) (ifindex int, hwaddr [MACLen]byte, mtu int, ok bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	c, open := t.circuits[name]
	if !open {
		return 0, [MACLen]byte{}, 0, false
	}
	return c.handle.IfIndex(), c.handle.HWAddr(), c.handle.MTU(), true
}

// CircuitNameByIfIndex returns the interface name of the open circuit with the
// given ifindex, so the engine can route a received PDU (delivered keyed by
// ifindex) to the matching circuit. The boolean is false when no open circuit
// has that ifindex.
func (t *Transport) CircuitNameByIfIndex(ifindex int) (string, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for name, c := range t.circuits {
		if c.handle.IfIndex() == ifindex {
			return name, true
		}
	}
	return "", false
}

// ObserveNeighborFrame records the size of a received (padded-Hello) frame and,
// if the inferred neighbor MTU differs from the local interface MTU, invokes the
// MTU-mismatch callback. ISO/IEC 10589 sec 8.2.3: a router that receives a
// padded Hello can infer the sender's MTU from the frame size and detect a
// mismatch. The transport only surfaces the mismatch; the engine acts on it.
func (t *Transport) ObserveNeighborFrame(name string, frameSize int) {
	neighborMTU := InferNeighborMTU(frameSize)
	if neighborMTU <= 0 {
		return
	}
	t.mu.Lock()
	c, ok := t.circuits[name]
	onMismatch := t.onMismatch
	t.mu.Unlock()
	if !ok || onMismatch == nil {
		return
	}
	localMTU := c.handle.MTU()
	if neighborMTU != localMTU {
		onMismatch(name, localMTU, neighborMTU)
	}
}

// InferNeighborMTU maps a received frame size to the neighbor's inferred MTU.
// The 802.3 frame is FrameHeaderLen (dst+src+length+LLC) plus the PDU; the
// neighbor MTU is the LLC+PDU payload it padded to fill the link, i.e. the frame
// size minus the Ethernet addressing/length overhead (dst+src+length). Returns a
// non-positive value for a frame shorter than the header (caller: treat as
// unknown). ISO/IEC 10589 sec 8.2.3.
func InferNeighborMTU(frameSize int) int {
	// A valid frame carries at least the full header (dst+src+length+LLC); a
	// shorter capture cannot imply a meaningful MTU.
	if frameSize < FrameHeaderLen {
		return 0
	}
	// MTU is the L2 payload: LLC + PDU = frameSize - (dst+src+length field).
	overhead := 2*MACLen + LengthFieldLen
	return frameSize - overhead
}

// Close tears down EventBus subscriptions, stops all circuits, and waits for the
// RX and worker goroutines to exit. Used on component shutdown. Teardown runs
// before wg.Wait so the subscription worker goroutine (which ranges over the
// work channel) exits and the wait does not deadlock.
func (t *Transport) Close() {
	t.mu.Lock()
	teardown := t.teardown
	t.teardown = nil
	names := make([]string, 0, len(t.circuits))
	for name := range t.circuits {
		names = append(names, name)
	}
	t.mu.Unlock()

	for _, fn := range teardown {
		fn()
	}
	for _, name := range names {
		if err := t.HandleLinkDown(name); err != nil {
			logger().Warn("isis/transport: close circuit", "interface", name, "err", err)
		}
	}
	t.wg.Wait()
}
