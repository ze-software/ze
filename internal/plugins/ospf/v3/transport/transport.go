// Design: docs/architecture/ospf/ospfv3-3-ipv6-transport.md -- OSPFv3 raw IPv6 transport orchestrator
// RFC: rfc/short/rfc5340.md (§2.9 transport, §A.3.1 checksum, §4.2.1 Instance ID demux)

package transport

import (
	"errors"
	"log/slog"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/v3/types"
	"github.com/ze-software/ze/internal/plugins/ospf/wire"
	"github.com/ze-software/ze/pkg/ze"
)

const (
	rescanInterval = 30 * time.Second
	// One raw IPv6 PacketConn per interface (the OSPFv2 sibling used two FDs for a
	// separate RX/TX split; the ipv6.PacketConn serves both directions).
	socketsPerInterface = 1

	dropShort            = "short"
	dropInstanceMismatch = "instance-mismatch"
	dropSendError        = "send-error"
)

var loggerPtr atomic.Pointer[slog.Logger]

func init() { loggerPtr.Store(slogutil.DiscardLogger()) }

func logger() *slog.Logger { return loggerPtr.Load() }

// SetLogger installs the package logger (production wires the daemon logger).
func SetLogger(l *slog.Logger) {
	if l != nil {
		loggerPtr.Store(l)
	}
}

// RawPacket is one received OSPFv3 datagram. Unlike the OSPFv2 sibling there is no
// IP header to strip: an IPv6 raw socket delivers the upper-layer payload, and
// Src/Dst/HopLimit come from the ancillary control message. Payload is owned by
// the receiver (the backend copies out of its shared receive buffer before
// queueing) and is the FULL received datagram. RFC 5340 §A.3.1 binds the checksum
// to the OSPF packet length, so the ospfv3-4 dispatcher MUST verify the checksum
// over Payload[:Header.Length], not the whole Payload (a datagram may carry
// trailing bytes such as an LLS block).
type RawPacket = wire.RawPacket

func packetTypeLabel(payload []byte) string {
	if len(payload) < 2 {
		return "short"
	}
	switch payload[1] {
	case 1:
		return "hello"
	case 2:
		return "dbdesc"
	case 3:
		return "lsreq"
	case 4:
		return "lsupdate"
	case 5:
		return "lsack"
	default:
		return "unknown"
	}
}

// InterfaceHandle is one open per-interface OSPFv3 raw IPv6 socket. Send takes the
// explicit source address (the interface link-local) so the on-wire source equals
// the address fed to the checksum pseudo-header (RFC 5340 §A.3.1); LinkLocalSource
// reports that address so the orchestrator can finalize the checksum.
//
// LinkLocalSource stays exported even though its only cross-package callers are test
// backends (internal/plugins/ospf/instance_v6_test.go fakeV6Handle): InterfaceHandle is
// the transport's pluggable-backend seam, so an interface method cannot be unexported
// without making the interface un-implementable from other packages. This is the spec's
// "keep + justify (public API used by tests)" outcome for ze-repository-check's heuristic.
type InterfaceHandle interface {
	IfIndex() int
	LinkLocalSource() netip.Addr
	Send(dst, src netip.Addr, payload []byte) error
	Recv() <-chan RawPacket
	JoinAllSPFRouters() error
	JoinAllDRouters() error
	LeaveAllDRouters() error
	Close() error
}

// DropRecorder reports a backend-level receive drop (e.g. a socket read error)
// through the transport-owned metrics; the orchestrator owns short/Instance-ID
// drop accounting. It is exported (like ospf/transport.DropRecorder) so an
// out-of-package fake Backend can satisfy the interface in tests.
type DropRecorder func(reason string)

// Backend opens per-interface raw sockets. Tests substitute a fake backend.
type Backend interface {
	OpenInterface(name string, recordDrop DropRecorder) (InterfaceHandle, error)
}

type ifaceState struct {
	name   string
	handle InterfaceHandle
	stop   chan struct{}
}

// Transport is the OSPFv3 raw IPv6 transport orchestrator.
type Transport struct {
	backend Backend

	mu         sync.Mutex
	enabled    map[string]types.InstanceID
	interfaces map[string]*ifaceState

	deliver chan RawPacket
	onDown  func(ifindex int, name string)
	onUp    func(ifindex int, name string)
	signer  func(name string, payload []byte) []byte

	subscribe   func(name string) (<-chan iface.LinkEvent, func())
	ifaceSubs   map[string]func()
	eventsWired bool
	teardown    []func()

	metrics *transportMetrics
	wg      sync.WaitGroup
}

// New constructs a Transport over backend. Production passes NewBackend(); tests
// pass a fake. Nil backend is allowed for tests that exercise pure helpers only.
func New(backend Backend) *Transport {
	return &Transport{
		backend:    backend,
		enabled:    make(map[string]types.InstanceID),
		interfaces: make(map[string]*ifaceState),
		ifaceSubs:  make(map[string]func()),
		subscribe:  iface.Subscribe,
		deliver:    make(chan RawPacket, 256),
		metrics:    nopTransportMetrics(),
	}
}

// SetMetrics installs the telemetry registry; the four transport series register
// against it.
func (t *Transport) SetMetrics(reg metrics.Registry) {
	if reg == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.metrics = newTransportMetrics(reg)
}

// EnableInterface enables name with the default Instance ID 0 (RFC 5340 §4.2.1). It
// satisfies the shared engine Transport interface; use enableInterfaceInstance to set a
// non-zero Instance ID from config.
func (t *Transport) EnableInterface(name string) { t.enableInterfaceInstance(name, 0) }

// enableInterfaceInstance marks an interface as OSPFv3-enabled with a specific Instance
// ID (RFC 5340 §4.2.1). It is opened only when a link-up event or rescan says the
// interface is up and a link-local source is available.
func (t *Transport) enableInterfaceInstance(name string, instanceID types.InstanceID) {
	t.mu.Lock()
	t.enabled[name] = instanceID
	wired := t.eventsWired
	t.mu.Unlock()
	if wired {
		t.subscribeIface(name)
	}
}

// DisableInterface unmarks an interface and closes it if open.
func (t *Transport) DisableInterface(name string) {
	t.mu.Lock()
	delete(t.enabled, name)
	cancel := t.ifaceSubs[name]
	delete(t.ifaceSubs, name)
	t.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if err := t.HandleLinkDown(name); err != nil {
		logger().Warn("ospfv3/transport: close on disable", "interface", name, "err", err)
	}
}

// SetSigner installs a hook that authenticates (signs) every outgoing OSPFv3
// packet just before it is sent. The hook returns the wire bytes to transmit. When
// a signer is set the transport does NOT finalize the packet checksum: RFC 7166
// §2.2 leaves the checksum zero and the Authentication Trailer covers integrity.
// ospfv3-12 owns the signer.
func (t *Transport) SetSigner(fn func(name string, payload []byte) []byte) {
	t.mu.Lock()
	t.signer = fn
	t.mu.Unlock()
}

// OnInterfaceDown registers the engine teardown callback for a closed interface.
func (t *Transport) OnInterfaceDown(fn func(ifindex int, name string)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onDown = fn
}

// OnInterfaceUp registers the engine callback for a newly-opened interface.
func (t *Transport) OnInterfaceUp(fn func(ifindex int, name string)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onUp = fn
}

// Receive returns the channel of received OSPFv3 datagrams.
func (t *Transport) Receive() <-chan RawPacket { return t.deliver }

// SubscribeIfaceEvents wires interface up/down (and addr-added, surfaced as a
// non-down event) to per-interface open/close, with a periodic rescan backstop.
func (t *Transport) SubscribeIfaceEvents(eb ze.EventBus) func() {
	return t.subscribeIfaceEventsWithRescan(eb, rescanInterval)
}

func (t *Transport) subscribeIfaceEventsWithRescan(eb ze.EventBus, interval time.Duration) func() {
	if eb == nil {
		return func() {}
	}
	t.mu.Lock()
	t.eventsWired = true
	names := make([]string, 0, len(t.enabled))
	for n := range t.enabled {
		names = append(names, n)
	}
	t.mu.Unlock()
	for _, n := range names {
		t.subscribeIface(n)
	}

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
			t.mu.Lock()
			t.eventsWired = false
			cancels := make([]func(), 0, len(t.ifaceSubs))
			for n, c := range t.ifaceSubs {
				cancels = append(cancels, c)
				delete(t.ifaceSubs, n)
			}
			t.mu.Unlock()
			for _, c := range cancels {
				c()
			}
			close(stopRescan)
		})
	}
	t.mu.Lock()
	t.teardown = append(t.teardown, cleanup)
	t.mu.Unlock()
	return cleanup
}

func (t *Transport) subscribeIface(name string) {
	ch, cancel := t.subscribe(name)
	t.mu.Lock()
	if _, dup := t.ifaceSubs[name]; dup || !t.eventsWired {
		t.mu.Unlock()
		cancel()
		return
	}
	t.ifaceSubs[name] = cancel
	t.mu.Unlock()

	t.wg.Go(func() {
		for ev := range ch {
			var err error
			if ev.Kind == iface.LinkDown {
				err = t.HandleLinkDown(ev.Name)
			} else {
				// Link-up and addr-added both retry the open; HandleLinkUp is
				// idempotent, so an addr-added after a pending open (IPv6 DAD) now
				// completes it.
				err = t.HandleLinkUp(ev.Name)
			}
			if err != nil {
				logger().Warn("ospfv3/transport: iface event handling", "interface", ev.Name, "kind", string(ev.Kind), "err", err)
			}
		}
	})
}

// RescanInterfaces re-attempts an open for every enabled-but-not-open interface.
// This backstops a pending open whose link-local source was not yet ready at
// link-up (IPv6 DAD).
func (t *Transport) RescanInterfaces() {
	t.mu.Lock()
	pending := make([]string, 0, len(t.enabled))
	for name := range t.enabled {
		if _, open := t.interfaces[name]; !open {
			pending = append(pending, name)
		}
	}
	t.mu.Unlock()
	for _, name := range pending {
		if err := t.HandleLinkUp(name); err != nil {
			logger().Warn("ospfv3/transport: rescan open interface", "interface", name, "err", err)
		}
	}
}

// HandleLinkUp opens the per-interface socket, joins ff02::5, and starts the RX
// loop. A backend error (including ErrNoLinkLocal while the link-local source is
// still tentative) leaves the interface enabled-but-not-open for the rescan to
// retry.
func (t *Transport) HandleLinkUp(name string) error {
	t.mu.Lock()
	_, enabled := t.enabled[name]
	_, open := t.interfaces[name]
	t.mu.Unlock()
	if !enabled || open {
		return nil
	}
	if t.backend == nil {
		return ErrNoBackend
	}
	handle, err := t.backend.OpenInterface(name, t.dropRecorder(name))
	if err != nil {
		return err
	}
	// NOTE (RFC 4552 residual startup gap, spec-ospf-ext-16 FIX-3): the socket is
	// opened and joins ff02::5 here, but the inbound IPsec require-policy is installed
	// later, in the onUp callback below (engine.onInterfaceUp -> ipsecInstaller). Until
	// that policy exists, the kernel has no reason to drop unprotected inbound OSPF, so
	// a neighbor's unprotected Hello arriving in this brief window could be delivered.
	// The outbound path is safe (onUp installs the SA/policy before the interface FSM
	// sends the first Hello). Closing the inbound window requires installing the inbound
	// policy before the group join, which means splitting the single onUp callback into
	// a pre-join "secure" hook and a post-open "start FSM" hook across the shared engine
	// Transport seam (the v4 engine has no IPsec) -- deferred as a larger refactor.
	if err := handle.JoinAllSPFRouters(); err != nil {
		if cerr := handle.Close(); cerr != nil {
			logger().Warn("ospfv3/transport: close after join failure", "interface", name, "err", cerr)
		}
		return err
	}

	st := &ifaceState{name: name, handle: handle, stop: make(chan struct{})}
	t.mu.Lock()
	if _, stillEnabled := t.enabled[name]; !stillEnabled {
		t.mu.Unlock()
		if cerr := handle.Close(); cerr != nil {
			logger().Warn("ospfv3/transport: close disabled interface", "interface", name, "err", cerr)
		}
		return nil
	}
	if _, dup := t.interfaces[name]; dup {
		t.mu.Unlock()
		if cerr := handle.Close(); cerr != nil {
			logger().Warn("ospfv3/transport: close duplicate interface", "interface", name, "err", cerr)
		}
		return nil
	}
	t.interfaces[name] = st
	t.metrics.socketsOpen.Set(float64(len(t.interfaces) * socketsPerInterface))
	onUp := t.onUp
	t.mu.Unlock()

	t.wg.Go(func() { t.rxLoop(st) })
	if onUp != nil {
		onUp(handle.IfIndex(), name)
	}
	return nil
}

// HandleLinkDown stops RX, leaves groups (via Close), closes the socket, and
// signals the engine to tear down adjacencies on the interface.
func (t *Transport) HandleLinkDown(name string) error {
	t.mu.Lock()
	st, open := t.interfaces[name]
	if !open {
		t.mu.Unlock()
		return nil
	}
	delete(t.interfaces, name)
	t.metrics.socketsOpen.Set(float64(len(t.interfaces) * socketsPerInterface))
	onDown := t.onDown
	t.mu.Unlock()

	close(st.stop)
	ifindex := st.handle.IfIndex()
	err := st.handle.Close()
	if onDown != nil {
		onDown(ifindex, name)
	}
	return err
}

func (t *Transport) dropRecorder(name string) DropRecorder {
	return func(reason string) {
		t.metrics.packetsDropped.With(name, reason).Inc()
	}
}

// RecordDrop increments the dropped-packets counter for an interface and reason.
func (t *Transport) RecordDrop(name, reason string) {
	t.metrics.packetsDropped.With(name, reason).Inc()
}

func (t *Transport) interfaceInstance(name string) (types.InstanceID, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	id, ok := t.enabled[name]
	return id, ok
}

func (t *Transport) rxLoop(st *ifaceState) {
	recv := st.handle.Recv()
	for {
		select {
		case <-st.stop:
			return
		case pkt, ok := <-recv:
			if !ok {
				return
			}
			// Drop a datagram too short to carry the 16-byte OSPFv3 common header
			// before any field is read (RFC 5340 §A.3.1).
			if len(pkt.Payload) < packet.CommonHeaderLen {
				t.metrics.packetsDropped.With(st.name, dropShort).Inc()
				continue
			}
			// RFC 5340 §4.2.1: discard a packet whose Instance ID does not match the
			// Instance ID configured for the receiving interface. This is the only
			// header field the transport reads.
			if want, known := t.interfaceInstance(st.name); known {
				if got, ok := packet.PeekInstanceID(pkt.Payload); !ok || got != want {
					t.metrics.packetsDropped.With(st.name, dropInstanceMismatch).Inc()
					continue
				}
			}
			t.metrics.packetsReceived.With(st.name, packetTypeLabel(pkt.Payload)).Inc()
			select {
			case t.deliver <- pkt:
			case <-st.stop:
				return
			}
		}
	}
}

// Transport error sentinels.
var (
	ErrNoBackend          = errors.New("ospfv3/transport: no backend")
	ErrInterfaceNotOpen   = errors.New("ospfv3/transport: interface not open")
	ErrInvalidDestination = errors.New("ospfv3/transport: invalid destination")
	// ErrNoLinkLocal is always wrapped with the interface name by its producer
	// (interfaceLinkLocal), so the sentinel itself carries only the condition.
	// Unwrapped it named no subject, and the ospfv3-vlink QEMU failure logged
	// "opening ipv6 interfaces: ... has no usable link-local source" with no way
	// to tell WHICH of the configured interfaces had none (ai/rules/cli.md).
	ErrNoLinkLocal = errors.New("no usable link-local source")
)

// SendPacket finalizes the IPv6 upper-layer checksum (unless a signer owns it) and
// sends final OSPFv3 bytes to dst on name. The egress link-local source is bound
// explicitly (Send's src argument / ControlMessage.Src) so the on-wire source
// equals the checksum pseudo-header source (RFC 5340 §A.3.1). Packet construction
// is not a transport concern; the transport changes only the checksum field.
func (t *Transport) SendPacket(name string, dst netip.Addr, payload []byte) error {
	if !dst.Is6() {
		return ErrInvalidDestination
	}
	t.mu.Lock()
	st, open := t.interfaces[name]
	signer := t.signer
	t.mu.Unlock()
	if !open {
		return ErrInterfaceNotOpen
	}
	src := st.handle.LinkLocalSource()
	if signer != nil {
		payload = signer(name, payload)
	} else {
		// RFC 5340 §A.3.1: finalize the IPv6 upper-layer checksum from the egress
		// source and destination before sending.
		packet.FinalizePacketChecksum(src, dst, payload)
	}
	if err := st.handle.Send(dst, src, payload); err != nil {
		t.metrics.packetsDropped.With(name, dropSendError).Inc()
		return err
	}
	t.metrics.packetsSent.With(name, packetTypeLabel(payload)).Inc()
	return nil
}

// routedHopLimit is the hop limit for a routed OSPFv3 virtual-link packet: unlike the
// hop-limit-1 link-local path, a virtual-link packet is routed across the transit area (RFC
// 5340 §2.9), so it needs a hop limit large enough to traverse it.
const routedHopLimit = 64

// routedSenderV6 is the optional capability of an InterfaceHandle to send a routed unicast
// packet from an explicit GLOBAL source with a hop limit > 1.
type routedSenderV6 interface {
	SendRouted(dst, src netip.Addr, payload []byte, hopLimit int) error
}

// SendPacketRouted sends an OSPFv3 virtual-link packet ROUTED across a transit area (RFC
// 5340 §2.9): unicast to the neighbor's GLOBAL address dst from the local GLOBAL source src
// with a hop limit > 1 (not the link-local source / hop-limit-1 path). The IPv6 upper-layer
// checksum pseudo-header is finalized against the GLOBAL src (RFC 5340 §A.3.1), so it must
// match the on-wire source. name is a real transit egress interface (the packet is routed,
// not bound to the synthetic virtual interface).
func (t *Transport) SendPacketRouted(name string, dst, src netip.Addr, payload []byte) error {
	if !dst.Is6() || !src.Is6() {
		return ErrInvalidDestination
	}
	t.mu.Lock()
	st, open := t.interfaces[name]
	signer := t.signer
	t.mu.Unlock()
	if !open {
		return ErrInterfaceNotOpen
	}
	if signer != nil {
		payload = signer(name, payload)
	} else {
		packet.FinalizePacketChecksum(src, dst, payload)
	}
	var err error
	if rs, ok := st.handle.(routedSenderV6); ok {
		err = rs.SendRouted(dst, src, payload, routedHopLimit)
	} else {
		err = st.handle.Send(dst, src, payload)
	}
	if err != nil {
		t.metrics.packetsDropped.With(name, dropSendError).Inc()
		return err
	}
	t.metrics.packetsSent.With(name, packetTypeLabel(payload)).Inc()
	return nil
}

// JoinAllDRouters joins ff02::6 on the interface (RFC 5340 §2.9: only the DR/BDR).
func (t *Transport) JoinAllDRouters(name string) error {
	t.mu.Lock()
	st, open := t.interfaces[name]
	t.mu.Unlock()
	if !open {
		return ErrInterfaceNotOpen
	}
	return st.handle.JoinAllDRouters()
}

// LeaveAllDRouters leaves ff02::6 on the interface (on losing the DR/BDR role).
func (t *Transport) LeaveAllDRouters(name string) error {
	t.mu.Lock()
	st, open := t.interfaces[name]
	t.mu.Unlock()
	if !open {
		return ErrInterfaceNotOpen
	}
	return st.handle.LeaveAllDRouters()
}

// InterfaceSource returns the bound link-local source and kernel ifindex of an open
// interface, or ok=false when the interface is not open. The RFC 4552 IPsec installer
// uses it to scope the transport-mode SA/policy selectors to the exact on-wire source
// (RFC 5340 §A.3.1) and to derive a per-interface reqid.
func (t *Transport) InterfaceSource(name string) (netip.Addr, int, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	st, ok := t.interfaces[name]
	if !ok {
		return netip.Addr{}, 0, false
	}
	return st.handle.LinkLocalSource(), st.handle.IfIndex(), true
}

// InterfaceOpen reports whether the interface socket is currently open.
func (t *Transport) InterfaceOpen(name string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	_, ok := t.interfaces[name]
	return ok
}

// OpenInterfaceCount returns the number of currently-open interfaces.
func (t *Transport) OpenInterfaceCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.interfaces)
}

// InterfaceNameByIfIndex reverse-maps a receiving ifindex to its interface name.
func (t *Transport) InterfaceNameByIfIndex(ifindex int) (string, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for name, st := range t.interfaces {
		if st.handle.IfIndex() == ifindex {
			return name, true
		}
	}
	return "", false
}

// Close tears down all subscriptions and interfaces and waits for goroutines.
func (t *Transport) Close() {
	t.mu.Lock()
	teardown := t.teardown
	t.teardown = nil
	names := make([]string, 0, len(t.interfaces))
	for name := range t.interfaces {
		names = append(names, name)
	}
	t.mu.Unlock()
	for _, fn := range teardown {
		fn()
	}
	for _, name := range names {
		if err := t.HandleLinkDown(name); err != nil {
			logger().Warn("ospfv3/transport: close interface", "interface", name, "err", err)
		}
	}
	t.wg.Wait()
}
